package appfx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"auth-service/config"
	repository "auth-service/internal/infra/write/yugabyte"

	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
)

func TestFX(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "FX Suite")
}

var (
	fxNATSContainer testcontainers.Container
	fxNATSCfg       config.NATSConfig
	fxYBContainer   testcontainers.Container
	fxYBCfg         config.DBConfig
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fxNATSContainer, fxNATSCfg = startFXNATSContainer(ctx)
	fxYBContainer, fxYBCfg = startFXYugabyteContainer(ctx)
})

var _ = AfterSuite(func() {
	if fxNATSContainer != nil {
		Expect(fxNATSContainer.Terminate(context.Background())).To(Succeed())
	}
	if fxYBContainer != nil {
		Expect(fxYBContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("fx providers and invokes", func() {
	var (
		ctrl   *gomock.Controller
		logger logging.Logger
		cfg    *config.Config
		lc     *fxtest.Lifecycle
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		var err error
		logger, err = logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		cfg = &config.Config{
			App: config.AppConfig{
				Env:      "test",
				LogLevel: "debug",
			},
			DB: config.DBConfig{
				Host:            "127.0.0.1",
				Port:            1,
				User:            "admin",
				Password:        "admin",
				Name:            "auth_service",
				SSLMode:         "disable",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Minute,
				MigrationsPath:  "file:///missing",
				MigrationsTable: "schema_migrations_auth_service",
			},
			GRPC: config.GRPCConfig{
				Host: "127.0.0.1",
				Port: 19591,
			},
			NATS: config.NATSConfig{
				URL:                         "nats://127.0.0.1:4222",
				AuthEventsStream:            "AUTH_EVENTS",
				MailCommandsStream:          "MAIL_COMMANDS",
				SagaCommandsStream:          "SAGA_AUTH_COMMANDS",
				AuthCreatedSubject:          "auth.created",
				SagaCreateAuthSubject:       "saga.auth.create_pending_registration",
				SagaDeleteAuthSubject:       "saga.auth.delete",
				SagaCreateAuthResultSubject: "saga.auth.create_pending_registration.result",
				SagaDeleteAuthResultSubject: "saga.auth.delete.result",
				MailSendSubject:             "mail.send",
				SagaCreateAuthDurable:       "auth_service_saga_create_pending_registration",
				SagaDeleteAuthDurable:       "auth_service_saga_delete",
				SagaBatchSize:               1,
				SagaMaxWait:                 time.Millisecond,
				SagaWorkers:                 1,
				SagaQueueSize:               1,
				SagaAckWait:                 time.Second,
				SagaMaxDeliver:              1,
			},
			Kafka: config.KafkaConfig{Brokers: []string{"127.0.0.1:9092"}, GroupID: "auth-test"},
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "ofm-auth-service",
			},
		}

		lc = fxtest.NewLifecycle(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("logs startup", func() {
		InvokeStartLog(logger)
	})

	It("provides config from environment", func() {
		Expect(os.Setenv("DB_HOST", "127.0.0.1")).To(Succeed())
		Expect(os.Setenv("DB_PORT", "5433")).To(Succeed())
		Expect(os.Setenv("DB_USER", "admin")).To(Succeed())
		Expect(os.Setenv("DB_PASSWORD", "admin")).To(Succeed())
		Expect(os.Setenv("DB_NAME", "auth_service")).To(Succeed())
		Expect(os.Setenv("NATS_URL", "nats://127.0.0.1:4222")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv("DB_HOST")).To(Succeed())
			Expect(os.Unsetenv("DB_PORT")).To(Succeed())
			Expect(os.Unsetenv("DB_USER")).To(Succeed())
			Expect(os.Unsetenv("DB_PASSWORD")).To(Succeed())
			Expect(os.Unsetenv("DB_NAME")).To(Succeed())
			Expect(os.Unsetenv("NATS_URL")).To(Succeed())
		})

		loaded, err := ProvideConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.DB.Host).To(Equal("127.0.0.1"))
	})

	It("provides a logger and appends a stop hook", func() {
		provided, err := ProvideLogger(lc, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(provided).NotTo(BeNil())
		Expect(lc.Stop(context.Background())).To(Succeed())
	})

	It("fails logger construction on invalid log levels", func() {
		badCfg := *cfg
		badCfg.App.LogLevel = "definitely-invalid"

		provided, err := ProvideLogger(lc, &badCfg)
		Expect(provided).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("constructs the application service", func() {
		repo := NewMockAuthRepository(ctrl)

		svc, err := ProvideAuthService(repo, cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(svc).NotTo(BeNil())
	})

	It("constructs the write repository", func() {
		repoAny, err := ProvideWriteRepo(&sqlx.DB{}, repository.NewPgErrorTranslator())
		Expect(err).NotTo(HaveOccurred())
		Expect(repoAny).NotTo(BeNil())
	})

	It("constructs presentation adapters", func() {
		broker := NewMockEventBroker(ctrl)
		svc := NewMockAuthService(ctrl)

		subscriber, err := ProvideRegistrationSagaSubscriber(broker, svc, cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(subscriber).NotTo(BeNil())

		server, err := ProvideGRPCServer(svc, cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(server).NotTo(BeNil())
	})

	It("registers subscriber lifecycle hooks and runs start-stop cleanly", func() {
		subscriber := &registrationSagaSubscriberStub{}

		InvokeSubscribeRegistrationSaga(lc, subscriber, cfg, logger)

		Expect(lc.Start(context.Background())).To(Succeed())
		Expect(lc.Stop(context.Background())).To(Succeed())
		Eventually(func() int { return subscriber.calls }).Should(Equal(1))
	})

	It("propagates subscriber startup failures", func() {
		subscriber := &registrationSagaSubscriberStub{err: errors.New("boom")}

		InvokeSubscribeRegistrationSaga(lc, subscriber, cfg, logger)

		Expect(lc.Start(context.Background())).To(Succeed())
		Eventually(func() int { return subscriber.calls }).Should(Equal(1))
	})

	It("registers grpc lifecycle hooks and stops the server", func() {
		server := NewMockServer(ctrl)
		started := make(chan struct{}, 1)

		server.EXPECT().Start().DoAndReturn(func() error {
			started <- struct{}{}
			return nil
		})
		server.EXPECT().Shutdown(gomock.Any()).Return(nil)

		InvokeRunGRPCServer(lc, server)

		Expect(lc.Start(context.Background())).To(Succeed())
		Eventually(started).Should(Receive())
		Expect(lc.Stop(context.Background())).To(Succeed())
	})

	It("fails migration invocation and database open with bad config", func() {
		Expect(InvokeRunMigrations(cfg, logger)).To(HaveOccurred())

		dbx, err := ProvideYugaByteDB(lc, cfg, logger)
		Expect(dbx).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("opens a Kafka event broker without service-specific stream bootstrap", func() {
		Expect(InvokeEnsureStream(cfg, logger)).To(Succeed())

		eventBroker, err := ProvideEventBroker(lc, cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(eventBroker).NotTo(BeNil())
		Expect(lc.Stop(context.Background())).To(Succeed())
	})

	It("rejects an empty Kafka broker configuration", func() {
		badCfg := *cfg
		badCfg.Kafka.Brokers = nil
		eventBroker, err := ProvideEventBroker(lc, &badCfg, logger)
		Expect(eventBroker).To(BeNil())
		Expect(err).To(HaveOccurred())
	})

	It("runs migrations and opens a real yugabyte connection", func() {
		cfg.DB = fxYBCfg

		Expect(InvokeRunMigrations(cfg, logger)).To(Succeed())

		dbx, err := ProvideYugaByteDB(lc, cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbx.Ping()).To(Succeed())
		Expect(lc.Stop(context.Background())).To(Succeed())
	})
})

type registrationSagaSubscriberStub struct {
	err   error
	calls int
}

func (s *registrationSagaSubscriberStub) Subscribe(context.Context) error {
	s.calls++
	return s.err
}

func startFXNATSContainer(ctx context.Context) (testcontainers.Container, config.NATSConfig) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.12.4-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := container.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := container.MappedPort(ctx, "4222/tcp")
	Expect(err).NotTo(HaveOccurred())

	return container, config.NATSConfig{
		URL:                         "nats://" + host + ":" + port.Port(),
		AuthEventsStream:            "AUTH_EVENTS_FX",
		MailCommandsStream:          "MAIL_COMMANDS_FX",
		SagaCommandsStream:          "SAGA_AUTH_COMMANDS_FX",
		AuthCreatedSubject:          "auth.created.fx",
		SagaCreateAuthSubject:       "saga.auth.create_pending_registration.fx",
		SagaDeleteAuthSubject:       "saga.auth.delete.fx",
		SagaCreateAuthResultSubject: "saga.auth.create_pending_registration.result.fx",
		SagaDeleteAuthResultSubject: "saga.auth.delete.result.fx",
		MailSendSubject:             "mail.send.fx",
		SagaCreateAuthDurable:       "auth_service_saga_create_pending_registration_fx",
		SagaDeleteAuthDurable:       "auth_service_saga_delete_fx",
		SagaBatchSize:               1,
		SagaMaxWait:                 time.Millisecond,
		SagaWorkers:                 1,
		SagaQueueSize:               1,
		SagaAckWait:                 time.Second,
		SagaMaxDeliver:              1,
	}
}

func startFXYugabyteContainer(ctx context.Context) (testcontainers.Container, config.DBConfig) {
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
	Expect(adminDB.Get(&exists, `SELECT COUNT(*) FROM pg_database WHERE datname = 'auth_service_fx'`)).To(Succeed())
	if exists == 0 {
		_, err = adminDB.Exec(`CREATE DATABASE auth_service_fx OWNER admin`)
		Expect(err).NotTo(HaveOccurred())
	}

	dbPort, err := strconv.Atoi(port.Port())
	Expect(err).NotTo(HaveOccurred())

	return container, config.DBConfig{
		Host:            host,
		Port:            dbPort,
		User:            "admin",
		Password:        "admin",
		Name:            "auth_service_fx",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Minute,
		MigrationsPath:  "file://" + filepath.Join(authServiceRoot(), "migration", "yugabyte"),
		MigrationsTable: "schema_migrations_auth_service_fx",
	}
}

func authServiceRoot() string {
	_, file, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../"))
}
