package model

import "time"

// CredentialRow is the Yugabyte persistence model for auth credentials.
type CredentialRow struct {
	UserID        string    `db:"user_id"`
	Email         string    `db:"email"`
	Username      string    `db:"username"`
	PasswordHash  string    `db:"password_hash"`
	EmailVerified bool      `db:"email_verified"`
	Status        string    `db:"status"`
	Roles         []string  `db:"roles"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
