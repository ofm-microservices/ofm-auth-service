package repository

import (
	auth "auth-service/internal/domain"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	authCredentialsPrimaryKeyConstraint = "auth_credentials_pkey"
	authCredentialsEmailConstraint      = "auth_credentials_email_key"
)

// PgErrorTranslator converts pgx/Yugabyte errors into domain-aware repository
// errors.
type PgErrorTranslator struct{}

// NewPgErrorTranslator constructs the default Yugabyte error translator.
func NewPgErrorTranslator() DBErrorTranslator {
	return &PgErrorTranslator{}
}

func (t *PgErrorTranslator) TranslateCreateCredentialError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			switch pgErr.ConstraintName {
			case authCredentialsPrimaryKeyConstraint:
				return auth.ErrAuthCredentialsAlreadyExist
			case authCredentialsEmailConstraint:
				return auth.ErrEmailAlreadyTaken
			default:
				switch {
				case strings.Contains(err.Error(), authCredentialsPrimaryKeyConstraint):
					return auth.ErrAuthCredentialsAlreadyExist
				case strings.Contains(err.Error(), authCredentialsEmailConstraint):
					return auth.ErrEmailAlreadyTaken
				}
			}
		case pgerrcode.InvalidTextRepresentation:
			return auth.ErrInvalidUserID
		}
	}

	return auth.ErrFailedToCreateCredential
}

func (t *PgErrorTranslator) TranslateCreateVerificationCodeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.InvalidTextRepresentation:
			return auth.ErrInvalidUserID
		case pgerrcode.ForeignKeyViolation:
			return auth.ErrAuthNotFound
		}
	}

	return auth.ErrFailedToCreateVerificationCode
}

func (t *PgErrorTranslator) TranslateVerifyEmailError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrInvalidVerificationCode
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.InvalidTextRepresentation {
		return auth.ErrInvalidUserID
	}

	return auth.ErrFailedToVerifyEmail
}

func (t *PgErrorTranslator) TranslateCreateRefreshTokenError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.InvalidTextRepresentation:
			return auth.ErrInvalidUserID
		case pgerrcode.ForeignKeyViolation:
			return auth.ErrAuthNotFound
		}
	}

	return auth.ErrFailedToCreateRefreshToken
}

func (t *PgErrorTranslator) TranslateFindCredentialError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrAuthNotFound
	}

	return auth.ErrFailedToFindCredential
}

func (t *PgErrorTranslator) TranslateDeactivateCredentialError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.InvalidTextRepresentation {
		return auth.ErrInvalidUserID
	}

	return auth.ErrFailedToDeactivateCredential
}

func (t *PgErrorTranslator) TranslateDeleteCredentialError(err error) error {
	return auth.ErrFailedToDeleteCredential
}
