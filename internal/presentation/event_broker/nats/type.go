package nats

import (
	"auth-service/config"
	eventbroker "auth-service/internal/presentation/event_broker"
	"context"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
)

// RegistrationSagaSubscriber consumes auth-related saga commands from NATS.
type RegistrationSagaSubscriber interface {
	Subscribe(ctx context.Context) error
}

// RegistrationSagaMessageMapper translates auth registration results into NATS
// payloads consumed by the saga and mail services.
type RegistrationSagaMessageMapper interface {
	ToCreateFailureResultPayload(cmd createAuthCommand, reason string) ([]byte, error)
	ToCreateSuccessResultPayload(cmd createAuthCommand, email, verificationCode, expiresIn string) ([]byte, error)
	ToDeleteFailureResultPayload(cmd deleteAuthCommand, reason string) ([]byte, error)
	ToDeleteSuccessResultPayload(cmd deleteAuthCommand) ([]byte, error)
	ToMailSendCommandPayload(cmd createAuthCommand, email, verificationCode, expiresIn string) ([]byte, error)
}

// PullConsumerConfigValidator validates pull-consumer runtime configuration
// before the broker touches JetStream state.
type PullConsumerConfigValidator interface {
	Validate(cfg config.PullConsumerConfig) error
}

// PullConsumerRuntime represents one configured JetStream pull-consumer
// runtime.
type PullConsumerRuntime interface {
	Start(ctx context.Context)
}

// PullConsumerRuntimeFactory builds the runtime used by the broker after the
// config is validated.
type PullConsumerRuntimeFactory interface {
	Create(
		nc *nats.Conn,
		log logging.Logger,
		cfg config.PullConsumerConfig,
		handler eventbroker.MessageHandler,
	) (PullConsumerRuntime, error)
}
