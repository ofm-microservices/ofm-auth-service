package auth

import (
	"context"
	"time"
)

const (
	// RoleAdmin marks a credential that may access admin-only flows.
	RoleAdmin = "admin"
	// CredentialStatusPendingRegistration marks auth data created before email
	// verification finishes.
	CredentialStatusPendingRegistration = "pending_registration"
	// CredentialStatusEmailVerified marks auth data after the email code is
	// accepted and before final token issuing.
	CredentialStatusEmailVerified = "email_verified"
	// CredentialStatusRegistrationFailed marks auth data compensated by the
	// registration saga.
	CredentialStatusRegistrationFailed = "registration_failed"
)

// Credential is the auth-service write model for one user's credentials.
type Credential struct {
	UserID        string
	Email         string
	Username      string
	PasswordHash  string
	EmailVerified bool
	Status        string
	Roles         []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateCredentialParams contains the data required to create a credential row.
type CreateCredentialParams struct {
	UserID       string
	Email        string
	Username     string
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

// RegistrationEmailVerificationResult reports a successful verification-code
// check for a pending registration.
type RegistrationEmailVerificationResult struct {
	UserID string
	Email  string
	Status string
}

// TokenPair is the auth-owned login token result returned after registration
// completion.
type TokenPair struct {
	UserID       string
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

// RefreshToken identifies the persisted refresh-token record owned by
// auth-service.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// CreateRefreshTokenParams contains the data required to persist a refresh
// token record.
type CreateRefreshTokenParams struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// RotateRefreshTokenParams contains the data required to revoke the old
// refresh token and persist the rotated one atomically.
type RotateRefreshTokenParams struct {
	CurrentTokenHash string
	NewTokenID       string
	NewTokenHash     string
	NewExpiresAt     time.Time
	Now              time.Time
}

// RevokeRefreshTokenParams contains the data required to revoke an existing
// refresh token without minting a replacement.
type RevokeRefreshTokenParams struct {
	CurrentTokenHash string
	Now              time.Time
}

// AuthRepository persists auth-service credentials and verification codes.
type AuthRepository interface {
	Create(ctx context.Context, params CreateCredentialParams) (*Credential, error)
	CreateVerificationCode(ctx context.Context, params CreateVerificationCodeParams) error
	VerifyRegistrationEmail(ctx context.Context, userID, tokenHash string, now time.Time) (*Credential, error)
	CreateRefreshToken(ctx context.Context, params CreateRefreshTokenParams) error
	RotateRefreshToken(ctx context.Context, params RotateRefreshTokenParams) (*Credential, error)
	RevokeRefreshToken(ctx context.Context, params RevokeRefreshTokenParams) (*Credential, error)
	GetByUserID(ctx context.Context, userID string) (*Credential, error)
	GetByIdentifier(ctx context.Context, identifier string) (*Credential, error)
	ListRolesByUserID(ctx context.Context, userID string) ([]string, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	DeactivateRegistrationAuth(ctx context.Context, userID string) error
	DeleteByUserID(ctx context.Context, userID string) error
}
