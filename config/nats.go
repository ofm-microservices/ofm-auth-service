package config

import "time"

// NATSConfig defines the auth-service command, result, and consumer settings.
type NATSConfig struct {
	URL                         string        `env:"NATS_URL,required"`
	User                        string        `env:"NATS_USER"`
	Password                    string        `env:"NATS_PASSWORD"`
	AuthEventsStream            string        `env:"NATS_STREAM_AUTH_EVENTS" envDefault:"AUTH_EVENTS"`
	MailCommandsStream          string        `env:"NATS_STREAM_MAIL_COMMANDS" envDefault:"MAIL_COMMANDS"`
	AuthCreatedSubject          string        `env:"NATS_SUBJECT_AUTH_CREATED" envDefault:"auth.created"`
	SagaCommandsStream          string        `env:"NATS_STREAM_SAGA_COMMANDS" envDefault:"SAGA_AUTH_COMMANDS"`
	SagaCreateAuthSubject       string        `env:"NATS_SUBJECT_SAGA_CREATE_PENDING_AUTH" envDefault:"saga.auth.create_pending_registration"`
	SagaDeleteAuthSubject       string        `env:"NATS_SUBJECT_SAGA_DELETE_AUTH" envDefault:"saga.auth.delete"`
	SagaCreateAuthResultSubject string        `env:"NATS_SUBJECT_SAGA_CREATE_PENDING_AUTH_RESULT" envDefault:"saga.auth.create_pending_registration.result"`
	SagaDeleteAuthResultSubject string        `env:"NATS_SUBJECT_SAGA_DELETE_AUTH_RESULT" envDefault:"saga.auth.delete.result"`
	MailSendSubject             string        `env:"NATS_SUBJECT_MAIL_SEND" envDefault:"mail.send"`
	SagaCreateAuthDurable       string        `env:"NATS_DURABLE_SAGA_CREATE_AUTH" envDefault:"auth_service_saga_create_pending_registration"`
	SagaDeleteAuthDurable       string        `env:"NATS_DURABLE_SAGA_DELETE_AUTH" envDefault:"auth_service_saga_delete"`
	SagaBatchSize               int           `env:"NATS_SAGA_BATCH_SIZE" envDefault:"32"`
	SagaMaxWait                 time.Duration `env:"NATS_SAGA_MAX_WAIT" envDefault:"10ms"`
	SagaWorkers                 int           `env:"NATS_SAGA_WORKERS" envDefault:"8"`
	SagaQueueSize               int           `env:"NATS_SAGA_QUEUE_SIZE" envDefault:"500"`
	SagaAckWait                 time.Duration `env:"NATS_SAGA_ACK_WAIT" envDefault:"30s"`
	SagaMaxDeliver              int           `env:"NATS_SAGA_MAX_DELIVER" envDefault:"5"`
	SagaAdaptiveEnabled         bool          `env:"NATS_SAGA_ADAPTIVE_ENABLED" envDefault:"false"`
	SagaAdaptiveCheckInterval   time.Duration `env:"NATS_SAGA_ADAPTIVE_CHECK_INTERVAL" envDefault:"2s"`
	SagaAdaptiveMediumPending   int           `env:"NATS_SAGA_ADAPTIVE_MEDIUM_PENDING" envDefault:"200"`
	SagaAdaptiveHighPending     int           `env:"NATS_SAGA_ADAPTIVE_HIGH_PENDING" envDefault:"1000"`
	SagaAdaptiveLowBatchSize    int           `env:"NATS_SAGA_ADAPTIVE_LOW_BATCH_SIZE" envDefault:"8"`
	SagaAdaptiveLowMaxWait      time.Duration `env:"NATS_SAGA_ADAPTIVE_LOW_MAX_WAIT" envDefault:"25ms"`
	SagaAdaptiveMediumBatchSize int           `env:"NATS_SAGA_ADAPTIVE_MEDIUM_BATCH_SIZE" envDefault:"32"`
	SagaAdaptiveMediumMaxWait   time.Duration `env:"NATS_SAGA_ADAPTIVE_MEDIUM_MAX_WAIT" envDefault:"10ms"`
	SagaAdaptiveHighBatchSize   int           `env:"NATS_SAGA_ADAPTIVE_HIGH_BATCH_SIZE" envDefault:"128"`
	SagaAdaptiveHighMaxWait     time.Duration `env:"NATS_SAGA_ADAPTIVE_HIGH_MAX_WAIT" envDefault:"2ms"`
}
