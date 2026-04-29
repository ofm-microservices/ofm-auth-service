package nats

import (
	"encoding/json"
	"time"
)

type registrationSagaMessageMapper struct{}

func newRegistrationSagaMessageMapper() RegistrationSagaMessageMapper {
	return &registrationSagaMessageMapper{}
}

func (m *registrationSagaMessageMapper) ToCreateFailureResultPayload(cmd createAuthCommand, reason string) ([]byte, error) {
	payload, err := json.Marshal(createAuthResult{
		SessionID: cmd.SessionID,
		ClientID:  cmd.ClientID,
		UserID:    cmd.UserID,
		SagaID:    cmd.SagaID,
		Status:    "failed",
		Error:     reason,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalCreateAuthResultError(err)
	}

	return payload, nil
}

func (m *registrationSagaMessageMapper) ToCreateSuccessResultPayload(cmd createAuthCommand, email, verificationCode, expiresIn string) ([]byte, error) {
	payload, err := json.Marshal(createAuthResult{
		SessionID:        cmd.SessionID,
		ClientID:         cmd.ClientID,
		UserID:           cmd.UserID,
		SagaID:           cmd.SagaID,
		Email:            email,
		VerificationCode: verificationCode,
		ExpiresIn:        expiresIn,
		Status:           "success",
		Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalCreateAuthResultError(err)
	}

	return payload, nil
}

func (m *registrationSagaMessageMapper) ToDeleteFailureResultPayload(cmd deleteAuthCommand, reason string) ([]byte, error) {
	payload, err := json.Marshal(deleteAuthResult{
		UserID:    cmd.UserID,
		SagaID:    cmd.SagaID,
		Status:    "failed",
		Error:     reason,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalDeleteAuthResultError(err)
	}

	return payload, nil
}

func (m *registrationSagaMessageMapper) ToDeleteSuccessResultPayload(cmd deleteAuthCommand) ([]byte, error) {
	payload, err := json.Marshal(deleteAuthResult{
		UserID:    cmd.UserID,
		SagaID:    cmd.SagaID,
		Status:    "success",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalDeleteAuthResultError(err)
	}

	return payload, nil
}

func (m *registrationSagaMessageMapper) ToMailSendCommandPayload(cmd createAuthCommand, email, verificationCode, expiresIn string) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"session_id":     cmd.SessionID,
		"client_id":      cmd.ClientID,
		"user_id":        cmd.UserID,
		"request_id":     cmd.SessionID,
		"correlation_id": cmd.SessionID,
		"message_type":   "email_code",
		"to":             email,
		"data": map[string]any{
			"name":       email,
			"code":       verificationCode,
			"expires_in": expiresIn,
		},
	})
	if err != nil {
		return nil, WrapMarshalMailSendCommandError(err)
	}

	return payload, nil
}
