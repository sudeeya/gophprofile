package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Minio    MinioConfig
	Rabbitmq RabbitmqConfig
}

type ServerConfig struct {
	Port            string        `env:"SERVER_PORT,required"`
	ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT,required"`
}

type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST,required"`
	Port     string `env:"POSTGRES_PORT,required"`
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	DB       string `env:"POSTGRES_DB,required"`
}

type MinioConfig struct {
	Endpoint string `env:"MINIO_ENDPOINT,required"`
	User     string `env:"MINIO_ROOT_USER,required"`
	Password string `env:"MINIO_ROOT_PASSWORD,required"`
}

type RabbitmqConfig struct {
	Host                           string        `env:"RABBITMQ_HOST,required"`
	Port                           string        `env:"RABBITMQ_PORT,required"`
	User                           string        `env:"RABBITMQ_DEFAULT_USER,required"`
	Password                       string        `env:"RABBITMQ_DEFAULT_PASS,required"`
	PublisherConfirmsRetryAttempts uint          `env:"RABBITMQ_PUBLISHER_CONFIRMS_RETRY_ATTEMPTS" envDefault:"5"`
	PublisherConfirmsRetryDelay    time.Duration `env:"RABBITMQ_PUBLISHER_CONFIRMS_RETRY_DELAY" envDefault:"100ms"`
}

func New() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
