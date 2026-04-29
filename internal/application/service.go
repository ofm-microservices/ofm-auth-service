package service

import (
	auth "auth-service/internal/domain"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"strings"
	"time"

	"github.com/google/uuid"
)

const invalidCreateCredentialCommandMessage = "invalid create auth credential command"

type authService struct {
	repo AuthRepository
	log  Logger
}

// New constructs the auth application service.
func New(repo AuthRepository, log Logger) (AuthService, error) {
	if repo == nil {
		return nil, ErrNilAuthRepository
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &authService{
		repo: repo,
		log:  log.With(logging.String("module", "application")),
	}, nil
}

// CreateCredential validates and persists the auth credential.
func (s *authService) CreateCredential(ctx context.Context, userID, email, passwordHash string) (*auth.Credential, error) {
	s.log.Info("create auth credential command received",
		logging.String("user_id", userID),
		logging.String("email", email),
	)

	if userID == "" {
		s.log.Error(invalidCreateCredentialCommandMessage, logging.String("reason", "empty user_id"))
		return nil, auth.ErrInvalidUserID
	}
	if strings.TrimSpace(email) == "" {
		s.log.Error(invalidCreateCredentialCommandMessage, logging.String("reason", "empty email"))
		return nil, auth.ErrInvalidEmail
	}
	if strings.TrimSpace(passwordHash) == "" {
		s.log.Error(invalidCreateCredentialCommandMessage, logging.String("reason", "empty password_hash"))
		return nil, auth.ErrInvalidPasswordHash
	}

	credential, err := s.repo.Create(ctx, auth.CreateCredentialParams{
		UserID:       userID,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		s.log.Error("failed to create auth credential",
			logging.String("user_id", userID),
			logging.String("email", email),
			logging.Err(err),
		)
		return nil, err
	}

	s.log.Info("auth credential created",
		logging.String("user_id", credential.UserID),
		logging.String("email", credential.Email),
	)
	return credential, nil
}

// DeleteCredential removes a credential by user id.
func (s *authService) DeleteCredential(ctx context.Context, userID string) error {
	s.log.Info("delete auth credential command received", logging.String("user_id", userID))

	if userID == "" {
		s.log.Error("invalid delete auth credential command", logging.String("reason", "empty user_id"))
		return auth.ErrInvalidUserID
	}

	if err := s.repo.DeleteByUserID(ctx, userID); err != nil {
		s.log.Error("failed to delete auth credential", logging.String("user_id", userID), logging.Err(err))
		return err
	}

	s.log.Info("auth credential deleted", logging.String("user_id", userID))
	return nil
}

// ExistsByEmail reports whether auth-service already owns the supplied email.
func (s *authService) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if strings.TrimSpace(email) == "" {
		return false, auth.ErrInvalidEmail
	}

	return s.repo.ExistsByEmail(ctx, strings.TrimSpace(email))
}

// CreatePendingRegistration creates the credential and issues a verification
// code owned by auth-service.
func (s *authService) CreatePendingRegistration(ctx context.Context, userID, email, passwordHash string) (*auth.PendingRegistrationResult, error) {
	credential, err := s.CreateCredential(ctx, userID, email, passwordHash)
	if err != nil {
		return nil, err
	}

	code, err := generateVerificationCode()
	if err != nil {
		s.log.Error("generate verification code failed", logging.String("user_id", userID), logging.Err(err))
		return nil, auth.ErrFailedToCreateVerificationCode
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	if err := s.repo.CreateVerificationCode(ctx, auth.CreateVerificationCodeParams{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: hashVerificationCode(code),
		ExpiresAt: expiresAt,
	}); err != nil {
		s.log.Error("failed to create verification code", logging.String("user_id", userID), logging.Err(err))
		return nil, err
	}

	return &auth.PendingRegistrationResult{
		UserID:           credential.UserID,
		Email:            credential.Email,
		VerificationCode: code,
		ExpiresIn:        "10 minutes",
	}, nil
}

func generateVerificationCode() (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	value := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func hashVerificationCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
