package repository

// DBErrorTranslator maps storage-driver failures into domain-aware repository
// errors.
type DBErrorTranslator interface {
	TranslateCreateCredentialError(err error) error
	TranslateCreateVerificationCodeError(err error) error
	TranslateFindCredentialError(err error) error
	TranslateDeleteCredentialError(err error) error
}
