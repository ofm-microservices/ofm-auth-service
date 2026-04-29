package auth

import "errors"

var (
	ErrInvalidUserID                  = errors.New("invalid user id")
	ErrInvalidEmail                   = errors.New("invalid email")
	ErrInvalidPasswordHash            = errors.New("invalid password hash")
	ErrInvalidVerificationCode        = errors.New("invalid verification code")
	ErrEmailAlreadyTaken              = errors.New("email is already taken")
	ErrAuthCredentialsAlreadyExist    = errors.New("auth credentials already exist")
	ErrAuthNotFound                   = errors.New("auth credentials not found")
	ErrFailedToCreateCredential       = errors.New("failed to create auth credential")
	ErrFailedToCreateVerificationCode = errors.New("failed to create verification code")
	ErrFailedToFindCredential         = errors.New("failed to find auth credential")
	ErrFailedToDeleteCredential       = errors.New("failed to delete auth credential")
)
