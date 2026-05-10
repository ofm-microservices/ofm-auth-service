package appfx

import "go.uber.org/fx"

// Module is the compatibility bundle of all FX modules used by auth-service.
var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	MetricsModule,
	AppModule,
	StorageModule,
	MessagingModule,
	RepoModule,
	ServiceModule,
	PresentationModule,
)
