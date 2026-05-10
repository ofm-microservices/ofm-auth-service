package nats

import (
	"auth-service/config"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"time"

	"github.com/nats-io/nats.go"
)

// EnsureStream creates or updates the JetStream streams required by
// auth-service.
func EnsureStream(cfg config.NATSConfig, log logging.Logger) error {
	if log == nil {
		return ErrNilLogger
	}

	lg := log.With(logging.String("module", "jetstream-bootstrap"))
	lg.Info("ensuring jetstream stream",
		logging.String("stream", cfg.AuthEventsStream),
		logging.String("subject", cfg.AuthCreatedSubject),
	)

	nc, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return WrapInitJetStreamContextError(err)
	}

	streamCfg := &nats.StreamConfig{
		Name:      cfg.AuthEventsStream,
		Subjects:  []string{cfg.AuthCreatedSubject, cfg.SagaCreateAuthResultSubject, cfg.SagaDeleteAuthResultSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	_, err = js.AddStream(streamCfg)
	if err != nil {
		if _, updateErr := js.UpdateStream(streamCfg); updateErr != nil {
			return WrapEnsureStreamError(streamCfg.Name, err, updateErr)
		}
	}

	sagaStreamCfg := &nats.StreamConfig{
		Name:      cfg.SagaCommandsStream,
		Subjects:  []string{cfg.SagaCreateAuthSubject, cfg.SagaDeleteAuthSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	mailCommandsStream := &nats.StreamConfig{
		Name:      cfg.MailCommandsStream,
		Subjects:  []string{cfg.MailSendSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	if _, err := js.AddStream(mailCommandsStream); err != nil {
		if _, updateErr := js.UpdateStream(mailCommandsStream); updateErr != nil {
			return WrapEnsureStreamError(mailCommandsStream.Name, err, updateErr)
		}
	}

	_, err = js.AddStream(sagaStreamCfg)
	if err != nil {
		if _, updateErr := js.UpdateStream(sagaStreamCfg); updateErr != nil {
			return WrapEnsureStreamError(sagaStreamCfg.Name, err, updateErr)
		}
	}

	lg.Info("jetstream streams ensured",
		logging.String("auth_events_stream", streamCfg.Name),
		logging.String("saga_commands_stream", sagaStreamCfg.Name),
	)
	return nil
}

// Connect establishes the low-level NATS connection used by bootstrap code.
func Connect(cfg config.NATSConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("auth-service"),
		nats.MaxReconnects(-1),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, WrapConnectToNATSError(err)
	}

	return nc, nil
}
