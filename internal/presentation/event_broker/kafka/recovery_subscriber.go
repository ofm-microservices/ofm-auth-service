package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"auth-service/config"
	app "auth-service/internal/application"
	eventbroker "auth-service/internal/presentation/event_broker"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/migration/events"
	"github.com/ofm-microservices/ofm-common/pkg/resilience"
	"golang.org/x/crypto/bcrypt"
)

// AuthRecoverySubscriber applies auth-owned fallback commands through the auth application service.
type AuthRecoverySubscriber interface{ Subscribe(context.Context) error }

type authRecoverySubscriber struct {
	broker  eventbroker.EventBroker
	service app.AuthService
	cfg     config.KafkaConfig
	log     logging.Logger
}

// NewAuthRecoverySubscriber constructs the auth recovery Kafka adapter.
func NewAuthRecoverySubscriber(b eventbroker.EventBroker, svc app.AuthService, cfg config.KafkaConfig, log logging.Logger) (AuthRecoverySubscriber, error) {
	if b == nil || svc == nil || log == nil {
		return nil, errors.New("invalid auth recovery subscriber dependency")
	}
	return &authRecoverySubscriber{broker: b, service: svc, cfg: cfg, log: log.With(logging.String("module", "kafka-auth-recovery-subscriber"))}, nil
}

func (s *authRecoverySubscriber) Subscribe(ctx context.Context) error {
	return s.broker.RunPullConsumer(ctx, config.PullConsumerConfig{Subject: s.cfg.RecoveryTopic, GroupID: s.cfg.RecoveryGroup}, s.handle)
}

func (s *authRecoverySubscriber) handle(ctx context.Context, _ string, raw []byte) error {
	var cmd events.Envelope
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return fmt.Errorf("decode auth recovery command: %w", err)
	}
	if !strings.EqualFold(cmd.AggregateType, "auth") {
		return fmt.Errorf("unsupported auth recovery aggregate_type=%q", cmd.AggregateType)
	}
	var p struct {
		UserID       string `json:"user_id"`
		ClientID     string `json:"client_id"`
		Email        string `json:"email"`
		Username     string `json:"username"`
		PasswordHash string `json:"password_hash"`
		Password     string `json:"password"`
	}
	if err := json.Unmarshal(cmd.Payload, &p); err != nil {
		return fmt.Errorf("decode auth recovery payload: %w", err)
	}
	if p.UserID == "" {
		p.UserID = cmd.AggregateID
		if p.UserID == "" {
			p.UserID = p.ClientID
		}
		if p.UserID == "" {
			p.UserID = cmd.RecoveryPrincipalID
		}
	}
	if strings.TrimSpace(p.UserID) == "" {
		return resilience.Permanent(fmt.Errorf("auth recovery command %s is missing user_id", cmd.CommandID))
	}
	var result any
	var err error
	switch strings.ToLower(cmd.Operation) {
	case "post", "create":
		if strings.TrimSpace(p.PasswordHash) == "" {
			// Legacy monolith fallback commands contain the signup password,
			// while the auth application contract stores a bcrypt hash. Normalize
			// that transport payload at the recovery boundary and keep the
			// application layer identical to the gRPC path.
			if strings.TrimSpace(p.Password) == "" {
				return resilience.Permanent(errors.New("auth recovery payload has neither password_hash nor password"))
			}
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
			if hashErr != nil {
				return fmt.Errorf("hash auth recovery password: %w", hashErr)
			}
			p.PasswordHash = string(hash)
		}
		result, err = s.service.CreateCredential(ctx, p.UserID, p.Email, p.Username, p.PasswordHash)
	case "delete":
		err = s.service.DeleteCredential(ctx, p.UserID)
	default:
		return fmt.Errorf("unsupported auth recovery operation=%s", cmd.Operation)
	}
	if err != nil {
		return err
	}
	completed := events.Envelope{EventID: cmd.EventID + ".completed", CommandID: cmd.CommandID, CorrelationID: cmd.CorrelationID, CausationID: cmd.EventID, IdempotencyKey: cmd.IdempotencyKey, TestRunID: cmd.TestRunID, EventType: "migration.recovery.completed", Operation: cmd.Operation, SchemaVersion: 1, AggregateType: "auth", AggregateID: p.UserID, SourceService: "auth-service-recovery", OccurredAt: time.Now().UTC(), Payload: marshal(result)}
	body, err := json.Marshal(completed)
	if err != nil {
		return err
	}
	s.log.Info("auth recovery command completed", logging.Operation("auth.recovery.completed"), logging.String("command_id", cmd.CommandID), logging.String("aggregate_id", p.UserID))
	return s.broker.Publish(context.WithoutCancel(ctx), s.cfg.RecoveryCompleted, body)
}

func marshal(v any) []byte { b, _ := json.Marshal(v); return b }
