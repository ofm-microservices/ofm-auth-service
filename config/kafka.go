package config

// KafkaConfig defines the Kafka broker and consumer group used by auth-service.
type KafkaConfig struct {
	Brokers           []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"127.0.0.1:9092"`
	GroupID           string   `env:"KAFKA_AUTH_GROUP_ID" envDefault:"auth-service"`
	CreateTopic       string   `env:"KAFKA_AUTH_CREATE_TOPIC" envDefault:"saga.auth.create_pending_registration"`
	DeleteTopic       string   `env:"KAFKA_AUTH_DELETE_TOPIC" envDefault:"saga.auth.delete"`
	CreateResultTopic string   `env:"KAFKA_AUTH_CREATE_RESULT_TOPIC" envDefault:"saga.auth.create_pending_registration.result"`
	DeleteResultTopic string   `env:"KAFKA_AUTH_DELETE_RESULT_TOPIC" envDefault:"saga.auth.delete.result"`
	MailTopic         string   `env:"KAFKA_AUTH_MAIL_TOPIC" envDefault:"mail.send"`
	DeadLetterTopic   string   `env:"KAFKA_AUTH_DLQ_TOPIC" envDefault:"auth-service.dead-letter"`
	RecoveryTopic     string   `env:"KAFKA_AUTH_RECOVERY_TOPIC" envDefault:"migration.recovery.commands.auth"`
	RecoveryGroup     string   `env:"KAFKA_AUTH_RECOVERY_GROUP" envDefault:"auth-service-recovery"`
	RecoveryCompleted string   `env:"KAFKA_AUTH_RECOVERY_COMPLETED_TOPIC" envDefault:"migration.recovery.completed"`
}
