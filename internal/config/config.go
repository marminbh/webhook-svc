package config

import (
	"fmt"
	"os"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
}

type ServerConfig struct {
	Port string
	Host string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RabbitMQConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

func Load() (*Config, error) {
	var missing []string

	get := func(key string) string {
		val := os.Getenv(key)
		if val == "" {
			missing = append(missing, key)
		}
		return val
	}

	config := &Config{
		Server: ServerConfig{
			Port: get("SERVER_PORT"),
			Host: get("SERVER_HOST"),
		},
		Database: DatabaseConfig{
			Host:     get("DB_HOST"),
			Port:     get("DB_PORT"),
			User:     get("DB_USER"),
			Password: get("DB_PASSWORD"),
			DBName:   get("DB_NAME"),
			SSLMode:  get("DB_SSLMODE"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:      os.Getenv("RABBITMQ_URL"),
			Host:     get("RABBITMQ_HOST"),
			Port:     get("RABBITMQ_PORT"),
			User:     get("RABBITMQ_USER"),
			Password: get("RABBITMQ_PASSWORD"),
			VHost:    get("RABBITMQ_VHOST"),
		},
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return config, nil
}

// ConnectionString returns a DSN string for GORM
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		c.Host, c.User, c.Password, c.DBName, c.Port, c.SSLMode)
}

func (c *RabbitMQConfig) ConnectionURL() string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf("amqp://%s:%s@%s:%s%s",
		c.User, c.Password, c.Host, c.Port, c.VHost)
}
