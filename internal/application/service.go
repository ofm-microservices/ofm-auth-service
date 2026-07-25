package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"auth-service/config"
	auth "auth-service/internal/domain"
	"github.com/google/uuid"
	commonjwt "github.com/ofm-microservices/ofm-common/pkg/jwt"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"golang.org/x/crypto/bcrypt"
)

const invalidCreateCredentialCommandMessage = "invalid create auth credential command"

type authService struct {
	repo   AuthRepository
	cfg    config.JWTConfig
	log    Logger
	signer commonjwt.Signer
}

// New constructs the auth application service.
func New(repo AuthRepository, cfg config.JWTConfig, log Logger) (AuthService, error) {
	if repo == nil {
		return nil, ErrNilAuthRepository
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	signer, err := commonjwt.NewSigner(commonjwt.Config{Secret: cfg.AccessSecret})
	if err != nil {
		return nil, err
	}

	return &authService{
		repo:   repo,
		cfg:    cfg,
		log:    log.With(logging.String("module", "application")),
		signer: signer,
	}, nil
}

// CreateCredential validates and persists the auth credential.
func (s *authService) CreateCredential(ctx context.Context, userID, email, username, passwordHash string) (*auth.Credential, error) {
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
	if strings.TrimSpace(username) == "" {
		s.log.Error(invalidCreateCredentialCommandMessage, logging.String("reason", "empty username"))
		return nil, auth.ErrInvalidUsername
	}
	if strings.TrimSpace(passwordHash) == "" {
		s.log.Error(invalidCreateCredentialCommandMessage, logging.String("reason", "empty password_hash"))
		return nil, auth.ErrInvalidPasswordHash
	}

	credential, err := s.repo.Create(ctx, auth.CreateCredentialParams{
		UserID:       userID,
		Email:        email,
		Username:     username,
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

// GetEmailByUserID returns the stored email for one user.
func (s *authService) GetEmailByUserID(ctx context.Context, userID string) (string, error) {
	credential, err := s.repo.GetByUserID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return "", err
	}
	return credential.Email, nil
}

// CreatePendingRegistration creates the credential and issues a verification
// code owned by auth-service.
func (s *authService) CreatePendingRegistration(ctx context.Context, userID, email, username, passwordHash string) (*auth.PendingRegistrationResult, error) {
	credential, err := s.CreateCredential(ctx, userID, email, username, passwordHash)
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
		ID:        uuid.Must(uuid.NewV7()).String(),
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

// VerifyRegistrationEmail verifies the pending email code and marks the auth
// credential as email-verified.
func (s *authService) VerifyRegistrationEmail(ctx context.Context, userID, code string) (*auth.RegistrationEmailVerificationResult, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, auth.ErrInvalidUserID
	}
	if strings.TrimSpace(code) == "" {
		return nil, auth.ErrInvalidVerificationCode
	}

	credential, err := s.repo.VerifyRegistrationEmail(ctx, strings.TrimSpace(userID), hashVerificationCode(strings.TrimSpace(code)), time.Now().UTC())
	if err != nil {
		s.log.Error("verify registration email failed", logging.String("user_id", userID), logging.Err(err))
		return nil, err
	}

	return &auth.RegistrationEmailVerificationResult{
		UserID: credential.UserID,
		Email:  credential.Email,
		Status: "verified",
	}, nil
}

// IssueRegistrationTokens creates an access token and stores a refresh token
// for an email-verified credential.
func (s *authService) IssueRegistrationTokens(ctx context.Context, userID string) (*auth.TokenPair, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, auth.ErrInvalidUserID
	}

	credential, err := s.repo.GetByUserID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if !credential.EmailVerified {
		return nil, auth.ErrEmailNotVerified
	}
	if credential.Status != auth.CredentialStatusEmailVerified {
		return nil, auth.ErrEmailNotVerified
	}

	now := time.Now().UTC()
	accessToken, err := s.signAccessToken(credential, now)
	if err != nil {
		return nil, err
	}
	refreshToken, err := generateRandomToken(s.cfg.RefreshTokenBytes)
	if err != nil {
		return nil, auth.ErrFailedToCreateRefreshToken
	}

	if err := s.repo.CreateRefreshToken(ctx, auth.CreateRefreshTokenParams{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    credential.UserID,
		TokenHash: hashVerificationCode(refreshToken),
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return nil, err
	}

	return &auth.TokenPair{
		UserID:       credential.UserID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// SignIn validates a stored credential against a username-or-email identifier
// and returns auth-owned tokens.
func (s *authService) SignIn(ctx context.Context, identifier, password string) (*auth.TokenPair, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || strings.TrimSpace(password) == "" {
		return nil, auth.ErrInvalidCredentials
	}

	credential, err := s.repo.GetByIdentifier(ctx, identifier)
	if err != nil {
		return nil, auth.ErrInvalidCredentials
	}
	if !credential.EmailVerified || credential.Status != auth.CredentialStatusEmailVerified {
		return nil, auth.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)); err != nil {
		return nil, auth.ErrInvalidCredentials
	}

	now := time.Now().UTC()
	accessToken, err := s.signAccessToken(credential, now)
	if err != nil {
		return nil, err
	}
	refreshToken, err := generateRandomToken(s.cfg.RefreshTokenBytes)
	if err != nil {
		return nil, auth.ErrFailedToCreateRefreshToken
	}

	if err := s.repo.CreateRefreshToken(ctx, auth.CreateRefreshTokenParams{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    credential.UserID,
		TokenHash: hashVerificationCode(refreshToken),
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return nil, err
	}

	return &auth.TokenPair{
		UserID:       credential.UserID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// Refresh rotates a refresh token and returns a new token pair for the owning
// credential.
func (s *authService) Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, auth.ErrInvalidRefreshToken
	}

	now := time.Now().UTC()
	newRefreshToken, err := generateRandomToken(s.cfg.RefreshTokenBytes)
	if err != nil {
		return nil, auth.ErrFailedToCreateRefreshToken
	}
	credential, err := s.repo.RotateRefreshToken(ctx, auth.RotateRefreshTokenParams{
		CurrentTokenHash: hashVerificationCode(refreshToken),
		NewTokenID:       uuid.Must(uuid.NewV7()).String(),
		NewTokenHash:     hashVerificationCode(newRefreshToken),
		NewExpiresAt:     now.Add(s.cfg.RefreshTokenTTL),
		Now:              now,
	})
	if err != nil {
		return nil, err
	}

	accessToken, err := s.signAccessToken(credential, now)
	if err != nil {
		return nil, err
	}

	return &auth.TokenPair{
		UserID:       credential.UserID,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// SignOut revokes a refresh token without issuing replacement credentials.
func (s *authService) SignOut(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return auth.ErrInvalidRefreshToken
	}

	if _, err := s.repo.RevokeRefreshToken(ctx, auth.RevokeRefreshTokenParams{
		CurrentTokenHash: hashVerificationCode(refreshToken),
		Now:              time.Now().UTC(),
	}); err != nil {
		return err
	}

	return nil
}

// DeactivateRegistrationAuth marks auth registration data inactive for
// compensation.
func (s *authService) DeactivateRegistrationAuth(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return auth.ErrInvalidUserID
	}

	return s.repo.DeactivateRegistrationAuth(ctx, strings.TrimSpace(userID))
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

func generateRandomToken(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 32
	}
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *authService) signAccessToken(credential *auth.Credential, now time.Time) (string, error) {
	return s.signer.Sign(commonjwt.Claims{
		Subject:   credential.UserID,
		Email:     credential.Email,
		Username:  credential.Username,
		Roles:     credential.Roles,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.cfg.AccessTokenTTL).Unix(),
	})
}
