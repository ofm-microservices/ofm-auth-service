package repository

import "errors"

var (
	ErrNilYugaByteDB        = errors.New("yugabyte db is nil")
	ErrNilDBErrorTranslator = errors.New("db error translator is nil")
)
