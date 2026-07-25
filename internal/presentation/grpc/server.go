package grpc

import (
	"auth-service/config"
	app "auth-service/internal/application"
	auth "auth-service/internal/domain"
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	authv1 "github.com/ofm-microservices/ofm-common/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	authv1.UnimplementedAuthQueryServiceServer
	authv1.UnimplementedAuthSessionServiceServer
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

	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()))
	s := &server{
		svc: svc,
		cfg: cfg,
		log: log.With(logging.String("module", "grpc-server")),
		srv: grpcSrv,
	}
	authv1.RegisterAuthQueryServiceServer(grpcSrv, s)
	authv1.RegisterAuthSessionServiceServer(grpcSrv, s)
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

// GetEmailByUserID returns the stored email address for one user.
func (s *server) GetEmailByUserID(ctx context.Context, req *authv1.GetEmailByUserIDRequest) (*authv1.GetEmailByUserIDResponse, error) {
	email, err := s.svc.GetEmailByUserID(ctx, req.GetUserId())
	if err != nil {
		s.log.Error("get email by user id failed", logging.Err(err))
		return nil, status.Error(codes.NotFound, "auth credential not found")
	}
	return &authv1.GetEmailByUserIDResponse{UserId: req.GetUserId(), Email: email}, nil
}

// VerifyRegistrationEmail verifies the pending registration email code.
func (s *server) VerifyRegistrationEmail(ctx context.Context, req *authv1.VerifyRegistrationEmailRequest) (*authv1.VerifyRegistrationEmailResponse, error) {
	result, err := s.svc.VerifyRegistrationEmail(ctx, req.GetUserId(), req.GetCode())
	if err != nil {
		s.log.Error("verify registration email failed", logging.Err(err))
		return nil, status.Error(codes.InvalidArgument, "invalid registration email verification")
	}

	return &authv1.VerifyRegistrationEmailResponse{
		UserId: result.UserID,
		Email:  result.Email,
		Status: result.Status,
	}, nil
}

// IssueRegistrationTokens creates the final login tokens for a completed
// registration.
func (s *server) IssueRegistrationTokens(ctx context.Context, req *authv1.IssueRegistrationTokensRequest) (*authv1.IssueRegistrationTokensResponse, error) {
	tokens, err := s.svc.IssueRegistrationTokens(ctx, req.GetUserId())
	if err != nil {
		s.log.Error("issue registration tokens failed", logging.Err(err))
		return nil, status.Error(codes.FailedPrecondition, "registration tokens unavailable")
	}

	return &authv1.IssueRegistrationTokensResponse{
		UserId:       tokens.UserID,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

// SignIn validates credentials and returns auth-owned tokens.
func (s *server) SignIn(ctx context.Context, req *authv1.SignInRequest) (*authv1.SignInResponse, error) {
	tokens, err := s.svc.SignIn(ctx, req.GetIdentifier(), req.GetPassword())
	if err != nil {
		s.log.Error("sign in failed", logging.Err(err))
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return &authv1.SignInResponse{
		UserId:       tokens.UserID,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

// Refresh rotates a refresh token and returns a new auth-owned token pair.
func (s *server) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	tokens, err := s.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		s.log.Error("refresh failed", logging.Err(err))
		switch err {
		case auth.ErrInvalidRefreshToken:
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		case auth.ErrRefreshTokenExpired, auth.ErrRefreshTokenRevoked, auth.ErrInvalidCredentials, auth.ErrAuthNotFound:
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &authv1.RefreshResponse{
		UserId:       tokens.UserID,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

// SignOut revokes a refresh token and ends the session.
func (s *server) SignOut(ctx context.Context, req *authv1.SignOutRequest) (*authv1.SignOutResponse, error) {
	if err := s.svc.SignOut(ctx, req.GetRefreshToken()); err != nil {
		s.log.Error("sign out failed", logging.Err(err))
		switch err {
		case auth.ErrInvalidRefreshToken:
			return nil, status.Error(codes.InvalidArgument, "invalid refresh token")
		case auth.ErrRefreshTokenExpired, auth.ErrRefreshTokenRevoked, auth.ErrInvalidCredentials, auth.ErrAuthNotFound:
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &authv1.SignOutResponse{
		Status: "signed_out",
	}, nil
}

// DeactivateRegistrationAuth marks auth registration data inactive for saga
// compensation.
func (s *server) DeactivateRegistrationAuth(ctx context.Context, req *authv1.DeactivateRegistrationAuthRequest) (*authv1.DeactivateRegistrationAuthResponse, error) {
	if err := s.svc.DeactivateRegistrationAuth(ctx, req.GetUserId()); err != nil {
		s.log.Error("deactivate registration auth failed", logging.Err(err))
		return nil, status.Error(codes.Internal, "failed to deactivate registration auth")
	}

	return &authv1.DeactivateRegistrationAuthResponse{UserId: req.GetUserId(), Status: "registration_failed"}, nil
}
