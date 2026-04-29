package service

import "errors"

var (
	ErrNilAuthRepository = errors.New("auth repository is nil")
	ErrNilLogger         = errors.New("logger is nil")
)
