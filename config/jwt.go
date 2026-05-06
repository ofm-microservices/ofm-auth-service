package config

import "time"

// JWTConfig defines auth-service token issuing settings.
type JWTConfig struct {
	Secret            string        `env:"JWT_SECRET" envDefault:"local-dev-secret-change-me"`
	Issuer            string        `env:"JWT_ISSUER" envDefault:"ofm-auth-service"`
	AccessTokenTTL    time.Duration `env:"JWT_ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL   time.Duration `env:"JWT_REFRESH_TOKEN_TTL" envDefault:"720h"`
	RefreshTokenBytes int           `env:"JWT_REFRESH_TOKEN_BYTES" envDefault:"32"`
}
