package db

import (
	"auth-service/config"
	"errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"path/filepath"
)

var _ = Describe("RunMigrations", func() {
	It("wraps migrator construction failures", func() {
		cfg := config.DBConfig{
			User:            "user",
			Password:        "pass",
			Host:            "127.0.0.1",
			Port:            5433,
			Name:            "auth",
			SSLMode:         "disable",
			MigrationsTable: "schema_migrations",
			MigrationsPath:  "file:///definitely/does/not/exist",
		}

		Expect(RunMigrations(cfg)).To(HaveOccurred())
	})

	It("resolves relative file migration paths before construction", func() {
		abs, err := filepath.Abs(".")
		Expect(err).NotTo(HaveOccurred())
		Expect(WrapResolveMigrationsPathError(errors.New("boom"))).To(MatchError(ContainSubstring("resolve migrations path")))
		Expect(WrapCreateMigratorError(errors.New("boom"))).To(MatchError(ContainSubstring("create migrator")))
		Expect(WrapRunMigrationsError(errors.New("boom"))).To(MatchError(ContainSubstring("run migrations")))
		Expect(WrapOpenDBError(errors.New("boom"))).To(MatchError(ContainSubstring("open db")))
		Expect(abs).NotTo(BeEmpty())
	})
})
