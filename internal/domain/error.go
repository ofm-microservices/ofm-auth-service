package auth

import "errors"

var (
	ErrInvalidUserID                  = errors.New("invalid user id")
	ErrInvalidEmail                   = errors.New("invalid email")
	ErrInvalidUsername                = errors.New("invalid username")
	ErrInvalidPasswordHash            = errors.New("invalid password hash")
	ErrInvalidVerificationCode        = errors.New("invalid verification code")
	ErrExpiredVerificationCode        = errors.New("verification code expired")
	ErrEmailAlreadyTaken              = errors.New("email is already taken")
	ErrAuthCredentialsAlreadyExist    = errors.New("auth credentials already exist")
	ErrAuthNotFound                   = errors.New("auth credentials not found")
	ErrEmailNotVerified               = errors.New("email is not verified")
	ErrInvalidCredentials             = errors.New("invalid credentials")
	ErrInvalidRefreshToken            = errors.New("invalid refresh token")
	ErrRefreshTokenExpired            = errors.New("refresh token expired")
	ErrRefreshTokenRevoked            = errors.New("refresh token revoked")
	ErrFailedToCreateCredential       = errors.New("failed to create auth credential")
	ErrFailedToCreateVerificationCode = errors.New("failed to create verification code")
	ErrFailedToVerifyEmail            = errors.New("failed to verify email")
	ErrFailedToCreateRefreshToken     = errors.New("failed to create refresh token")
	ErrFailedToRotateRefreshToken     = errors.New("failed to rotate refresh token")
	ErrFailedToFindCredential         = errors.New("failed to find auth credential")
	ErrFailedToDeactivateCredential   = errors.New("failed to deactivate auth credential")
	ErrFailedToDeleteCredential       = errors.New("failed to delete auth credential")
)
