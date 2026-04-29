package repository

import (
	domain "auth-service/internal/domain"
	"auth-service/internal/infra/write/yugabyte/mapper"
	"auth-service/internal/infra/write/yugabyte/model"
	"context"

	"github.com/jmoiron/sqlx"
)

type repo struct {
	db         *sqlx.DB
	translator DBErrorTranslator
}

// New constructs the Yugabyte-backed auth repository.
func New(db *sqlx.DB, translator DBErrorTranslator) (domain.AuthRepository, error) {
	if db == nil {
		return nil, ErrNilYugaByteDB
	}
	if translator == nil {
		return nil, ErrNilDBErrorTranslator
	}

	return &repo{db: db, translator: translator}, nil
}

func (r *repo) Create(ctx context.Context, params domain.CreateCredentialParams) (*domain.Credential, error) {
	var row model.CredentialRow
	if err := r.db.QueryRowContext(
		ctx,
		createCredentialQuery,
		params.UserID,
		params.Email,
		params.PasswordHash,
	).Scan(
		&row.UserID,
		&row.Email,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, r.translator.TranslateCreateCredentialError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) CreateVerificationCode(ctx context.Context, params domain.CreateVerificationCodeParams) error {
	if _, err := r.db.ExecContext(
		ctx,
		createVerificationCodeQuery,
		params.ID,
		params.UserID,
		params.TokenHash,
		params.ExpiresAt,
	); err != nil {
		return r.translator.TranslateCreateVerificationCodeError(err)
	}

	return nil
}

func (r *repo) GetByUserID(ctx context.Context, userID string) (*domain.Credential, error) {
	var row model.CredentialRow
	if err := r.db.QueryRowContext(ctx, getCredentialByUserIDQuery, userID).Scan(
		&row.UserID,
		&row.Email,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, r.translator.TranslateFindCredentialError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, existsCredentialByEmailQuery, email).Scan(&exists); err != nil {
		return false, WrapFindCredentialError(err)
	}

	return exists, nil
}

func (r *repo) DeleteByUserID(ctx context.Context, userID string) error {
	result, err := r.db.ExecContext(ctx, deleteCredentialByUserIDQuery, userID)
	if err != nil {
		return r.translator.TranslateDeleteCredentialError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return r.translator.TranslateDeleteCredentialError(err)
	}
	if rowsAffected == 0 {
		return domain.ErrAuthNotFound
	}

	return nil
}
