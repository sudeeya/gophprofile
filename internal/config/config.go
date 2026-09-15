package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Server ServerConfig
}

type ServerConfig struct {
	Port            string        `env:"SERVER_PORT,required"`
	ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT,required"`
}

func New() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
