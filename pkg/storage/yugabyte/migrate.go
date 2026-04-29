package db

import (
	"auth-service/config"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/yugabytedb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies the auth-service Yugabyte schema migrations.
func RunMigrations(cfg config.DBConfig) error {
	dsn := fmt.Sprintf(
		"yugabytedb://%s:%s@%s:%d/%s?sslmode=%s&x-migrations-table=%s",
		url.QueryEscape(cfg.User),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.SSLMode,
		url.QueryEscape(cfg.MigrationsTable),
	)

	migrationPath := cfg.MigrationsPath
	if after, ok := strings.CutPrefix(migrationPath, "file://"); ok {
		raw := after
		if !filepath.IsAbs(raw) {
			abs, err := filepath.Abs(raw)
			if err != nil {
				return WrapResolveMigrationsPathError(err)
			}
			migrationPath = "file://" + abs
		}
	}

	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		return WrapCreateMigratorError(err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return WrapRunMigrationsError(err)
	}

	return nil
}
