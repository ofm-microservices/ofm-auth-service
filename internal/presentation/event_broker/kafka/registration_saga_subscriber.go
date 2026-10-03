package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"auth-service/config"
	application "auth-service/internal/application"
	eventbroker "auth-service/internal/presentation/event_broker"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
)

type createAuthCommand struct {
	SessionID    string `json:"session_id"`
	ClientID     string `json:"client_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	SagaID       string `json:"saga_id,omitempty"`
}
type deleteAuthCommand struct {
	UserID string `json:"user_id"`
	SagaID string `json:"saga_id,omitempty"`
}

// RegistrationSagaSubscriber consumes registration commands from Kafka.
type RegistrationSagaSubscriber interface{ Subscribe(context.Context) error }

type registrationSagaSubscriber struct {
	broker  eventbroker.EventBroker
	service application.AuthService
	cfg     config.KafkaConfig
	log     logging.Logger
}

// NewRegistrationSagaSubscriber constructs the Kafka registration command consumer.
func NewRegistrationSagaSubscriber(broker eventbroker.EventBroker, service application.AuthService, cfg config.KafkaConfig, log logging.Logger) (RegistrationSagaSubscriber, error) {
	if broker == nil {
		return nil, errors.New("event broker is nil")
	}
	if service == nil {
		return nil, errors.New("auth service is nil")
	}
	if log == nil {
		return nil, errors.New("logger is nil")
	}
	return &registrationSagaSubscriber{broker: broker, service: service, cfg: cfg, log: log.With(logging.String("module", "registration-saga-subscriber"))}, nil
}

func (s *registrationSagaSubscriber) Subscribe(ctx context.Context) error {
	if err := s.broker.RunPullConsumer(ctx, config.PullConsumerConfig{Subject: s.cfg.CreateTopic}, s.handleCreate); err != nil {
		return err
	}
	return s.broker.RunPullConsumer(ctx, config.PullConsumerConfig{Subject: s.cfg.DeleteTopic}, s.handleDelete)
}

func (s *registrationSagaSubscriber) handleCreate(ctx context.Context, _ string, payload []byte) error {
	s.log.Info("registration command handler entered", logging.Int("payload_bytes", len(payload)))
	var cmd createAuthCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("unmarshal create auth command: %w", err)
	}
	result, err := s.service.CreatePendingRegistration(ctx, cmd.UserID, cmd.Email, cmd.Username, cmd.PasswordHash)
	if err != nil {
		s.log.Error("registration command application failed", logging.String("user_id", cmd.UserID), logging.Err(err))
		return s.publish(ctx, s.cfg.CreateResultTopic, map[string]any{"session_id": cmd.SessionID, "client_id": cmd.ClientID, "user_id": cmd.UserID, "saga_id": cmd.SagaID, "status": "failed", "error": err.Error(), "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
	}
	if err := s.publish(ctx, s.cfg.CreateResultTopic, map[string]any{"session_id": cmd.SessionID, "client_id": cmd.ClientID, "user_id": cmd.UserID, "saga_id": cmd.SagaID, "email": result.Email, "verification_code": result.VerificationCode, "expires_in": result.ExpiresIn, "status": "success", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		s.log.Error("publish auth registration result failed", logging.String("topic", s.cfg.CreateResultTopic), logging.Err(err))
		return err
	}
	s.log.Info("registration result published", logging.String("topic", s.cfg.CreateResultTopic), logging.String("session_id", cmd.SessionID))
	if err := s.publish(ctx, s.cfg.MailTopic, map[string]any{"session_id": cmd.SessionID, "client_id": cmd.ClientID, "user_id": cmd.UserID, "request_id": cmd.SessionID, "correlation_id": cmd.SessionID, "message_type": "email_code", "to": result.Email, "data": map[string]any{"name": result.Email, "code": result.VerificationCode, "expires_in": result.ExpiresIn}}); err != nil {
		s.log.Error("publish registration mail command failed", logging.String("topic", s.cfg.MailTopic), logging.Err(err))
		return err
	}
	return nil
}

func (s *registrationSagaSubscriber) handleDelete(ctx context.Context, _ string, payload []byte) error {
	var cmd deleteAuthCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("unmarshal delete auth command: %w", err)
	}
	if err := s.service.DeleteCredential(ctx, cmd.UserID); err != nil {
		return s.publish(ctx, s.cfg.DeleteResultTopic, map[string]any{"user_id": cmd.UserID, "saga_id": cmd.SagaID, "status": "failed", "error": err.Error(), "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
	}
	return s.publish(ctx, s.cfg.DeleteResultTopic, map[string]any{"user_id": cmd.UserID, "saga_id": cmd.SagaID, "status": "success", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *registrationSagaSubscriber) publish(ctx context.Context, topic string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, topic, payload)
}
