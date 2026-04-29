package grpc

import (
	"auth-service/config"
	app "auth-service/internal/application"
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	authv1 "github.com/ofm-microseervices/ofm-common/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	authv1.UnimplementedAuthQueryServiceServer
	svc      app.AuthService
	cfg      config.GRPCConfig
	log      logging.Logger
	srv      *grpc.Server
	listener net.Listener
}

// NewServer constructs the auth-service gRPC query server.
func NewServer(svc app.AuthService, cfg config.GRPCConfig, log logging.Logger) (Server, error) {
	if svc == nil {
		return nil, ErrNilAuthService
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	grpcSrv := grpc.NewServer()
	s := &server{
		svc: svc,
		cfg: cfg,
		log: log.With(logging.String("module", "grpc-server")),
		srv: grpcSrv,
	}
	authv1.RegisterAuthQueryServiceServer(grpcSrv, s)
	return s, nil
}

// Start begins serving gRPC traffic on the configured address.
func (s *server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.listener = lis
	s.log.Info("starting grpc server", logging.String("addr", addr))
	return s.srv.Serve(lis)
}

// Shutdown gracefully stops the gRPC server.
func (s *server) Shutdown(context.Context) error {
	if s.srv != nil {
		s.log.Info("shutting down grpc server")
		s.srv.GracefulStop()
	}
	if s.listener != nil {
		if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	}
	return nil
}

// ExistsByEmail answers whether auth-service already owns the supplied email.
func (s *server) ExistsByEmail(ctx context.Context, req *authv1.ExistsByEmailRequest) (*authv1.ExistsByEmailResponse, error) {
	exists, err := s.svc.ExistsByEmail(ctx, req.GetEmail())
	if err != nil {
		s.log.Error("exists by email failed", logging.Err(err))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &authv1.ExistsByEmailResponse{Exists: exists}, nil
}
