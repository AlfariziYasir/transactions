package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Name            string `env:"APP_NAME"`
	Version         string `env:"APP_VERSION"`
	GrpcPort        int    `env:"GRPC_PORT"`
	GrpcHost        string `env:"GRPC_HOST"`
	HtpPort         int    `env:"HTTP_PORT"`
	AccessTokenKey  string `env:"ACCESS_TOKEN_KEY"`
	RefreshTokenKey string `env:"REFRESH_TOKEN_KEY"`
	DbDsn           string `env:"DB_DSN"`
	LogLevel        string `env:"LOG_LEVEL"`
	RedisAddress    string `env:"REDIS_ADDRESS"`
	RedisPassword   string `env:"REDIS_PASSWORD"`
	RedisDB         int    `env:"REDIS_DB"`
	RabbitMQUrl     string `env:"RABBITMQ_URL"`
	SwaggerPath     string `env:"SWAGGER_PATH"`
}

func NewConfig() (*Config, error) {
	config := Config{}

	if err := env.Parse(&config); err != nil {
		return nil, fmt.Errorf("read env error: %w", err)
	}

	return &config, nil
}
