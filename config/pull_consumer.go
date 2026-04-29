package config

import "time"

// PullConsumerConfig defines the runtime behavior of one pull-consumer worker
// group.
type PullConsumerConfig struct {
	Stream     string
	Subject    string
	Durable    string
	BatchSize  int
	MaxWait    time.Duration
	Workers    int
	QueueSize  int
	AckWait    time.Duration
	MaxDeliver int
	Adaptive   PullAdaptiveConfig
}

// PullAdaptiveConfig defines the adaptive batching thresholds for pull
// consumers.
type PullAdaptiveConfig struct {
	Enabled         bool
	CheckInterval   time.Duration
	MediumPending   int
	HighPending     int
	LowBatchSize    int
	LowMaxWait      time.Duration
	MediumBatchSize int
	MediumMaxWait   time.Duration
	HighBatchSize   int
	HighMaxWait     time.Duration
}
