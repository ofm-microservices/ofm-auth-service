package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"auth-service/config"
	auth "auth-service/internal/domain"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	authv1 "github.com/ofm-microseervices/ofm-common/proto/auth/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestGRPC(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC Suite")
}

var _ = Describe("gRPC server", func() {
	var (
		ctrl   *gomock.Controller
		svc    *MockAuthService
		logger logging.Logger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		svc = NewMockAuthService(ctrl)

		var err error
		logger, err = logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("validates constructor dependencies", func() {
		server, err := NewServer(nil, config.GRPCConfig{}, logger)
		Expect(server).To(BeNil())
		Expect(err).To(MatchError(ErrNilAuthService))

		server, err = NewServer(svc, config.GRPCConfig{}, nil)
		Expect(server).To(BeNil())
		Expect(err).To(MatchError(ErrNilLogger))
	})

	It("answers email existence queries", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())
		srv := srvAny.(*server)

		svc.EXPECT().ExistsByEmail(gomock.Any(), "user@example.com").Return(true, nil)

		resp, err := srv.ExistsByEmail(context.Background(), &authv1.ExistsByEmailRequest{Email: "user@example.com"})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Exists).To(BeTrue())
	})

	It("maps service failures to internal grpc errors", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())
		srv := srvAny.(*server)

		svc.EXPECT().ExistsByEmail(gomock.Any(), "user@example.com").Return(false, auth.ErrFailedToFindCredential)

		resp, err := srv.ExistsByEmail(context.Background(), &authv1.ExistsByEmailRequest{Email: "user@example.com"})
		Expect(resp).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("internal server error"))
	})

	It("shuts down safely without a listener", func() {
		srvAny, err := NewServer(svc, config.GRPCConfig{}, logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(srvAny.Shutdown(context.Background())).To(Succeed())
	})

	It("starts and shuts down a real grpc listener", func() {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		port := lis.Addr().(*net.TCPAddr).Port
		Expect(lis.Close()).To(Succeed())
		addr := fmt.Sprintf("127.0.0.1:%d", port)

		srvAny, err := NewServer(svc, config.GRPCConfig{Host: "127.0.0.1", Port: port}, logger)
		Expect(err).NotTo(HaveOccurred())
		srv := srvAny.(*server)

		done := make(chan error, 1)
		go func() {
			done <- srv.Start()
		}()

		Eventually(func() bool {
			conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
			if dialErr != nil {
				return false
			}
			_ = conn.Close()
			return true
		}).Should(BeTrue())

		Expect(srv.Shutdown(context.Background())).To(Succeed())
		Eventually(done).Should(Receive(BeNil()))
	})
})
