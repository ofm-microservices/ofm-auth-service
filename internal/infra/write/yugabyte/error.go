package repository

import (
	auth "auth-service/internal/domain"
	"errors"
	"fmt"
)

const repositoryWrapFormat = "%w: %v"

var (
	ErrNilYugaByteDB        = errors.New("yugabyte db is nil")
	ErrNilDBErrorTranslator = errors.New("db error translator is nil")
)

// WrapCreateCredentialError annotates credential insert failures.
func WrapCreateCredentialError(err error) error {
	return fmt.Errorf(repositoryWrapFormat, auth.ErrFailedToCreateCredential, err)
}

// WrapCreateVerificationCodeError annotates verification-code insert failures.
func WrapCreateVerificationCodeError(err error) error {
	return fmt.Errorf(repositoryWrapFormat, auth.ErrFailedToCreateVerificationCode, err)
}

// WrapFindCredentialError annotates credential lookup failures.
func WrapFindCredentialError(err error) error {
	return fmt.Errorf(repositoryWrapFormat, auth.ErrFailedToFindCredential, err)
}

// WrapDeleteCredentialError annotates credential delete failures.
func WrapDeleteCredentialError(err error) error {
	return fmt.Errorf(repositoryWrapFormat, auth.ErrFailedToDeleteCredential, err)
}
