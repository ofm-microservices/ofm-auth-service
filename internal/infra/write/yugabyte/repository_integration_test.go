package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"auth-service/config"
	auth "auth-service/internal/domain"
	pkgdb "auth-service/pkg/storage/yugabyte"

	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestYugabyteRepository(t *testing.T) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Yugabyte Repository Suite")
}

var (
	repoSuiteContainer testcontainers.Container
	repoSuiteCfg       config.DBConfig
	repoSuiteDB        *sqlx.DB
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	repoSuiteContainer, repoSuiteCfg = startYugabyteContainer(ctx)
	Expect(pkgdb.RunMigrations(repoSuiteCfg)).To(Succeed())

	var err error
	repoSuiteDB, err = pkgdb.Open(repoSuiteCfg)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if repoSuiteDB != nil {
		Expect(repoSuiteDB.Close()).To(Succeed())
	}
	if repoSuiteContainer != nil {
		Expect(repoSuiteContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("repository integration", func() {
	var repoAny auth.AuthRepository

	BeforeEach(func() {
		_, err := repoSuiteDB.Exec(`TRUNCATE TABLE auth_user_roles, email_verification_codes, refresh_tokens, auth_credentials CASCADE`)
		Expect(err).NotTo(HaveOccurred())

		var errNew error
		repoAny, errNew = New(repoSuiteDB, NewPgErrorTranslator())
		Expect(errNew).NotTo(HaveOccurred())
	})

	It("validates constructor dependencies", func() {
		repo, err := New(nil, NewPgErrorTranslator())
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilYugaByteDB))

		repo, err = New(repoSuiteDB, nil)
		Expect(repo).To(BeNil())
		Expect(err).To(MatchError(ErrNilDBErrorTranslator))
	})

	It("creates and loads credentials", func() {
		credential, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "11111111-1111-1111-1111-111111111111",
			Email:        "user@example.com",
			PasswordHash: "hash",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(credential.EmailVerified).To(BeFalse())
		Expect(credential.CreatedAt.IsZero()).To(BeFalse())

		loaded, err := repoAny.GetByUserID(context.Background(), "11111111-1111-1111-1111-111111111111")
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.UserID).To(Equal(credential.UserID))
		Expect(loaded.Email).To(Equal("user@example.com"))
	})

	It("loads assigned roles for a credential", func() {
		_, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "12121212-1212-1212-1212-121212121212",
			Email:        "roles@example.com",
			PasswordHash: "hash",
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = repoSuiteDB.Exec(`INSERT INTO auth_user_roles (user_id, role) VALUES ($1, $2)`, "12121212-1212-1212-1212-121212121212", auth.RoleAdmin)
		Expect(err).NotTo(HaveOccurred())

		loaded, err := repoAny.GetByUserID(context.Background(), "12121212-1212-1212-1212-121212121212")
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Roles).To(ContainElement(auth.RoleAdmin))
	})

	It("reports credential existence by email", func() {
		_, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "22222222-2222-2222-2222-222222222222",
			Email:        "exists@example.com",
			PasswordHash: "hash",
		})
		Expect(err).NotTo(HaveOccurred())

		exists, err := repoAny.ExistsByEmail(context.Background(), "exists@example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())

		exists, err = repoAny.ExistsByEmail(context.Background(), "missing@example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("wraps lookup and delete execution failures", func() {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		exists, err := repoAny.ExistsByEmail(canceledCtx, "exists@example.com")
		Expect(exists).To(BeFalse())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to find auth credential"))

		err = repoAny.DeleteByUserID(canceledCtx, "11111111-1111-1111-1111-111111111111")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to delete auth credential"))
	})

	It("maps duplicate email and invalid user ids to domain-aware errors", func() {
		_, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "33333333-3333-3333-3333-333333333333",
			Email:        "taken@example.com",
			PasswordHash: "hash",
		})
		Expect(err).NotTo(HaveOccurred())

		credential, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "44444444-4444-4444-4444-444444444444",
			Email:        "taken@example.com",
			PasswordHash: "hash",
		})
		Expect(credential).To(BeNil())
		Expect(err).To(MatchError(auth.ErrEmailAlreadyTaken))

		credential, err = repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "not-a-uuid",
			Email:        "invalid-id@example.com",
			PasswordHash: "hash",
		})
		Expect(credential).To(BeNil())
		Expect(err).To(MatchError(auth.ErrInvalidUserID))
	})

	It("creates verification codes and deletes credentials with cascade", func() {
		_, err := repoAny.Create(context.Background(), auth.CreateCredentialParams{
			UserID:       "55555555-5555-5555-5555-555555555555",
			Email:        "verify@example.com",
			PasswordHash: "hash",
		})
		Expect(err).NotTo(HaveOccurred())

		err = repoAny.CreateVerificationCode(context.Background(), auth.CreateVerificationCodeParams{
			ID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			UserID:    "55555555-5555-5555-5555-555555555555",
			TokenHash: "token-hash",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		Expect(err).NotTo(HaveOccurred())

		var count int
		Expect(repoSuiteDB.Get(&count, `SELECT COUNT(*) FROM email_verification_codes WHERE user_id = $1`, "55555555-5555-5555-5555-555555555555")).To(Succeed())
		Expect(count).To(Equal(1))

		Expect(repoAny.DeleteByUserID(context.Background(), "55555555-5555-5555-5555-555555555555")).To(Succeed())

		Expect(repoSuiteDB.Get(&count, `SELECT COUNT(*) FROM email_verification_codes WHERE user_id = $1`, "55555555-5555-5555-5555-555555555555")).To(Succeed())
		Expect(count).To(Equal(0))
	})

	It("maps lookup and delete misses to auth not found", func() {
		credential, err := repoAny.GetByUserID(context.Background(), "66666666-6666-6666-6666-666666666666")
		Expect(credential).To(BeNil())
		Expect(err).To(MatchError(auth.ErrAuthNotFound))

		err = repoAny.DeleteByUserID(context.Background(), "66666666-6666-6666-6666-666666666666")
		Expect(err).To(MatchError(auth.ErrAuthNotFound))
	})

	It("maps invalid verification-code user ids and missing parents", func() {
		err := repoAny.CreateVerificationCode(context.Background(), auth.CreateVerificationCodeParams{
			ID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			UserID:    "not-a-uuid",
			TokenHash: "token-hash",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		Expect(err).To(MatchError(auth.ErrInvalidUserID))

		err = repoAny.CreateVerificationCode(context.Background(), auth.CreateVerificationCodeParams{
			ID:        "cccccccc-cccc-cccc-cccc-cccccccccccc",
			UserID:    "77777777-7777-7777-7777-777777777777",
			TokenHash: "token-hash",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		Expect(err).To(MatchError(auth.ErrAuthNotFound))
	})
})

func startYugabyteContainer(ctx context.Context) (testcontainers.Container, config.DBConfig) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "yugabytedb/yugabyte:2025.2.2.2-b11",
			ExposedPorts: []string{"5433/tcp"},
			Cmd:          []string{"bin/yugabyted", "start", "--daemon=false"},
			WaitingFor:   wait.ForListeningPort("5433/tcp").WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := container.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := container.MappedPort(ctx, "5433/tcp")
	Expect(err).NotTo(HaveOccurred())

	adminDSN := fmt.Sprintf("postgres://yugabyte@%s:%s/yugabyte?sslmode=disable", host, port.Port())
	var adminDB *sqlx.DB
	Eventually(func() error {
		dbx, openErr := sqlx.Connect("pgx", adminDSN)
		if openErr != nil {
			return openErr
		}
		if pingErr := dbx.Ping(); pingErr != nil {
			_ = dbx.Close()
			return pingErr
		}
		adminDB = dbx
		return nil
	}, 90*time.Second, time.Second).Should(Succeed())
	defer func() {
		if adminDB != nil {
			_ = adminDB.Close()
		}
	}()

	_, err = adminDB.Exec(`
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'admin') THEN
		CREATE ROLE admin WITH LOGIN SUPERUSER PASSWORD 'admin';
	ELSE
		ALTER ROLE admin WITH LOGIN SUPERUSER PASSWORD 'admin';
	END IF;
END
$$;
`)
	Expect(err).NotTo(HaveOccurred())

	var exists int
	Expect(adminDB.Get(&exists, `SELECT COUNT(*) FROM pg_database WHERE datname = 'auth_service'`)).To(Succeed())
	if exists == 0 {
		_, err = adminDB.Exec(`CREATE DATABASE auth_service OWNER admin`)
		Expect(err).NotTo(HaveOccurred())
	}

	dbPort, err := strconv.Atoi(port.Port())
	Expect(err).NotTo(HaveOccurred())

	cfg := config.DBConfig{
		Host:            host,
		Port:            dbPort,
		User:            "admin",
		Password:        "admin",
		Name:            "auth_service",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Minute,
		MigrationsPath:  "file://" + filepath.Join(authServiceRoot(), "migration", "yugabyte"),
		MigrationsTable: "schema_migrations_auth_service",
	}

	Eventually(func() error {
		dbx, openErr := pkgdb.Open(cfg)
		if openErr != nil {
			return openErr
		}
		defer dbx.Close()
		if pingErr := dbx.Ping(); pingErr != nil {
			return pingErr
		}
		var one int
		return dbx.Get(&one, `SELECT 1`)
	}, 3*time.Minute, 2*time.Second).Should(Succeed())

	return container, cfg
}

func authServiceRoot() string {
	_, file, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../"))
}
