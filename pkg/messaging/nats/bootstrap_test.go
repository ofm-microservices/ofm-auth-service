package nats

import (
	"errors"

	"auth-service/config"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("bootstrap unit cases", func() {
	It("validates missing logger and wraps connect failures", func() {
		err := EnsureStream(config.NATSConfig{}, nil)
		Expect(err).To(MatchError(ErrNilLogger))

		lg, err := logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		Expect(EnsureStream(config.NATSConfig{URL: "nats://127.0.0.1:1"}, lg)).To(HaveOccurred())

		_, err = Connect(config.NATSConfig{URL: "nats://127.0.0.1:1"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})

	It("builds the bootstrap logger context", func() {
		lg, err := logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		Expect(lg).NotTo(BeNil())
	})

	It("preserves wrapped bootstrap causes", func() {
		cause := errors.New("boom")
		Expect(WrapInitJetStreamContextError(cause)).To(MatchError(ContainSubstring("init jetstream context")))
		Expect(WrapEnsureStreamError("STREAM", cause, cause)).To(MatchError(ContainSubstring(`ensure stream "STREAM"`)))
		Expect(WrapConnectToNATSError(cause)).To(MatchError(ContainSubstring("connect to nats")))
	})
})
