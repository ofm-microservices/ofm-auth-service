package model

import (
	"database/sql"
	"time"
)

// RefreshTokenRotationRow is the storage projection used to validate and
// rotate one refresh token atomically.
type RefreshTokenRotationRow struct {
	RefreshTokenID        string       `db:"refresh_token_id"`
	RefreshTokenUserID    string       `db:"refresh_user_id"`
	RefreshTokenHash      string       `db:"refresh_token_hash"`
	RefreshTokenExpiry    time.Time    `db:"refresh_token_expires_at"`
	RefreshTokenRevokedAt sql.NullTime `db:"refresh_token_revoked_at"`
	CredentialUserID      string       `db:"credential_user_id"`
	Email                 string       `db:"email"`
	Username              string       `db:"username"`
	PasswordHash          string       `db:"password_hash"`
	EmailVerified         bool         `db:"email_verified"`
	Status                string       `db:"status"`
	CreatedAt             time.Time    `db:"created_at"`
	UpdatedAt             time.Time    `db:"updated_at"`
}
