package appfx

import (
	"auth-service/config"
	eventbroker "auth-service/internal/presentation/event_broker"
	broker "auth-service/internal/presentation/event_broker/kafka"
	"context"
	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// MessagingModule wires the Kafka event broker into auth-service. Legacy
// JetStream bootstrap is intentionally absent from production FX composition.
var MessagingModule = fx.Options(
	fx.Provide(ProvideEventBrokerWithDB),
)

// InvokeEnsureStream is retained as a compatibility no-op for callers that
// previously bootstrapped JetStream. Kafka topics are provisioned by the
// broker/runtime rather than by a service-specific stream bootstrap.
func InvokeEnsureStream(*config.Config, logging.Logger) error { return nil }

// ProvideEventBroker constructs the concrete Kafka event broker.
func ProvideEventBroker(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (eventbroker.EventBroker, error) {
	return provideEventBroker(lc, cfg, nil, lg)
}

// ProvideEventBrokerWithDB wires durable event claims into Kafka consumers.
func ProvideEventBrokerWithDB(lc fx.Lifecycle, cfg *config.Config, db *sqlx.DB, lg logging.Logger) (eventbroker.EventBroker, error) {
	return provideEventBroker(lc, cfg, db, lg)
}

func provideEventBroker(lc fx.Lifecycle, cfg *config.Config, db *sqlx.DB, lg logging.Logger) (eventbroker.EventBroker, error) {
	eventBroker, err := broker.NewBrokerWithDB(cfg.Kafka, db)
	if err != nil {
		lg.Error("connect kafka failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			eventBroker.Close()
			return nil
		},
	})

	return eventBroker, nil
}
