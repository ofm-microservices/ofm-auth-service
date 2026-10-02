package mapper

import (
	auth "auth-service/internal/domain"
	"auth-service/internal/infra/write/postgres/model"
)

// MapCredentialRowToDomain maps the PostgreSQL row to the auth domain entity.
func MapCredentialRowToDomain(row model.CredentialRow) *auth.Credential {
	return &auth.Credential{
		UserID:        row.UserID,
		Email:         row.Email,
		Username:      row.Username,
		PasswordHash:  row.PasswordHash,
		EmailVerified: row.EmailVerified,
		Status:        row.Status,
		Roles:         row.Roles,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}
