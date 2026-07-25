package appfx

import (
	"auth-service/config"
	app "auth-service/internal/application"
	eventbroker "auth-service/internal/presentation/event_broker"
	events "auth-service/internal/presentation/event_broker/nats"
	grpcserver "auth-service/internal/presentation/grpc"
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// PresentationModule wires transport adapters and background subscribers into
// the FX lifecycle.
var PresentationModule = fx.Options(
	fx.Provide(
		events.NewDomainFailureReasonResolver,
		ProvideRegistrationSagaSubscriber,
		ProvideGRPCServer,
	),
	fx.Invoke(
		InvokeSubscribeRegistrationSaga,
		InvokeRunGRPCServer,
	),
)

// ProvideRegistrationSagaSubscriber constructs the NATS subscriber that
// consumes registration-saga commands.
func ProvideRegistrationSagaSubscriber(
	broker eventbroker.EventBroker,
	service app.AuthService,
	cfg *config.Config,
	resolver events.FailureReasonResolver,
	lg logging.Logger,
) (events.RegistrationSagaSubscriber, error) {
	return events.NewRegistrationSagaSubscriber(broker, service, cfg.NATS, resolver, lg)
}

// ProvideGRPCServer constructs the gRPC query server exposed by auth-service.
func ProvideGRPCServer(
	service app.AuthService,
	cfg *config.Config,
	lg logging.Logger,
) (grpcserver.Server, error) {
	return grpcserver.NewServer(service, cfg.GRPC, lg)
}

// InvokeSubscribeRegistrationSaga starts background consumers for registration
// saga commands.
func InvokeSubscribeRegistrationSaga(
	lc fx.Lifecycle,
	subscriber events.RegistrationSagaSubscriber,
	cfg *config.Config,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			if err := subscriber.Subscribe(runCtx); err != nil {
				lg.Error("subscribe to registration saga commands failed", logging.Err(err))
				cancel()
				return err
			}

			lg.Info("auth-service initialized", logging.String("env", cfg.App.Env))
			return nil
		},
		OnStop: func(context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
}

// InvokeRunGRPCServer starts and gracefully stops the gRPC server with the FX
// lifecycle.
func InvokeRunGRPCServer(lc fx.Lifecycle, srv grpcserver.Server) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.Start(); err != nil {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}
