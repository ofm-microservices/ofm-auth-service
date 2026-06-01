package service

import (
	"auth-service/internal/domain"
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

// AuthService owns auth credential creation, pending-registration setup, and
// auth data compensation within auth-service.
type AuthService interface {
	// CreateCredential persists the auth credential owned by auth-service.
	CreateCredential(ctx context.Context, userID, email, username, passwordHash string) (*auth.Credential, error)
	// CreatePendingRegistration creates the credential and an email verification
	// code for the first registration slice.
	CreatePendingRegistration(ctx context.Context, userID, email, username, passwordHash string) (*auth.PendingRegistrationResult, error)
	// ExistsByEmail reports whether auth-service already owns the supplied email.
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// GetEmailByUserID returns the stored email for one user.
	GetEmailByUserID(ctx context.Context, userID string) (string, error)
	// VerifyRegistrationEmail verifies the email code for pending registration.
	VerifyRegistrationEmail(ctx context.Context, userID, code string) (*auth.RegistrationEmailVerificationResult, error)
	// IssueRegistrationTokens creates auth-owned login tokens after saga completion.
	IssueRegistrationTokens(ctx context.Context, userID string) (*auth.TokenPair, error)
	// SignIn validates a username or email plus password and issues auth-owned tokens.
	SignIn(ctx context.Context, identifier, password string) (*auth.TokenPair, error)
	// Refresh rotates a refresh token and returns a new auth token pair.
	Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error)
	// DeactivateRegistrationAuth marks registration auth data inactive for compensation.
	DeactivateRegistrationAuth(ctx context.Context, userID string) error
	// DeleteCredential removes auth data for compensation flows.
	DeleteCredential(ctx context.Context, userID string) error
}

// AuthRepository aliases the persistence contract consumed by the application
// layer.
type AuthRepository = auth.AuthRepository

// Logger aliases the shared structured logger used by the application layer.
type Logger = logging.Logger
