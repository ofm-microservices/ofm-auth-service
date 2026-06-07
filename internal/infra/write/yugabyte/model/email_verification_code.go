package model

import "time"

// VerificationCodeRow is the Yugabyte persistence model for email verification
// codes.
type VerificationCodeRow struct {
	EmailVerificationCodeID string    `db:"email_verification_code_id"`
	UserID                  string    `db:"user_id"`
	TokenHash               string    `db:"token_hash"`
	ExpiresAt               time.Time `db:"expires_at"`
	UsedAt                  time.Time `db:"used_at"`
	CreatedAt               time.Time `db:"created_at"`
}
