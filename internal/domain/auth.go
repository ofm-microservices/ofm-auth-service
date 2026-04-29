package auth

import (
	"context"
	"time"
)

// Credential is the auth-service write model for one user's credentials.
type Credential struct {
	UserID        string
	Email         string
	PasswordHash  string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateCredentialParams contains the data required to create a credential row.
type CreateCredentialParams struct {
	UserID       string
	Email        string
	PasswordHash string
}

// CreateVerificationCodeParams contains the data required to persist a hashed
// email-verification code.
type CreateVerificationCodeParams struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// PendingRegistrationResult is returned after auth data and the verification
// code are created for a new registration session.
type PendingRegistrationResult struct {
	UserID           string
	Email            string
	VerificationCode string
	ExpiresIn        string
}

// AuthRepository persists auth-service credentials and verification codes.
type AuthRepository interface {
	Create(ctx context.Context, params CreateCredentialParams) (*Credential, error)
	CreateVerificationCode(ctx context.Context, params CreateVerificationCodeParams) error
	GetByUserID(ctx context.Context, userID string) (*Credential, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	DeleteByUserID(ctx context.Context, userID string) error
}
