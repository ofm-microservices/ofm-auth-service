package grpc

import "errors"

var (
	ErrNilAuthService = errors.New("auth service is nil")
	ErrNilLogger      = errors.New("logger is nil")
)
