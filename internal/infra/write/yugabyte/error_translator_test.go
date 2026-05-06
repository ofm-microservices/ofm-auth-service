package repository

import (
	"database/sql"
	"errors"

	auth "auth-service/internal/domain"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PgErrorTranslator", func() {
	var translator DBErrorTranslator

	BeforeEach(func() {
		translator = NewPgErrorTranslator()
	})

	It("maps primary-key conflicts to auth already exists", func() {
		err := translator.TranslateCreateCredentialError(&pgconn.PgError{
			Code:           pgerrcode.UniqueViolation,
			ConstraintName: authCredentialsPrimaryKeyConstraint,
		})

		Expect(err).To(MatchError(auth.ErrAuthCredentialsAlreadyExist))
	})

	It("maps email conflicts and fallback error text", func() {
		err := translator.TranslateCreateCredentialError(&pgconn.PgError{
			Code:           pgerrcode.UniqueViolation,
			ConstraintName: authCredentialsEmailConstraint,
		})
		Expect(err).To(MatchError(auth.ErrEmailAlreadyTaken))

		err = translator.TranslateCreateCredentialError(&pgconn.PgError{
			Code:    pgerrcode.UniqueViolation,
			Message: `duplicate key value violates unique constraint "auth_credentials_email_key"`,
		})
		Expect(err).To(MatchError(auth.ErrEmailAlreadyTaken))
	})

	It("maps invalid user ids and foreign key violations", func() {
		err := translator.TranslateCreateCredentialError(&pgconn.PgError{
			Code: pgerrcode.InvalidTextRepresentation,
		})
		Expect(err).To(MatchError(auth.ErrInvalidUserID))

		err = translator.TranslateCreateCredentialError(&pgconn.PgError{
			Code:           pgerrcode.UniqueViolation,
			Message:        `duplicate key value violates unique constraint "auth_credentials_pkey"`,
		})
		Expect(err).To(MatchError(auth.ErrAuthCredentialsAlreadyExist))

		err = translator.TranslateVerifyEmailError(sql.ErrNoRows)
		Expect(err).To(MatchError(auth.ErrInvalidVerificationCode))

		err = translator.TranslateVerifyEmailError(&pgconn.PgError{
			Code: pgerrcode.InvalidTextRepresentation,
		})
		Expect(err).To(MatchError(auth.ErrInvalidUserID))

		err = translator.TranslateCreateVerificationCodeError(&pgconn.PgError{
			Code: pgerrcode.InvalidTextRepresentation,
		})
		Expect(err).To(MatchError(auth.ErrInvalidUserID))

		err = translator.TranslateCreateVerificationCodeError(&pgconn.PgError{
			Code: pgerrcode.ForeignKeyViolation,
		})
		Expect(err).To(MatchError(auth.ErrAuthNotFound))
	})

	It("maps generic lookup and delete failures", func() {
		Expect(translator.TranslateFindCredentialError(sql.ErrNoRows)).To(MatchError(auth.ErrAuthNotFound))
		Expect(translator.TranslateFindCredentialError(errors.New("boom"))).To(MatchError(ContainSubstring("failed to find auth credential")))
		Expect(translator.TranslateDeleteCredentialError(errors.New("boom"))).To(MatchError(ContainSubstring("failed to delete auth credential")))
	})

	It("wraps generic credential and verification failures", func() {
		Expect(translator.TranslateCreateCredentialError(errors.New("boom"))).To(MatchError(ContainSubstring("failed to create auth credential")))
		Expect(translator.TranslateCreateVerificationCodeError(errors.New("boom"))).To(MatchError(ContainSubstring("failed to create verification code")))
		Expect(translator.TranslateCreateRefreshTokenError(&pgconn.PgError{Code: pgerrcode.InvalidTextRepresentation})).To(MatchError(auth.ErrInvalidUserID))
		Expect(translator.TranslateCreateRefreshTokenError(&pgconn.PgError{Code: pgerrcode.ForeignKeyViolation})).To(MatchError(auth.ErrAuthNotFound))
		Expect(translator.TranslateCreateRefreshTokenError(errors.New("boom"))).To(MatchError(ContainSubstring("failed to create refresh token")))
	})
})
