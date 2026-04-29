package repository

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("repository error wrappers", func() {
	It("preserves wrapped causes", func() {
		cause := errors.New("boom")

		Expect(WrapCreateCredentialError(cause)).To(MatchError(ContainSubstring("failed to create auth credential")))
		Expect(WrapCreateVerificationCodeError(cause)).To(MatchError(ContainSubstring("failed to create verification code")))
		Expect(WrapFindCredentialError(cause)).To(MatchError(ContainSubstring("failed to find auth credential")))
		Expect(WrapDeleteCredentialError(cause)).To(MatchError(ContainSubstring("failed to delete auth credential")))
	})
})
