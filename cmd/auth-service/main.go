package main

import (
	appfx "auth-service/internal/fx"

	"go.uber.org/fx"
)

var run = func(opts ...fx.Option) {
	fx.New(opts...).Run()
}

func main() {
	run(
		appfx.ConfigModule,
		appfx.LoggerModule,
		appfx.AppModule,
		appfx.StorageModule,
		appfx.MessagingModule,
		appfx.RepoModule,
		appfx.ServiceModule,
		appfx.PresentationModule,
	)
}
