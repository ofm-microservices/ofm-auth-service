package repository

import "errors"

var (
	ErrNilPostgresDB        = errors.New("postgres db is nil")
	ErrNilDBErrorTranslator = errors.New("db error translator is nil")
)
