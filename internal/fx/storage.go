package appfx

import (
	"auth-service/config"
	db "auth-service/pkg/storage/yugabyte"
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"
)

// StorageModule wires the write-model database and migrations into auth-service.
var StorageModule = fx.Options(
	fx.Invoke(InvokeRunMigrations),
	fx.Provide(ProvideYugaByteDB),
)

// InvokeRunMigrations applies auth-service write-model migrations.
func InvokeRunMigrations(cfg *config.Config, lg logging.Logger) error {
	if err := db.RunMigrations(cfg.DB); err != nil {
		lg.Error("run migrations failed", logging.Err(err))
		return err
	}
	lg.Info("migrations applied")
	return nil
}

// ProvideYugaByteDB opens the YugabyteDB connection owned by auth-service.
func ProvideYugaByteDB(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (*sqlx.DB, error) {
	dbx, err := db.Open(cfg.DB)
	if err != nil {
		lg.Error("open database failed", logging.Err(err))
		return nil, err
	}

	lg.Info("database connected")

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			dbx.Close()
			return nil
		},
	})

	return dbx, nil
}
