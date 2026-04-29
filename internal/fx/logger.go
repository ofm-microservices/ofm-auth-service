package appfx

import (
	"auth-service/config"
	"context"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// LoggerModule provides the structured logger used across auth-service.
var LoggerModule = fx.Options(
	fx.Provide(ProvideLogger),
)

// ProvideLogger builds the service logger from runtime configuration.
func ProvideLogger(lc fx.Lifecycle, cfg *config.Config) (logging.Logger, error) {
	lg, err := logging.New("auth-service", cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return lg.Sync()
		},
	})

	return lg, nil
}
