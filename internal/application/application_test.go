package service

import (
	"context"
	"testing"
	"time"

	"auth-service/config"
	auth "auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestApplication(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Application Suite")
}

var _ = Describe("AuthService", func() {
	var (
		ctrl   *gomock.Controller
		repo   *MockAuthRepository
		logger logging.Logger
	)

	testJWTConfig := func() config.JWTConfig {
		return config.JWTConfig{
			AccessSecret:      "access-secret",
			RefreshSecret:     "refresh-secret",
			AccessTokenTTL:    time.Minute,
			RefreshTokenTTL:   2 * time.Minute,
			RefreshTokenBytes: 16,
		}
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		repo = NewMockAuthRepository(ctrl)

		var err error
		logger, err = logging.New("auth-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("New", func() {
		It("validates nil collaborators", func() {
			svc, err := New(nil, testJWTConfig(), logger)
			Expect(svc).To(BeNil())
			Expect(err).To(MatchError(ErrNilAuthRepository))

			svc, err = New(repo, testJWTConfig(), nil)
			Expect(svc).To(BeNil())
			Expect(err).To(MatchError(ErrNilLogger))
		})
	})

	Describe("CreateCredential", func() {
		It("validates input", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			result, err := svc.CreateCredential(context.Background(), "", "user@example.com", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidUserID))

			result, err = svc.CreateCredential(context.Background(), "user-1", "   ", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidEmail))

			result, err = svc.CreateCredential(context.Background(), "user-1", "user@example.com", "alex", "  ")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidPasswordHash))
		})

		It("creates credentials through the repository", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().
				Create(gomock.Any(), auth.CreateCredentialParams{
					UserID:       "user-1",
					Email:        "user@example.com",
					Username:     "alex",
					PasswordHash: "hash",
				}).
				Return(&auth.Credential{
					UserID:       "user-1",
					Email:        "user@example.com",
					Username:     "alex",
					PasswordHash: "hash",
				}, nil)

			result, err := svc.CreateCredential(context.Background(), "user-1", "user@example.com", "alex", "hash")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.UserID).To(Equal("user-1"))
		})

		It("returns repository failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				Return(nil, auth.ErrEmailAlreadyTaken)

			result, err := svc.CreateCredential(context.Background(), "user-1", "user@example.com", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrEmailAlreadyTaken))
		})
	})

	Describe("DeleteCredential", func() {
		It("validates user id", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			err = svc.DeleteCredential(context.Background(), "")
			Expect(err).To(MatchError(auth.ErrInvalidUserID))
		})

		It("deletes credentials through the repository", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().DeleteByUserID(gomock.Any(), "user-1").Return(nil)

			Expect(svc.DeleteCredential(context.Background(), "user-1")).To(Succeed())
		})

		It("returns repository failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().DeleteByUserID(gomock.Any(), "user-1").Return(auth.ErrAuthNotFound)

			err = svc.DeleteCredential(context.Background(), "user-1")
			Expect(err).To(MatchError(auth.ErrAuthNotFound))
		})
	})

	Describe("ExistsByEmail", func() {
		It("validates email", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			exists, err := svc.ExistsByEmail(context.Background(), "   ")
			Expect(exists).To(BeFalse())
			Expect(err).To(MatchError(auth.ErrInvalidEmail))
		})

		It("trims email before delegating", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().ExistsByEmail(gomock.Any(), "user@example.com").Return(true, nil)

			exists, err := svc.ExistsByEmail(context.Background(), " user@example.com ")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns repository failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().ExistsByEmail(gomock.Any(), "user@example.com").Return(false, auth.ErrFailedToFindCredential)

			exists, err := svc.ExistsByEmail(context.Background(), "user@example.com")
			Expect(exists).To(BeFalse())
			Expect(err).To(MatchError(auth.ErrFailedToFindCredential))
		})
	})

	Describe("CreatePendingRegistration", func() {
		It("returns credential validation failures from the first step", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			result, err := svc.CreatePendingRegistration(context.Background(), "", "user@example.com", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidUserID))
		})

		It("creates the credential and verification code", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().
				Create(gomock.Any(), auth.CreateCredentialParams{
					UserID:       "user-1",
					Email:        "user@example.com",
					Username:     "alex",
					PasswordHash: "hash",
				}).
				Return(&auth.Credential{
					UserID:       "user-1",
					Email:        "user@example.com",
					Username:     "alex",
					PasswordHash: "hash",
				}, nil)

			repo.EXPECT().
				CreateVerificationCode(gomock.Any(), gomock.AssignableToTypeOf(auth.CreateVerificationCodeParams{})).
				DoAndReturn(func(_ context.Context, params auth.CreateVerificationCodeParams) error {
					Expect(params.ID).NotTo(BeEmpty())
					parsed, err := uuid.Parse(params.ID)
					Expect(err).NotTo(HaveOccurred())
					Expect(parsed.Version()).To(Equal(uuid.Version(7)))
					Expect(params.UserID).To(Equal("user-1"))
					Expect(params.TokenHash).To(HaveLen(64))
					Expect(params.ExpiresAt.IsZero()).To(BeFalse())
					return nil
				})

			result, err := svc.CreatePendingRegistration(context.Background(), "user-1", "user@example.com", "alex", "hash")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.UserID).To(Equal("user-1"))
			Expect(result.Email).To(Equal("user@example.com"))
			Expect(result.VerificationCode).To(MatchRegexp(`^\d{6}$`))
			Expect(result.ExpiresIn).To(Equal("10 minutes"))
		})

		It("returns verification code persistence failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&auth.Credential{
				UserID:       "user-1",
				Email:        "user@example.com",
				PasswordHash: "hash",
			}, nil)
			repo.EXPECT().CreateVerificationCode(gomock.Any(), gomock.Any()).Return(auth.ErrFailedToCreateVerificationCode)

			result, err := svc.CreatePendingRegistration(context.Background(), "user-1", "user@example.com", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrFailedToCreateVerificationCode))
		})

		It("returns credential creation failures before verification generation", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, auth.ErrEmailAlreadyTaken)

			result, err := svc.CreatePendingRegistration(context.Background(), "user-1", "user@example.com", "alex", "hash")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrEmailAlreadyTaken))
		})
	})

	Describe("VerifyRegistrationEmail", func() {
		It("validates inputs", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			result, err := svc.VerifyRegistrationEmail(context.Background(), " ", "123456")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidUserID))

			result, err = svc.VerifyRegistrationEmail(context.Background(), "user-1", " ")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidVerificationCode))
		})

		It("verifies email through the repository", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().
				VerifyRegistrationEmail(gomock.Any(), "user-1", gomock.Len(64), gomock.Any()).
				Return(&auth.Credential{
					UserID:        "user-1",
					Email:         "user@example.com",
					EmailVerified: true,
					Status:        auth.CredentialStatusEmailVerified,
				}, nil)

			result, err := svc.VerifyRegistrationEmail(context.Background(), "user-1", "123456")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&auth.RegistrationEmailVerificationResult{
				UserID: "user-1",
				Email:  "user@example.com",
				Status: "verified",
			}))
		})

		It("returns repository failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().
				VerifyRegistrationEmail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, auth.ErrInvalidVerificationCode)

			result, err := svc.VerifyRegistrationEmail(context.Background(), "user-1", "123456")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(auth.ErrInvalidVerificationCode))
		})
	})

	Describe("DeactivateRegistrationAuth", func() {
		It("validates the user id", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			err = svc.DeactivateRegistrationAuth(context.Background(), " ")
			Expect(err).To(MatchError(auth.ErrInvalidUserID))
		})

		It("returns repository failures", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().DeactivateRegistrationAuth(gomock.Any(), "user-1").Return(auth.ErrFailedToDeactivateCredential)
			Expect(svc.DeactivateRegistrationAuth(context.Background(), "user-1")).To(MatchError(auth.ErrFailedToDeactivateCredential))
		})

		It("deactivates registration auth through the repository", func() {
			svc, err := New(repo, testJWTConfig(), logger)
			Expect(err).NotTo(HaveOccurred())

			repo.EXPECT().DeactivateRegistrationAuth(gomock.Any(), "user-1").Return(nil)
			Expect(svc.DeactivateRegistrationAuth(context.Background(), "user-1")).To(Succeed())
		})
	})
})
