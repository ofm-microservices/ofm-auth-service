package nats

import (
	"context"
	"encoding/json"
)

func (s *registrationSagaSubscriber) handleCreateAuthCommand(ctx context.Context, _ string, payload []byte) error {
	var cmd createAuthCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return WrapUnmarshalCreateAuthCommandError(err)
	}

	result, err := s.service.CreatePendingRegistration(ctx, cmd.UserID, cmd.Email, cmd.Username, cmd.PasswordHash)
	if err != nil {
		return s.publishCreateFailureResult(ctx, cmd, err)
	}

	return s.publishCreateSuccessResult(ctx, cmd, result.Email, result.VerificationCode, result.ExpiresIn)
}

func (s *registrationSagaSubscriber) handleDeleteAuthCommand(ctx context.Context, _ string, payload []byte) error {
	var cmd deleteAuthCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return WrapUnmarshalDeleteAuthCommandError(err)
	}

	if err := s.service.DeleteCredential(ctx, cmd.UserID); err != nil {
		return s.publishDeleteFailureResult(ctx, cmd, err)
	}

	return s.publishDeleteSuccessResult(ctx, cmd)
}

func (s *registrationSagaSubscriber) publishCreateFailureResult(ctx context.Context, cmd createAuthCommand, err error) error {
	payload, mapErr := s.mapr.ToCreateFailureResultPayload(cmd, s.resolver.CreateAuthFailureReason(err))
	if mapErr != nil {
		return mapErr
	}

	return s.broker.Publish(ctx, s.cfg.SagaCreateAuthResultSubject, payload)
}

func (s *registrationSagaSubscriber) publishCreateSuccessResult(
	ctx context.Context,
	cmd createAuthCommand,
	email string,
	verificationCode string,
	expiresIn string,
) error {
	resultPayload, err := s.mapr.ToCreateSuccessResultPayload(cmd, email, verificationCode, expiresIn)
	if err != nil {
		return err
	}
	if err := s.broker.Publish(ctx, s.cfg.SagaCreateAuthResultSubject, resultPayload); err != nil {
		return err
	}

	mailPayload, err := s.mapr.ToMailSendCommandPayload(cmd, email, verificationCode, expiresIn)
	if err != nil {
		return err
	}

	return s.broker.Publish(ctx, s.cfg.MailSendSubject, mailPayload)
}

func (s *registrationSagaSubscriber) publishDeleteFailureResult(ctx context.Context, cmd deleteAuthCommand, err error) error {
	payload, mapErr := s.mapr.ToDeleteFailureResultPayload(cmd, s.resolver.DeleteAuthFailureReason(err))
	if mapErr != nil {
		return mapErr
	}

	return s.broker.Publish(ctx, s.cfg.SagaDeleteAuthResultSubject, payload)
}

func (s *registrationSagaSubscriber) publishDeleteSuccessResult(ctx context.Context, cmd deleteAuthCommand) error {
	resultPayload, err := s.mapr.ToDeleteSuccessResultPayload(cmd)
	if err != nil {
		return err
	}

	return s.broker.Publish(ctx, s.cfg.SagaDeleteAuthResultSubject, resultPayload)
}
