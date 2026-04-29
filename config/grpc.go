package config

// GRPCConfig defines the gRPC listener used for auth ownership queries.
type GRPCConfig struct {
	Host string `env:"GRPC_HOST" envDefault:"0.0.0.0"`
	Port int    `env:"GRPC_PORT" envDefault:"9091"`
}
