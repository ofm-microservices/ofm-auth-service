package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth-service/config"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestBootstrap(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Bootstrap Suite")
}

var (
	bootstrapJSContainer   testcontainers.Container
	bootstrapNoJSContainer testcontainers.Container
	bootstrapJSCfg         config.NATSConfig
	bootstrapNoJSCfg       config.NATSConfig
	bootstrapLogger        logging.Logger
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var err error
	bootstrapLogger, err = logging.New("auth-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())

	bootstrapJSContainer, bootstrapJSCfg = startBootstrapNATSContainer(ctx, true)
	bootstrapNoJSContainer, bootstrapNoJSCfg = startBootstrapNATSContainer(ctx, false)
})

var _ = AfterSuite(func() {
	if bootstrapJSContainer != nil {
		Expect(bootstrapJSContainer.Terminate(context.Background())).To(Succeed())
	}
	if bootstrapNoJSContainer != nil {
		Expect(bootstrapNoJSContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("bootstrap integration", func() {
	It("connects to a real nats server", func() {
		nc, err := Connect(bootstrapJSCfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(nc.IsConnected()).To(BeTrue())
		nc.Close()
	})

	It("creates and then updates the required streams", func() {
		Expect(EnsureStream(bootstrapJSCfg, bootstrapLogger)).To(Succeed())
		Expect(EnsureStream(bootstrapJSCfg, bootstrapLogger)).To(Succeed())

		nc, err := nats.Connect(bootstrapJSCfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		js, err := nc.JetStream()
		Expect(err).NotTo(HaveOccurred())

		authEventsInfo, err := js.StreamInfo(bootstrapJSCfg.AuthEventsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(authEventsInfo.Config.Subjects).To(ContainElements(
			bootstrapJSCfg.AuthCreatedSubject,
			bootstrapJSCfg.SagaCreateAuthResultSubject,
			bootstrapJSCfg.SagaDeleteAuthResultSubject,
		))

		sagaCommandsInfo, err := js.StreamInfo(bootstrapJSCfg.SagaCommandsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(sagaCommandsInfo.Config.Subjects).To(ContainElements(
			bootstrapJSCfg.SagaCreateAuthSubject,
			bootstrapJSCfg.SagaDeleteAuthSubject,
		))

		mailCommandsInfo, err := js.StreamInfo(bootstrapJSCfg.MailCommandsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(mailCommandsInfo.Config.Subjects).To(ContainElement(bootstrapJSCfg.MailSendSubject))
	})

	It("wraps jetstream initialization failures when the server has no jetstream", func() {
		err := EnsureStream(bootstrapNoJSCfg, bootstrapLogger)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ensure stream"))
	})
})

var _ = Describe("error wrappers", func() {
	It("preserves wrapped causes", func() {
		cause := errors.New("boom")

		Expect(WrapInitJetStreamContextError(cause)).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureStreamError("STREAM", cause, cause)).To(MatchError(ContainSubstring(`ensure stream "STREAM"`)))
		Expect(WrapConnectToNATSError(cause)).To(MatchError(ContainSubstring("connect to nats")))
	})
})

func startBootstrapNATSContainer(ctx context.Context, jetstream bool) (testcontainers.Container, config.NATSConfig) {
	cmd := []string{}
	if jetstream {
		cmd = []string{"-js"}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.12.4-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          cmd,
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
	}
}
