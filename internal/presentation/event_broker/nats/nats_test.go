package nats

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"auth-service/config"
	auth "auth-service/internal/domain"
	eventbroker "auth-service/internal/presentation/event_broker"
	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestNATS(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Suite")
}

type fakeMapper struct {
	createFailure []byte
	createSuccess []byte
	deleteFailure []byte
	deleteSuccess []byte
	mailPayload   []byte
	err           error
}

func (m *fakeMapper) ToCreateFailureResultPayload(createAuthCommand, string) ([]byte, error) {
	return m.createFailure, m.err
}
func (m *fakeMapper) ToCreateSuccessResultPayload(createAuthCommand, string, string, string) ([]byte, error) {
	return m.createSuccess, m.err
}
func (m *fakeMapper) ToDeleteFailureResultPayload(deleteAuthCommand, string) ([]byte, error) {
	return m.deleteFailure, m.err
}
func (m *fakeMapper) ToDeleteSuccessResultPayload(deleteAuthCommand) ([]byte, error) {
	return m.deleteSuccess, m.err
}
func (m *fakeMapper) ToMailSendCommandPayload(createAuthCommand, string, string, string) ([]byte, error) {
	return m.mailPayload, m.err
}

type stubRuntimeFactory struct {
	runtime PullConsumerRuntime
	err     error
}

func (f *stubRuntimeFactory) Create(*nats.Conn, logging.Logger, config.PullConsumerConfig, eventbroker.MessageHandler) (PullConsumerRuntime, error) {
	return f.runtime, f.err
}

type stubRuntime struct {
	started bool
}

func (r *stubRuntime) Start(context.Context) {
	r.started = true
}

var _ = Describe("registration saga subscriber", func() {
	var (
		ctrl   *gomock.Controller
		broker *MockEventBroker
		svc    *MockAuthService
		logger logging.Logger
		cfg    config.NATSConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		broker = NewMockEventBroker(ctrl)
		svc = NewMockAuthService(ctrl)

		var err error
		logger, err = logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		cfg = config.NATSConfig{
			SagaCommandsStream:          "SAGA_AUTH_COMMANDS",
			SagaCreateAuthSubject:       "saga.auth.create_pending_registration",
			SagaDeleteAuthSubject:       "saga.auth.delete",
			SagaCreateAuthResultSubject: "saga.auth.create_pending_registration.result",
			SagaDeleteAuthResultSubject: "saga.auth.delete.result",
			MailSendSubject:             "mail.send",
			SagaCreateAuthDurable:       "auth_service_saga_create_pending_registration",
			SagaDeleteAuthDurable:       "auth_service_saga_delete",
			SagaBatchSize:               32,
			SagaMaxWait:                 10 * time.Millisecond,
			SagaWorkers:                 2,
			SagaQueueSize:               10,
			SagaAckWait:                 5 * time.Second,
			SagaMaxDeliver:              5,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("validates broker constructor dependencies", func() {
		brokerAny, err := NewBroker(config.NATSConfig{}, logger)
		Expect(brokerAny).To(BeNil())
		Expect(err).To(MatchError(ErrEmptyNATSURL))

		brokerAny, err = NewBroker(config.NATSConfig{URL: "nats://127.0.0.1:4222"}, nil)
		Expect(brokerAny).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("wraps broker connection failures", func() {
		brokerAny, err := NewBroker(config.NATSConfig{URL: "nats://127.0.0.1:1"}, logger)
		Expect(brokerAny).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})

	It("validates pull-consumer configuration before touching jetstream", func() {
		brokerAny := &natsBroker{log: logger}

		err := brokerAny.RunPullConsumer(context.Background(), config.PullConsumerConfig{}, func(context.Context, string, []byte) error {
			return nil
		})
		Expect(err).To(MatchError(ErrEmptyStreamName))
	})

	It("starts a created pull-consumer runtime", func() {
		runtime := &stubRuntime{}
		brokerAny := &natsBroker{
			log:            logger,
			validator:      newPullConsumerConfigValidator(),
			runtimeFactory: &stubRuntimeFactory{runtime: runtime},
		}

		err := brokerAny.RunPullConsumer(context.Background(), config.PullConsumerConfig{
			Stream:     "stream",
			Subject:    "subject",
			Durable:    "durable",
			BatchSize:  1,
			MaxWait:    time.Millisecond,
			Workers:    1,
			QueueSize:  1,
			AckWait:    time.Second,
			MaxDeliver: 1,
		}, func(context.Context, string, []byte) error {
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(runtime.started).To(BeTrue())
	})

	It("returns runtime factory failures", func() {
		brokerAny := &natsBroker{
			log:            logger,
			validator:      newPullConsumerConfigValidator(),
			runtimeFactory: &stubRuntimeFactory{err: errors.New("boom")},
		}

		err := brokerAny.RunPullConsumer(context.Background(), config.PullConsumerConfig{
			Stream:     "stream",
			Subject:    "subject",
			Durable:    "durable",
			BatchSize:  1,
			MaxWait:    time.Millisecond,
			Workers:    1,
			QueueSize:  1,
			AckWait:    time.Second,
			MaxDeliver: 1,
		}, func(context.Context, string, []byte) error {
			return nil
		})

		Expect(err).To(MatchError("boom"))
	})

	It("resolves medium and low adaptive pull tiers", func() {
		cfg := config.PullConsumerConfig{
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
		}

		tier, batchSize, maxWait := ResolvePullPlan(cfg, 12)
		Expect(tier).To(Equal("medium"))
		Expect(batchSize).To(Equal(4))
		Expect(maxWait).To(Equal(20 * time.Millisecond))

		tier, batchSize, maxWait = ResolvePullPlan(cfg, 1)
		Expect(tier).To(Equal("low"))
		Expect(batchSize).To(Equal(2))
		Expect(maxWait).To(Equal(40 * time.Millisecond))
	})

	It("validates constructor dependencies", func() {
		sub, err := NewRegistrationSagaSubscriber(nil, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(sub).To(BeNil())
		Expect(err).To(MatchError(ErrNilBroker))

		sub, err = NewRegistrationSagaSubscriber(broker, nil, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(sub).To(BeNil())
		Expect(err).To(MatchError(ErrNilAuthService))

		sub, err = NewRegistrationSagaSubscriber(broker, svc, cfg, nil, logger)
		Expect(sub).To(BeNil())
		Expect(err).To(MatchError(ErrNilFailureReasonResolver))

		sub, err = NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), nil)
		Expect(sub).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("registers both pull consumers", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())

		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

		Expect(subAny.Subscribe(context.Background())).To(Succeed())
	})

	It("returns pull-consumer registration failures", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())

		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))

		Expect(subAny.Subscribe(context.Background())).To(MatchError("boom"))
	})

	It("handles create auth success", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)
		sub.mapr = &fakeMapper{
			createSuccess: []byte(`{"status":"success"}`),
			mailPayload:   []byte(`{"message_type":"email_code"}`),
		}

		cmd := createAuthCommand{
			SessionID:    "session-1",
			ClientID:     "client-1",
			UserID:       "user-1",
			Email:        "user@example.com",
			Username:     "alex",
			PasswordHash: "hash",
		}
		payload, err := json.Marshal(cmd)
		Expect(err).NotTo(HaveOccurred())

		var createHandler func(context.Context, string, []byte) error
		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, pullCfg config.PullConsumerConfig, handler func(context.Context, string, []byte) error) error {
				if pullCfg.Subject == cfg.SagaCreateAuthSubject {
					createHandler = handler
				}
				return nil
			}).Times(2)

		Expect(sub.Subscribe(context.Background())).To(Succeed())
		Expect(createHandler).NotTo(BeNil())

		svc.EXPECT().
			CreatePendingRegistration(gomock.Any(), "user-1", "user@example.com", "alex", "hash").
			Return(&auth.PendingRegistrationResult{
				UserID:           "user-1",
				Email:            "user@example.com",
				VerificationCode: "123456",
				ExpiresIn:        "10 minutes",
			}, nil)

		broker.EXPECT().Publish(gomock.Any(), cfg.SagaCreateAuthResultSubject, []byte(`{"status":"success"}`)).Return(nil)
		broker.EXPECT().Publish(gomock.Any(), cfg.MailSendSubject, []byte(`{"message_type":"email_code"}`)).Return(nil)

		Expect(createHandler(context.Background(), cfg.SagaCreateAuthSubject, payload)).To(Succeed())
	})

	It("handles create auth failures by publishing a failure result", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)
		sub.mapr = &fakeMapper{createFailure: []byte(`{"status":"failed"}`)}

		cmd := createAuthCommand{UserID: "user-1", Email: "user@example.com", Username: "alex", PasswordHash: "hash"}
		payload, err := json.Marshal(cmd)
		Expect(err).NotTo(HaveOccurred())

		var createHandler func(context.Context, string, []byte) error
		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, pullCfg config.PullConsumerConfig, handler func(context.Context, string, []byte) error) error {
				if pullCfg.Subject == cfg.SagaCreateAuthSubject {
					createHandler = handler
				}
				return nil
			}).Times(2)

		Expect(sub.Subscribe(context.Background())).To(Succeed())
		Expect(createHandler).NotTo(BeNil())

		svc.EXPECT().
			CreatePendingRegistration(gomock.Any(), "user-1", "user@example.com", "alex", "hash").
			Return(nil, auth.ErrEmailAlreadyTaken)
		broker.EXPECT().Publish(gomock.Any(), cfg.SagaCreateAuthResultSubject, []byte(`{"status":"failed"}`)).Return(nil)

		Expect(createHandler(context.Background(), cfg.SagaCreateAuthSubject, payload)).To(Succeed())
	})

	It("returns create-command decode failures", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)

		err = sub.handleCreateAuthCommand(context.Background(), cfg.SagaCreateAuthSubject, []byte("{"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unmarshal create auth command"))
	})

	It("returns mapper and publish failures from create helper paths", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)
		sub.mapr = &fakeMapper{err: errors.New("map failed")}

		err = sub.publishCreateFailureResult(context.Background(), createAuthCommand{}, auth.ErrInvalidEmail)
		Expect(err).To(MatchError("map failed"))

		sub.mapr = &fakeMapper{createSuccess: []byte("ok"), mailPayload: []byte("mail")}
		broker.EXPECT().Publish(gomock.Any(), cfg.SagaCreateAuthResultSubject, []byte("ok")).Return(errors.New("publish failed"))
		err = sub.publishCreateSuccessResult(context.Background(), createAuthCommand{}, "user@example.com", "123456", "10 minutes")
		Expect(err).To(MatchError("publish failed"))

		broker.EXPECT().Publish(gomock.Any(), cfg.SagaCreateAuthResultSubject, []byte("ok")).Return(nil)
		broker.EXPECT().Publish(gomock.Any(), cfg.MailSendSubject, []byte("mail")).Return(errors.New("mail publish failed"))
		err = sub.publishCreateSuccessResult(context.Background(), createAuthCommand{}, "user@example.com", "123456", "10 minutes")
		Expect(err).To(MatchError("mail publish failed"))
	})

	It("handles delete auth success", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)
		sub.mapr = &fakeMapper{deleteSuccess: []byte(`{"status":"success"}`)}

		cmd := deleteAuthCommand{UserID: "user-1"}
		payload, err := json.Marshal(cmd)
		Expect(err).NotTo(HaveOccurred())

		var deleteHandler func(context.Context, string, []byte) error
		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, pullCfg config.PullConsumerConfig, handler func(context.Context, string, []byte) error) error {
				if pullCfg.Subject == cfg.SagaDeleteAuthSubject {
					deleteHandler = handler
				}
				return nil
			}).Times(2)

		Expect(sub.Subscribe(context.Background())).To(Succeed())
		Expect(deleteHandler).NotTo(BeNil())

		svc.EXPECT().DeleteCredential(gomock.Any(), "user-1").Return(nil)
		broker.EXPECT().Publish(gomock.Any(), cfg.SagaDeleteAuthResultSubject, []byte(`{"status":"success"}`)).Return(nil)

		Expect(deleteHandler(context.Background(), cfg.SagaDeleteAuthSubject, payload)).To(Succeed())
	})

	It("handles delete auth failures by publishing a failure result", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)
		sub.mapr = &fakeMapper{deleteFailure: []byte(`{"status":"failed"}`)}

		cmd := deleteAuthCommand{UserID: "user-1"}
		payload, err := json.Marshal(cmd)
		Expect(err).NotTo(HaveOccurred())

		var deleteHandler func(context.Context, string, []byte) error
		broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, pullCfg config.PullConsumerConfig, handler func(context.Context, string, []byte) error) error {
				if pullCfg.Subject == cfg.SagaDeleteAuthSubject {
					deleteHandler = handler
				}
				return nil
			}).Times(2)

		Expect(sub.Subscribe(context.Background())).To(Succeed())
		Expect(deleteHandler).NotTo(BeNil())

		svc.EXPECT().DeleteCredential(gomock.Any(), "user-1").Return(auth.ErrAuthNotFound)
		broker.EXPECT().Publish(gomock.Any(), cfg.SagaDeleteAuthResultSubject, []byte(`{"status":"failed"}`)).Return(nil)

		Expect(deleteHandler(context.Background(), cfg.SagaDeleteAuthSubject, payload)).To(Succeed())
	})

	It("returns delete-command decode failures and helper errors", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)

		err = sub.handleDeleteAuthCommand(context.Background(), cfg.SagaDeleteAuthSubject, []byte("{"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unmarshal delete auth command"))

		sub.mapr = &fakeMapper{err: errors.New("map failed")}
		err = sub.publishDeleteFailureResult(context.Background(), deleteAuthCommand{}, auth.ErrAuthNotFound)
		Expect(err).To(MatchError("map failed"))

		err = sub.publishDeleteSuccessResult(context.Background(), deleteAuthCommand{})
		Expect(err).To(MatchError("map failed"))
	})

	It("builds adaptive pull config", func() {
		cfg.SagaAdaptiveEnabled = true
		cfg.SagaAdaptiveCheckInterval = time.Second
		cfg.SagaAdaptiveMediumPending = 100
		cfg.SagaAdaptiveHighPending = 1000
		cfg.SagaAdaptiveLowBatchSize = 4
		cfg.SagaAdaptiveLowMaxWait = 50 * time.Millisecond
		cfg.SagaAdaptiveMediumBatchSize = 16
		cfg.SagaAdaptiveMediumMaxWait = 20 * time.Millisecond
		cfg.SagaAdaptiveHighBatchSize = 64
		cfg.SagaAdaptiveHighMaxWait = 5 * time.Millisecond

		result := BuildAdaptiveConfig(cfg)

		Expect(result.Enabled).To(BeTrue())
		Expect(result.HighPending).To(Equal(1000))
		Expect(result.HighBatchSize).To(Equal(64))
	})

	It("builds the pull-consumer config for each subscriber subject", func() {
		subAny, err := NewRegistrationSagaSubscriber(broker, svc, cfg, NewDomainFailureReasonResolver(), logger)
		Expect(err).NotTo(HaveOccurred())
		sub := subAny.(*registrationSagaSubscriber)

		pullCfg := sub.buildPullConsumerConfig(cfg.SagaCreateAuthSubject, cfg.SagaCreateAuthDurable)
		Expect(pullCfg.Stream).To(Equal(cfg.SagaCommandsStream))
		Expect(pullCfg.Subject).To(Equal(cfg.SagaCreateAuthSubject))
		Expect(pullCfg.Durable).To(Equal(cfg.SagaCreateAuthDurable))
	})

	It("resolves the base pull plan when adaptive mode is disabled", func() {
		tier, batchSize, maxWait := ResolvePullPlan(config.PullConsumerConfig{
			BatchSize: 8,
			MaxWait:   25 * time.Millisecond,
		}, 999)

		Expect(tier).To(Equal("base"))
		Expect(batchSize).To(Equal(8))
		Expect(maxWait).To(Equal(25 * time.Millisecond))
	})

	It("validates pull-consumer configs across all error branches", func() {
		validator := newPullConsumerConfigValidator()

		Expect(validator.Validate(config.PullConsumerConfig{})).To(MatchError(ErrEmptyStreamName))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream"})).To(MatchError(ErrEmptySubject))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject"})).To(MatchError(ErrEmptyDurableName))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable"})).To(MatchError(ErrInvalidBatchSize))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1})).To(MatchError(ErrInvalidMaxWait))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond})).To(MatchError(ErrInvalidWorkerCount))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1})).To(MatchError(ErrInvalidQueueSize))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1})).To(MatchError(ErrInvalidAckWait))
		Expect(validator.Validate(config.PullConsumerConfig{Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Second})).To(MatchError(ErrInvalidMaxDeliver))
		Expect(validator.Validate(config.PullConsumerConfig{
			Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Second, MaxDeliver: 1,
			Adaptive: config.PullAdaptiveConfig{Enabled: true},
		})).To(MatchError(ErrInvalidAdaptiveCheckInterval))
		Expect(validator.Validate(config.PullConsumerConfig{
			Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Second, MaxDeliver: 1,
			Adaptive: config.PullAdaptiveConfig{Enabled: true, CheckInterval: time.Second, MediumPending: 1, HighPending: 1},
		})).To(MatchError(ErrInvalidAdaptiveThresholds))
		Expect(validator.Validate(config.PullConsumerConfig{
			Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Second, MaxDeliver: 1,
			Adaptive: config.PullAdaptiveConfig{Enabled: true, CheckInterval: time.Second, MediumPending: 1, HighPending: 2},
		})).To(MatchError(ErrInvalidAdaptivePlan))
		Expect(validator.Validate(config.PullConsumerConfig{
			Stream: "stream", Subject: "subject", Durable: "durable", BatchSize: 1, MaxWait: time.Millisecond, Workers: 1, QueueSize: 1, AckWait: time.Second, MaxDeliver: 1,
			Adaptive: config.PullAdaptiveConfig{Enabled: true, CheckInterval: time.Second, MediumPending: 1, HighPending: 2, LowBatchSize: 1, LowMaxWait: time.Millisecond, MediumBatchSize: 1, MediumMaxWait: time.Millisecond, HighBatchSize: 1, HighMaxWait: time.Millisecond},
		})).To(Succeed())
	})

	It("covers runtime helper branches without JetStream", func() {
		runtime := &pullConsumerRuntime{
			log:     logger,
			cfg:     config.PullConsumerConfig{Subject: "subject", Durable: "durable", Adaptive: config.PullAdaptiveConfig{}},
			handler: func(context.Context, string, []byte) error { return nil },
			jobs:    make(chan *nats.Msg),
		}

		Expect(newPullConsumerRuntimeFactory()).NotTo(BeNil())
		Expect(runtime.handleFetchError(nil)).To(BeFalse())
		Expect(runtime.handleFetchError(nats.ErrTimeout)).To(BeTrue())
		Expect(runtime.handleFetchError(errors.New("boom"))).To(BeTrue())

		doneCtx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(runtime.shouldStop(doneCtx)).To(BeTrue())
		Expect(runtime.dispatchFetchedMessages(doneCtx, []*nats.Msg{{}})).To(BeTrue())

		state := pullConsumerFetchState{batch: 1, wait: time.Millisecond, tier: "base", lastAdaptiveCheck: time.Now()}
		runtime.maybeUpdateAdaptivePlan(&state)
		Expect(state.tier).To(Equal("base"))
	})
})

var _ = Describe("nats mapper and failure reasons", func() {
	It("maps create and delete results", func() {
		mapr := newRegistrationSagaMessageMapper()

		createFailurePayload, err := mapr.ToCreateFailureResultPayload(createAuthCommand{
			SessionID: "session-1",
			ClientID:  "client-1",
			UserID:    "user-1",
			SagaID:    "saga-1",
		}, "failed")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(createFailurePayload)).To(ContainSubstring(`"status":"failed"`))
		Expect(string(createFailurePayload)).To(ContainSubstring(`"error":"failed"`))

		createPayload, err := mapr.ToCreateSuccessResultPayload(createAuthCommand{
			SessionID: "session-1",
			ClientID:  "client-1",
			UserID:    "user-1",
			SagaID:    "saga-1",
		}, "user@example.com", "123456", "10 minutes")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(createPayload)).To(ContainSubstring(`"status":"success"`))
		Expect(string(createPayload)).To(ContainSubstring(`"verification_code":"123456"`))

		deletePayload, err := mapr.ToDeleteFailureResultPayload(deleteAuthCommand{
			UserID: "user-1",
			SagaID: "saga-1",
		}, "auth credentials not found")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(deletePayload)).To(ContainSubstring(`"status":"failed"`))
		Expect(string(deletePayload)).To(ContainSubstring(`"error":"auth credentials not found"`))

		deleteSuccessPayload, err := mapr.ToDeleteSuccessResultPayload(deleteAuthCommand{
			UserID: "user-1",
			SagaID: "saga-1",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(deleteSuccessPayload)).To(ContainSubstring(`"status":"success"`))

		mailPayload, err := mapr.ToMailSendCommandPayload(createAuthCommand{
			SessionID: "session-1",
			ClientID:  "client-1",
			UserID:    "user-1",
		}, "user@example.com", "123456", "10 minutes")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(mailPayload)).To(ContainSubstring(`"message_type":"email_code"`))
	})

	It("maps failure reasons from domain errors", func() {
		resolver := NewDomainFailureReasonResolver()

		Expect(resolver.CreateAuthFailureReason(auth.ErrInvalidUserID)).To(Equal("invalid user_id"))
		Expect(resolver.CreateAuthFailureReason(auth.ErrInvalidEmail)).To(Equal("invalid email"))
		Expect(resolver.CreateAuthFailureReason(auth.ErrInvalidPasswordHash)).To(Equal("invalid password_hash"))
		Expect(resolver.CreateAuthFailureReason(auth.ErrFailedToCreateVerificationCode)).To(Equal("failed to create verification code"))
		Expect(resolver.CreateAuthFailureReason(auth.ErrEmailAlreadyTaken)).To(Equal("email is already taken"))
		Expect(resolver.CreateAuthFailureReason(auth.ErrAuthCredentialsAlreadyExist)).To(Equal("auth credentials already exist"))
		Expect(resolver.CreateAuthFailureReason(errors.New("boom"))).To(Equal("failed to create auth credential"))

		Expect(resolver.DeleteAuthFailureReason(auth.ErrInvalidUserID)).To(Equal("invalid user_id"))
		Expect(resolver.DeleteAuthFailureReason(auth.ErrAuthNotFound)).To(Equal("auth credentials not found"))
		Expect(resolver.DeleteAuthFailureReason(errors.New("boom"))).To(Equal("failed to delete auth credential"))
	})

	It("covers wrapper helpers and constructor validation branches", func() {
		cause := errors.New("boom")

		Expect(WrapConnectToNATSError(cause)).To(MatchError(ContainSubstring("connect to nats")))
		Expect(WrapPublishToNATSError("subject", cause)).To(MatchError(ContainSubstring("publish to nats (subject)")))
		Expect(WrapSubscribeToNATSError("subject", cause)).To(MatchError(ContainSubstring("subscribe to nats (subject)")))
		Expect(WrapFlushNATSPublisherError(cause)).To(MatchError(ContainSubstring("flush nats publisher")))
		Expect(WrapInitJetStreamContextError(cause)).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureConsumerError("stream", "durable", cause, cause)).To(MatchError(ContainSubstring(`ensure consumer "durable" in stream "stream"`)))
		Expect(WrapCreatePullSubscriberError("subject", "durable", cause)).To(MatchError(ContainSubstring("create pull subscriber")))
		Expect(WrapUnmarshalCreateAuthCommandError(cause)).To(MatchError(ContainSubstring("unmarshal create auth command")))
		Expect(WrapUnmarshalDeleteAuthCommandError(cause)).To(MatchError(ContainSubstring("unmarshal delete auth command")))
		Expect(WrapMarshalCreateAuthResultError(cause)).To(MatchError(ContainSubstring("marshal create auth result")))
		Expect(WrapMarshalDeleteAuthResultError(cause)).To(MatchError(ContainSubstring("marshal delete auth result")))
		Expect(WrapMarshalMailSendCommandError(cause)).To(MatchError(ContainSubstring("marshal mail send command")))

		Expect(newPullConsumerConfigValidator()).NotTo(BeNil())
		lg, err := logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		brokerAny, err := NewBroker(config.NATSConfig{}, lg)
		Expect(brokerAny).To(BeNil())
		Expect(err).To(MatchError(ErrEmptyNATSURL))
	})
})
