package nats

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"auth-service/config"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	brokerSuiteContainer testcontainers.Container
	brokerSuiteBaseCfg   config.NATSConfig
	brokerSuiteLogger    logging.Logger
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var err error
	brokerSuiteLogger, err = logging.New("auth-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())

	brokerSuiteContainer, brokerSuiteBaseCfg = startBrokerNATSContainer(ctx)
})

var _ = AfterSuite(func() {
	if brokerSuiteContainer != nil {
		Expect(brokerSuiteContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("natsBroker integration", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
		cfg    config.NATSConfig
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 45*time.Second)
		cfg = uniqueBrokerConfig(brokerSuiteBaseCfg)

		nc, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		js, err := nc.JetStream()
		Expect(err).NotTo(HaveOccurred())

		_, err = js.AddStream(&nats.StreamConfig{
			Name:      cfg.SagaCommandsStream,
			Subjects:  []string{cfg.SagaCreateAuthSubject, cfg.SagaDeleteAuthSubject},
			Storage:   nats.FileStorage,
			Retention: nats.LimitsPolicy,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cancel()
	})

	It("publishes messages to core nats subjects", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		sub, err := rawConn.SubscribeSync(cfg.AuthCreatedSubject)
		Expect(err).NotTo(HaveOccurred())
		Expect(rawConn.Flush()).To(Succeed())

		brokerAny, err := NewBroker(cfg, brokerSuiteLogger)
		Expect(err).NotTo(HaveOccurred())
		defer brokerAny.Close()

		Expect(brokerAny.Publish(ctx, cfg.AuthCreatedSubject, []byte("payload"))).To(Succeed())

		msg, err := sub.NextMsg(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(msg.Data)).To(Equal("payload"))
	})

	It("flushes with a synthetic timeout when the caller provides no deadline", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		Expect(Flush(context.Background(), rawConn)).To(Succeed())
	})

	It("subscribes and dispatches core nats messages", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		brokerAny, err := NewBroker(cfg, brokerSuiteLogger)
		Expect(err).NotTo(HaveOccurred())
		defer brokerAny.Close()

		received := make(chan []byte, 1)
		Expect(brokerAny.Subscribe(ctx, cfg.AuthCreatedSubject, func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal(cfg.AuthCreatedSubject))
			received <- payload
			return nil
		})).To(Succeed())

		Expect(rawConn.Publish(cfg.AuthCreatedSubject, []byte("payload"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		Eventually(received).Should(Receive(Equal([]byte("payload"))))
	})

	It("runs a pull consumer against jetstream", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		brokerAny, err := NewBroker(cfg, brokerSuiteLogger)
		Expect(err).NotTo(HaveOccurred())
		concrete := brokerAny.(*natsBroker)
		defer concrete.Close()

		runCtx, runCancel := context.WithCancel(ctx)
		defer runCancel()

		var handled atomic.Int32
		Expect(concrete.RunPullConsumer(runCtx, config.PullConsumerConfig{
			Stream:     cfg.SagaCommandsStream,
			Subject:    cfg.SagaCreateAuthSubject,
			Durable:    cfg.SagaCreateAuthDurable,
			BatchSize:  2,
			MaxWait:    50 * time.Millisecond,
			Workers:    1,
			QueueSize:  4,
			AckWait:    2 * time.Second,
			MaxDeliver: 3,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				CheckInterval:   time.Millisecond,
				MediumPending:   1,
				HighPending:     2,
				LowBatchSize:    1,
				LowMaxWait:      20 * time.Millisecond,
				MediumBatchSize: 2,
				MediumMaxWait:   10 * time.Millisecond,
				HighBatchSize:   3,
				HighMaxWait:     5 * time.Millisecond,
			},
		}, func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal(cfg.SagaCreateAuthSubject))
			Expect(payload).NotTo(BeEmpty())
			handled.Add(1)
			return nil
		})).To(Succeed())

		Expect(rawConn.Publish(cfg.SagaCreateAuthSubject, []byte("one"))).To(Succeed())
		Expect(rawConn.Publish(cfg.SagaCreateAuthSubject, []byte("two"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		Eventually(func() int32 { return handled.Load() }).Should(Equal(int32(2)))
		runCancel()
	})

	It("wraps publish and subscribe failures on a closed connection", func() {
		brokerAny, err := NewBroker(cfg, brokerSuiteLogger)
		Expect(err).NotTo(HaveOccurred())
		concrete := brokerAny.(*natsBroker)
		concrete.Close()

		err = concrete.Publish(ctx, cfg.AuthCreatedSubject, []byte("payload"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("publish to nats"))

		err = concrete.Subscribe(ctx, cfg.AuthCreatedSubject, func(context.Context, string, []byte) error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("subscribe to nats"))
	})

	It("resolves adaptive pull tiers", func() {
		tier, batch, waitFor := ResolvePullPlan(config.PullConsumerConfig{
			BatchSize: 1,
			MaxWait:   50 * time.Millisecond,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				MediumPending:   10,
				HighPending:     20,
				LowBatchSize:    2,
				LowMaxWait:      40 * time.Millisecond,
				MediumBatchSize: 4,
				MediumMaxWait:   20 * time.Millisecond,
				HighBatchSize:   8,
				HighMaxWait:     10 * time.Millisecond,
			},
		}, 25)
		Expect(tier).To(Equal("high"))
		Expect(batch).To(Equal(8))
		Expect(waitFor).To(Equal(10 * time.Millisecond))
	})
})

var _ = Describe("broker wrappers", func() {
	It("preserves wrapped causes", func() {
		cause := errors.New("boom")

		Expect(WrapConnectToNATSError(cause)).To(MatchError(ContainSubstring("connect to nats")))
		Expect(WrapPublishToNATSError("subject", cause)).To(MatchError(ContainSubstring("publish to nats (subject)")))
		Expect(WrapSubscribeToNATSError("subject", cause)).To(MatchError(ContainSubstring("subscribe to nats (subject)")))
		Expect(WrapFlushNATSPublisherError(cause)).To(MatchError(ContainSubstring("flush nats publisher")))
		Expect(WrapInitJetStreamContextError(cause)).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureConsumerError("STREAM", "durable", cause, cause)).To(MatchError(ContainSubstring(`ensure consumer "durable" in stream "STREAM"`)))
		Expect(WrapCreatePullSubscriberError("subject", "durable", cause)).To(MatchError(ContainSubstring(`create pull subscriber subject="subject" durable="durable"`)))
		Expect(WrapUnmarshalCreateAuthCommandError(cause)).To(MatchError(ContainSubstring("unmarshal create auth command")))
		Expect(WrapUnmarshalDeleteAuthCommandError(cause)).To(MatchError(ContainSubstring("unmarshal delete auth command")))
		Expect(WrapMarshalCreateAuthResultError(cause)).To(MatchError(ContainSubstring("marshal create auth result")))
		Expect(WrapMarshalDeleteAuthResultError(cause)).To(MatchError(ContainSubstring("marshal delete auth result")))
		Expect(WrapMarshalMailSendCommandError(cause)).To(MatchError(ContainSubstring("marshal mail send command")))
	})
})

func startBrokerNATSContainer(ctx context.Context) (testcontainers.Container, config.NATSConfig) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.12.4-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := container.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := container.MappedPort(ctx, "4222/tcp")
	Expect(err).NotTo(HaveOccurred())

	return container, config.NATSConfig{
		URL:                         "nats://" + host + ":" + port.Port(),
		AuthEventsStream:            "AUTH_EVENTS",
		MailCommandsStream:          "MAIL_COMMANDS",
		SagaCommandsStream:          "SAGA_AUTH_COMMANDS",
		AuthCreatedSubject:          "auth.created",
		SagaCreateAuthSubject:       "saga.auth.create_pending_registration",
		SagaDeleteAuthSubject:       "saga.auth.delete",
		SagaCreateAuthResultSubject: "saga.auth.create_pending_registration.result",
		SagaDeleteAuthResultSubject: "saga.auth.delete.result",
		MailSendSubject:             "mail.send",
		SagaCreateAuthDurable:       "auth_service_saga_create_pending_registration",
		SagaDeleteAuthDurable:       "auth_service_saga_delete",
	}
}

func uniqueBrokerConfig(base config.NATSConfig) config.NATSConfig {
	suffix := time.Now().UTC().Format("150405000000000")
	base.AuthEventsStream += "_" + suffix
	base.MailCommandsStream += "_" + suffix
	base.SagaCommandsStream += "_" + suffix
	base.AuthCreatedSubject += "." + suffix
	base.SagaCreateAuthSubject += "." + suffix
	base.SagaDeleteAuthSubject += "." + suffix
	base.SagaCreateAuthResultSubject += "." + suffix
	base.SagaDeleteAuthResultSubject += "." + suffix
	base.MailSendSubject += "." + suffix
	base.SagaCreateAuthDurable += "_" + suffix
	base.SagaDeleteAuthDurable += "_" + suffix
	return base
}
