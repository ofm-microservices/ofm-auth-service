package repository

import (
	domain "auth-service/internal/domain"
	"auth-service/internal/infra/write/yugabyte/mapper"
	"auth-service/internal/infra/write/yugabyte/model"
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
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
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "create", "auth_credentials", status, time.Since(started))
	}()

	var row model.CredentialRow
	if err := r.db.QueryRowContext(
		ctx,
		createCredentialQuery,
		params.UserID,
		params.Email,
		params.Username,
		params.PasswordHash,
	).Scan(
		&row.UserID,
		&row.Email,
		&row.Username,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		status = "error"
		return nil, r.translator.TranslateCreateCredentialError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) CreateVerificationCode(ctx context.Context, params domain.CreateVerificationCodeParams) error {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "create", "email_verification_codes", status, time.Since(started))
	}()

	if _, err := r.db.ExecContext(
		ctx,
		createVerificationCodeQuery,
		params.ID,
		params.UserID,
		params.TokenHash,
		params.ExpiresAt,
	); err != nil {
		status = "error"
		return r.translator.TranslateCreateVerificationCodeError(err)
	}

	return nil
}

func (r *repo) VerifyRegistrationEmail(ctx context.Context, userID, tokenHash string, now time.Time) (*domain.Credential, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "verify_registration_email", "auth_credentials", status, time.Since(started))
	}()

	var row model.CredentialRow
	if err := r.db.QueryRowContext(ctx, verifyRegistrationEmailQuery, userID, tokenHash, now).Scan(
		&row.UserID,
		&row.Email,
		&row.Username,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		status = "error"
		return nil, r.translator.TranslateVerifyEmailError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) CreateRefreshToken(ctx context.Context, params domain.CreateRefreshTokenParams) error {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "create", "refresh_tokens", status, time.Since(started))
	}()

	if _, err := r.db.ExecContext(
		ctx,
		createRefreshTokenQuery,
		params.ID,
		params.UserID,
		params.TokenHash,
		params.ExpiresAt,
	); err != nil {
		status = "error"
		return r.translator.TranslateCreateRefreshTokenError(err)
	}

	return nil
}

func (r *repo) RotateRefreshToken(ctx context.Context, params domain.RotateRefreshTokenParams) (*domain.Credential, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "rotate", "refresh_tokens", status, time.Since(started))
	}()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	row, err := r.loadRefreshTokenForMutation(ctx, tx, refreshTokenForRotationQuery, params.CurrentTokenHash, params.Now)
	if err != nil {
		status = "error"
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, revokeRefreshTokenQuery, row.RefreshTokenID, params.Now); err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}

	if _, err := tx.ExecContext(ctx, createRefreshTokenQuery, params.NewTokenID, row.Credential.UserID, params.NewTokenHash, params.NewExpiresAt); err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}

	if err := tx.Commit(); err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}
	tx = nil

	return row.Credential, nil
}

func (r *repo) RevokeRefreshToken(ctx context.Context, params domain.RevokeRefreshTokenParams) (*domain.Credential, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "revoke", "refresh_tokens", status, time.Since(started))
	}()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	row, err := r.loadRefreshTokenForMutation(ctx, tx, refreshTokenForRevokeQuery, params.CurrentTokenHash, params.Now)
	if err != nil {
		status = "error"
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, revokeRefreshTokenQuery, row.RefreshTokenID, params.Now); err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}

	if err := tx.Commit(); err != nil {
		status = "error"
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}
	tx = nil

	return row.Credential, nil
}

type refreshTokenMutationRow struct {
	RefreshTokenID string
	Credential     *domain.Credential
}

func (r *repo) loadRefreshTokenForMutation(ctx context.Context, tx *sqlx.Tx, query, currentTokenHash string, now time.Time) (*refreshTokenMutationRow, error) {
	var row model.RefreshTokenRotationRow
	if err := tx.QueryRowContext(ctx, query, currentTokenHash).Scan(
		&row.RefreshTokenID,
		&row.RefreshTokenUserID,
		&row.RefreshTokenHash,
		&row.RefreshTokenExpiry,
		&row.RefreshTokenRevokedAt,
		&row.CredentialUserID,
		&row.Email,
		&row.Username,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, r.translator.TranslateRotateRefreshTokenError(err)
	}
	if row.RefreshTokenRevokedAt.Valid {
		return nil, domain.ErrRefreshTokenRevoked
	}
	if !row.RefreshTokenExpiry.After(now) {
		return nil, domain.ErrRefreshTokenExpired
	}

	credential := mapper.MapCredentialRowToDomain(model.CredentialRow{
		UserID:        row.CredentialUserID,
		Email:         row.Email,
		Username:      row.Username,
		PasswordHash:  row.PasswordHash,
		EmailVerified: row.EmailVerified,
		Status:        row.Status,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	})

	return &refreshTokenMutationRow{
		RefreshTokenID: row.RefreshTokenID,
		Credential:     credential,
	}, nil
}

func (r *repo) GetByUserID(ctx context.Context, userID string) (*domain.Credential, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "get_by_user_id", "auth_credentials", status, time.Since(started))
	}()

	var row model.CredentialRow
	if err := r.db.QueryRowContext(ctx, getCredentialByUserIDQuery, userID).Scan(
		&row.UserID,
		&row.Email,
		&row.Username,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		status = "error"
		return nil, r.translator.TranslateFindCredentialError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) GetByIdentifier(ctx context.Context, identifier string) (*domain.Credential, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "get_by_identifier", "auth_credentials", status, time.Since(started))
	}()

	var row model.CredentialRow
	if err := r.db.QueryRowContext(ctx, getCredentialByIdentifierQuery, identifier).Scan(
		&row.UserID,
		&row.Email,
		&row.Username,
		&row.PasswordHash,
		&row.EmailVerified,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		status = "error"
		return nil, r.translator.TranslateFindCredentialError(err)
	}

	return mapper.MapCredentialRowToDomain(row), nil
}

func (r *repo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "exists_by_email", "auth_credentials", status, time.Since(started))
	}()

	var exists bool
	if err := r.db.QueryRowContext(ctx, existsCredentialByEmailQuery, email).Scan(&exists); err != nil {
		status = "error"
		return false, domain.ErrFailedToFindCredential
	}

	return exists, nil
}

func (r *repo) DeactivateRegistrationAuth(ctx context.Context, userID string) error {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "deactivate_registration_auth", "auth_credentials", status, time.Since(started))
	}()

	result, err := r.db.ExecContext(ctx, deactivateRegistrationAuthQuery, userID)
	if err != nil {
		status = "error"
		return r.translator.TranslateDeactivateCredentialError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		status = "error"
		return r.translator.TranslateDeactivateCredentialError(err)
	}
	if rowsAffected == 0 {
		status = "error"
		return domain.ErrAuthNotFound
	}

	return nil
}

func (r *repo) DeleteByUserID(ctx context.Context, userID string) error {
	started := time.Now()
	status := "success"
	defer func() {
		metrics.Global().ObserveDB("yugabyte", "delete_by_user_id", "auth_credentials", status, time.Since(started))
	}()

	result, err := r.db.ExecContext(ctx, deleteCredentialByUserIDQuery, userID)
	if err != nil {
		status = "error"
		return r.translator.TranslateDeleteCredentialError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		status = "error"
		return r.translator.TranslateDeleteCredentialError(err)
	}
	if rowsAffected == 0 {
		status = "error"
		return domain.ErrAuthNotFound
	}

	return nil
}
