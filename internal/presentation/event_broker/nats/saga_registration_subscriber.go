package nats

import (
	"auth-service/config"
	application "auth-service/internal/application"
	eventbroker "auth-service/internal/presentation/event_broker"
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type createAuthResult struct {
	SessionID        string `json:"session_id"`
	ClientID         string `json:"client_id"`
	UserID           string `json:"user_id"`
	SagaID           string `json:"saga_id,omitempty"`
	Email            string `json:"email,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
	ExpiresIn        string `json:"expires_in,omitempty"`
	Status           string `json:"status"`
	Error            string `json:"error,omitempty"`
	Timestamp        string `json:"timestamp"`
}

type deleteAuthResult struct {
	UserID    string `json:"user_id"`
	SagaID    string `json:"saga_id,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

type registrationSagaSubscriber struct {
	broker   eventbroker.EventBroker
	service  application.AuthService
	cfg      config.NATSConfig
	resolver FailureReasonResolver
	mapr     RegistrationSagaMessageMapper
	log      logging.Logger
}

// NewRegistrationSagaSubscriber constructs the auth-side registration saga
// subscriber.
func NewRegistrationSagaSubscriber(
	broker eventbroker.EventBroker,
	service application.AuthService,
	cfg config.NATSConfig,
	resolver FailureReasonResolver,
	log logging.Logger,
) (RegistrationSagaSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if service == nil {
		return nil, ErrNilAuthService
	}
	if resolver == nil {
		return nil, ErrNilFailureReasonResolver
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &registrationSagaSubscriber{
		broker:   broker,
		service:  service,
		cfg:      cfg,
		resolver: resolver,
		mapr:     newRegistrationSagaMessageMapper(),
		log:      log.With(logging.String("module", "registration-saga-subscriber")),
	}, nil
}

// Subscribe starts the pull consumers for auth create/delete saga commands.
func (s *registrationSagaSubscriber) Subscribe(ctx context.Context) error {
	s.log.Info("registering saga pull consumers",
		logging.String("create_subject", s.cfg.SagaCreateAuthSubject),
		logging.String("delete_subject", s.cfg.SagaDeleteAuthSubject),
		logging.Int("batch_size", s.cfg.SagaBatchSize),
		logging.Any("max_wait", s.cfg.SagaMaxWait),
		logging.Int("workers", s.cfg.SagaWorkers),
	)

	if err := s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(s.cfg.SagaCreateAuthSubject, s.cfg.SagaCreateAuthDurable),
		s.handleCreateAuthCommand,
	); err != nil {
		return err
	}

	if err := s.broker.RunPullConsumer(
		ctx,
		s.buildPullConsumerConfig(s.cfg.SagaDeleteAuthSubject, s.cfg.SagaDeleteAuthDurable),
		s.handleDeleteAuthCommand,
	); err != nil {
		return err
	}

	s.log.Info("saga pull consumers ready")
	return nil
}

func (s *registrationSagaSubscriber) buildPullConsumerConfig(subject, durable string) config.PullConsumerConfig {
	return config.PullConsumerConfig{
		Stream:     s.cfg.SagaCommandsStream,
		Subject:    subject,
		Durable:    durable,
		BatchSize:  s.cfg.SagaBatchSize,
		MaxWait:    s.cfg.SagaMaxWait,
		Workers:    s.cfg.SagaWorkers,
		QueueSize:  s.cfg.SagaQueueSize,
		AckWait:    s.cfg.SagaAckWait,
		MaxDeliver: s.cfg.SagaMaxDeliver,
		Adaptive:   BuildAdaptiveConfig(s.cfg),
	}
}

// BuildAdaptiveConfig projects NATS config values into a generic adaptive pull
// consumer config.
func BuildAdaptiveConfig(cfg config.NATSConfig) config.PullAdaptiveConfig {
	return config.PullAdaptiveConfig{
		Enabled:         cfg.SagaAdaptiveEnabled,
		CheckInterval:   cfg.SagaAdaptiveCheckInterval,
		MediumPending:   cfg.SagaAdaptiveMediumPending,
		HighPending:     cfg.SagaAdaptiveHighPending,
		LowBatchSize:    cfg.SagaAdaptiveLowBatchSize,
		LowMaxWait:      cfg.SagaAdaptiveLowMaxWait,
		MediumBatchSize: cfg.SagaAdaptiveMediumBatchSize,
		MediumMaxWait:   cfg.SagaAdaptiveMediumMaxWait,
		HighBatchSize:   cfg.SagaAdaptiveHighBatchSize,
		HighMaxWait:     cfg.SagaAdaptiveHighMaxWait,
	}
}
