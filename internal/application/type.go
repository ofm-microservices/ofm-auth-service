package service

import (
	"auth-service/internal/domain"
	"context"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
)

// AuthService owns auth credential creation, pending-registration setup, and
// auth data compensation within auth-service.
type AuthService interface {
	// CreateCredential persists the auth credential owned by auth-service.
	CreateCredential(ctx context.Context, userID, email, passwordHash string) (*auth.Credential, error)
	// CreatePendingRegistration creates the credential and an email verification
	// code for the first registration slice.
	CreatePendingRegistration(ctx context.Context, userID, email, passwordHash string) (*auth.PendingRegistrationResult, error)
	// ExistsByEmail reports whether auth-service already owns the supplied email.
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// DeleteCredential removes auth data for compensation flows.
	DeleteCredential(ctx context.Context, userID string) error
}

// AuthRepository aliases the persistence contract consumed by the application
// layer.
type AuthRepository = auth.AuthRepository

// Logger aliases the shared structured logger used by the application layer.
type Logger = logging.Logger
