package repository

import (
	auth "auth-service/internal/domain"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("repository errors", func() {
	It("uses reusable domain sentinels", func() {
		Expect(auth.ErrFailedToCreateCredential).To(MatchError("failed to create auth credential"))
		Expect(auth.ErrFailedToCreateVerificationCode).To(MatchError("failed to create verification code"))
		Expect(auth.ErrFailedToFindCredential).To(MatchError("failed to find auth credential"))
		Expect(auth.ErrFailedToDeleteCredential).To(MatchError("failed to delete auth credential"))
	})
})
