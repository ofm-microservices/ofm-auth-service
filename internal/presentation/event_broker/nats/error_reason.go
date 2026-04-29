package nats

import (
	auth "auth-service/internal/domain"
	"errors"
)

// FailureReasonResolver maps domain errors to stable saga-facing failure
// reasons.
type FailureReasonResolver interface {
	CreateAuthFailureReason(err error) string
	DeleteAuthFailureReason(err error) string
}

// DomainFailureReasonResolver is the default domain-error-to-string mapper for
// auth saga results.
type DomainFailureReasonResolver struct{}

// NewDomainFailureReasonResolver constructs the default failure reason
// resolver.
func NewDomainFailureReasonResolver() FailureReasonResolver {
	return &DomainFailureReasonResolver{}
}

func (r *DomainFailureReasonResolver) CreateAuthFailureReason(err error) string {
	switch {
	case errors.Is(err, auth.ErrInvalidUserID):
		return "invalid user_id"
	case errors.Is(err, auth.ErrInvalidEmail):
		return "invalid email"
	case errors.Is(err, auth.ErrInvalidPasswordHash):
		return "invalid password_hash"
	case errors.Is(err, auth.ErrFailedToCreateVerificationCode):
		return "failed to create verification code"
	case errors.Is(err, auth.ErrEmailAlreadyTaken):
		return "email is already taken"
	case errors.Is(err, auth.ErrAuthCredentialsAlreadyExist):
		return "auth credentials already exist"
	default:
		return "failed to create auth credential"
	}
}

func (r *DomainFailureReasonResolver) DeleteAuthFailureReason(err error) string {
	switch {
	case errors.Is(err, auth.ErrInvalidUserID):
		return "invalid user_id"
	case errors.Is(err, auth.ErrAuthNotFound):
		return "auth credentials not found"
	default:
		return "failed to delete auth credential"
	}
}
