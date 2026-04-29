package repository

import (
	auth "auth-service/internal/domain"
	"database/sql"
	"errors"
	"fmt"
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
				return WrapDomainError(auth.ErrAuthCredentialsAlreadyExist, err)
			case authCredentialsEmailConstraint:
				return WrapDomainError(auth.ErrEmailAlreadyTaken, err)
			default:
				switch {
				case strings.Contains(err.Error(), authCredentialsPrimaryKeyConstraint):
					return WrapDomainError(auth.ErrAuthCredentialsAlreadyExist, err)
				case strings.Contains(err.Error(), authCredentialsEmailConstraint):
					return WrapDomainError(auth.ErrEmailAlreadyTaken, err)
				}
			}
		case pgerrcode.InvalidTextRepresentation:
			return WrapDomainError(auth.ErrInvalidUserID, err)
		}
	}

	return WrapCreateCredentialError(err)
}

func (t *PgErrorTranslator) TranslateCreateVerificationCodeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.InvalidTextRepresentation:
			return WrapDomainError(auth.ErrInvalidUserID, err)
		case pgerrcode.ForeignKeyViolation:
			return WrapDomainError(auth.ErrAuthNotFound, err)
		}
	}

	return WrapCreateVerificationCodeError(err)
}

func (t *PgErrorTranslator) TranslateFindCredentialError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return WrapDomainError(auth.ErrAuthNotFound, err)
	}

	return WrapFindCredentialError(err)
}

func (t *PgErrorTranslator) TranslateDeleteCredentialError(err error) error {
	return WrapDeleteCredentialError(err)
}

// WrapDomainError preserves the domain error while attaching the original
// database cause for logging and debugging.
func WrapDomainError(domainErr, err error) error {
	return fmt.Errorf("%w: %v", domainErr, err)
}
