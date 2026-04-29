package config

// AppConfig holds process-level runtime settings for auth-service.
type AppConfig struct {
	Env      string `env:"APP_ENV" envDefault:"local"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}
