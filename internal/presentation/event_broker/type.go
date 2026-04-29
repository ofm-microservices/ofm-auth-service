package eventbroker

import (
	"auth-service/config"
	"context"
)

// MessageHandler handles a single broker message in a transport-agnostic form.
type MessageHandler func(ctx context.Context, subject string, payload []byte) error

// EventBroker is the transport-agnostic contract used by auth-service
// presentation adapters.
type EventBroker interface {
	Publish(ctx context.Context, subject string, payload []byte) error
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error
	RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler MessageHandler) error
	Close()
}
