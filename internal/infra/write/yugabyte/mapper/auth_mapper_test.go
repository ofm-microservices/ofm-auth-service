package mapper

import (
	auth "auth-service/internal/domain"
	"auth-service/internal/infra/write/yugabyte/model"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMapper(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Yugabyte Mapper Suite")
}

var _ = Describe("MapCredentialRowToDomain", func() {
	It("maps the storage row into the domain entity", func() {
		now := time.Now().UTC()
		credential := MapCredentialRowToDomain(model.CredentialRow{
			UserID:        "11111111-1111-1111-1111-111111111111",
			Email:         "user@example.com",
			PasswordHash:  "hash",
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		})

		Expect(credential).To(Equal(&auth.Credential{
			UserID:        "11111111-1111-1111-1111-111111111111",
			Email:         "user@example.com",
			PasswordHash:  "hash",
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}))
	})
})
