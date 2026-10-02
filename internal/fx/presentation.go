package appfx

import (
	"auth-service/config"
	app "auth-service/internal/application"
	eventbroker "auth-service/internal/presentation/event_broker"
	events "auth-service/internal/presentation/event_broker/kafka"
	grpcserver "auth-service/internal/presentation/grpc"
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"time"

	"go.uber.org/fx"
)

// PresentationModule wires transport adapters and background subscribers into
// the FX lifecycle.
var PresentationModule = fx.Options(
	fx.Provide(
		ProvideRegistrationSagaSubscriber,
		ProvideAuthRecoverySubscriber,
		ProvideGRPCServer,
	),
	fx.Invoke(
		InvokeSubscribeRegistrationSaga,
		InvokeSubscribeAuthRecovery,
		InvokeRunGRPCServer,
	),
)

// ProvideRegistrationSagaSubscriber constructs the Kafka subscriber that
// consumes registration-saga commands.
func ProvideRegistrationSagaSubscriber(
	broker eventbroker.EventBroker,
	service app.AuthService,
	cfg *config.Config,
	lg logging.Logger,
) (events.RegistrationSagaSubscriber, error) {
	return events.NewRegistrationSagaSubscriber(broker, service, cfg.Kafka, lg)
}

// ProvideAuthRecoverySubscriber constructs the auth-owned migration recovery consumer.
func ProvideAuthRecoverySubscriber(broker eventbroker.EventBroker, service app.AuthService, cfg *config.Config, lg logging.Logger) (events.AuthRecoverySubscriber, error) {
	return events.NewAuthRecoverySubscriber(broker, service, cfg.Kafka, lg)
}

// InvokeSubscribeAuthRecovery starts auth recovery consumption during service startup.
func InvokeSubscribeAuthRecovery(lc fx.Lifecycle, subscriber events.AuthRecoverySubscriber, lg logging.Logger) {
	var cancel context.CancelFunc
	lc.Append(fx.Hook{OnStart: func(context.Context) error {
		ctx, stop := context.WithCancel(context.Background())
		cancel = stop
		go func() {
			backoff := time.Second
			for ctx.Err() == nil {
				if err := subscriber.Subscribe(ctx); err != nil && ctx.Err() == nil {
					lg.Error("auth recovery consumer stopped; retrying", logging.Err(err), logging.String("retry_in", backoff.String()))
					timer := time.NewTimer(backoff)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
					if backoff < 30*time.Second {
						backoff *= 2
					}
				} else {
					backoff = time.Second
				}
			}
		}()
		return nil
	}, OnStop: func(context.Context) error {
		if cancel != nil {
			cancel()
		}
		return nil
	}})
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

			go func() {
				backoff := time.Second
				for runCtx.Err() == nil {
					if err := subscriber.Subscribe(runCtx); err == nil {
						backoff = time.Second
					} else if runCtx.Err() == nil {
						lg.Error("subscribe to registration saga commands failed", logging.Err(err), logging.String("retry_in", backoff.String()))
						timer := time.NewTimer(backoff)
						select {
						case <-runCtx.Done():
							timer.Stop()
							return
						case <-timer.C:
						}
						if backoff < 30*time.Second {
							backoff *= 2
							if backoff > 30*time.Second {
								backoff = 30 * time.Second
							}
						}
					}
				}
			}()
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
