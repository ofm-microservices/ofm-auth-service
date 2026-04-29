package appfx

import (
	auth "auth-service/internal/domain"
	writerepo "auth-service/internal/infra/write/yugabyte"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"
)

// RepoModule wires storage repositories into the FX graph.
var RepoModule = fx.Options(
	fx.Provide(
		writerepo.NewPgErrorTranslator,
		ProvideWriteRepo,
	),
)

// ProvideWriteRepo constructs the Yugabyte-backed auth repository.
func ProvideWriteRepo(dbx *sqlx.DB, translator writerepo.DBErrorTranslator) (auth.AuthRepository, error) {
	return writerepo.New(dbx, translator)
}
