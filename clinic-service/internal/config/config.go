package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	JWTSecret string
}

type ServerConfig struct {
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	DBName   string
	User     string
	Password string
}

type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "postgres"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			DBName:   getEnv("POSTGRES_DBNAME", "clinic"),
			User:     getEnv("POSTGRES_USER", "admin"),
			Password: getEnv("POSTGRES_PASSWORD", "admin"),
		},
		RabbitMQ: RabbitMQConfig{
			Host:     getEnv("RABBIT_HOST", "rabbitmq"),
			Port:     getEnv("RABBIT_PORT", "5672"),
			User:     getEnv("RABBIT_USER", "guest"),
			Password: getEnv("RABBIT_PASSWORD", "guest"),
			VHost:    getEnv("RABBIT_VHOST", "/"),
		},
		JWTSecret: getEnv("JWT_SECRET", "clinic-secret-key-change-in-prod"),
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}