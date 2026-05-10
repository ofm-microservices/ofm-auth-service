package config

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Load", func() {
	var prevWD string

	BeforeEach(func() {
		var err error
		prevWD, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		tmpDir := GinkgoT().TempDir()
		Expect(os.Chdir(tmpDir)).To(Succeed())

		for _, key := range []string{
			"APP_ENV", "LOG_LEVEL",
			"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
			"GRPC_HOST", "GRPC_PORT",
			"NATS_URL",
		} {
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	})

	AfterEach(func() {
		Expect(os.Chdir(prevWD)).To(Succeed())
	})

	It("loads explicit environment values", func() {
		Expect(os.Setenv("APP_ENV", "test")).To(Succeed())
		Expect(os.Setenv("LOG_LEVEL", "debug")).To(Succeed())
		Expect(os.Setenv("DB_HOST", "127.0.0.1")).To(Succeed())
		Expect(os.Setenv("DB_PORT", "5434")).To(Succeed())
		Expect(os.Setenv("DB_USER", "admin")).To(Succeed())
		Expect(os.Setenv("DB_PASSWORD", "admin")).To(Succeed())
		Expect(os.Setenv("DB_NAME", "auth_service")).To(Succeed())
		Expect(os.Setenv("GRPC_HOST", "127.0.0.1")).To(Succeed())
		Expect(os.Setenv("GRPC_PORT", "9501")).To(Succeed())
		Expect(os.Setenv("NATS_URL", "nats://127.0.0.1:4222")).To(Succeed())

		cfg, err := Load()

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.App.Env).To(Equal("test"))
		Expect(cfg.App.LogLevel).To(Equal("debug"))
		Expect(cfg.DB.Host).To(Equal("127.0.0.1"))
		Expect(cfg.DB.Port).To(Equal(5434))
		Expect(cfg.DB.User).To(Equal("admin"))
		Expect(cfg.DB.Password).To(Equal("admin"))
		Expect(cfg.DB.Name).To(Equal("auth_service"))
		Expect(cfg.GRPC.Host).To(Equal("127.0.0.1"))
		Expect(cfg.GRPC.Port).To(Equal(9501))
		Expect(cfg.NATS.URL).To(Equal("nats://127.0.0.1:4222"))
		Expect(cfg.NATS.SagaBatchSize).To(Equal(32))
	})

	It("wraps env parsing failures", func() {
		Expect(os.Setenv("DB_PORT", "bad-port")).To(Succeed())

		cfg, err := Load()

		Expect(cfg).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parse env config"))
	})
})
