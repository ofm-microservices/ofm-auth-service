package appfx

import (
	app "auth-service/internal/application"
	auth "auth-service/internal/domain"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// ServiceModule provides the application service used by auth-service.
var ServiceModule = fx.Options(
	fx.Provide(ProvideAuthService),
)

// ProvideAuthService constructs the auth application service.
func ProvideAuthService(repo auth.AuthRepository, lg logging.Logger) (app.AuthService, error) {
	return app.New(repo, lg)
}
