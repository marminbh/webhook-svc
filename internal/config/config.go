package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	RabbitMQ   RabbitMQConfig
	Dispatcher DispatcherConfig
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

type DispatcherConfig struct {
	SourceExchange     string
	SourceQueue        string
	SourceRoutingKey   string
	DeliveryExchange   string
	DeliveryRoutingKey string
	DeliveryQueue      string
	PrefetchCount      int
}

func Load() (*Config, error) {
	var missingVars []string

	serverPort := getEnv("SERVER_PORT", &missingVars)
	serverHost := getEnv("SERVER_HOST", &missingVars)

	dbHost := getEnv("DB_HOST", &missingVars)
	dbPort := getEnv("DB_PORT", &missingVars)
	dbUser := getEnv("DB_USER", &missingVars)
	dbPassword := getEnv("DB_PASSWORD", &missingVars)
	dbName := getEnv("DB_NAME", &missingVars)
	dbSSLMode := getEnv("DB_SSLMODE", &missingVars)

	rabbitMQURL := os.Getenv("RABBITMQ_URL") // Optional, can be empty
	rabbitMQHost := getEnv("RABBITMQ_HOST", &missingVars)
	rabbitMQPort := getEnv("RABBITMQ_PORT", &missingVars)
	rabbitMQUser := getEnv("RABBITMQ_USER", &missingVars)
	rabbitMQPassword := getEnv("RABBITMQ_PASSWORD", &missingVars)
	rabbitMQVHost := getEnv("RABBITMQ_VHOST", &missingVars)

	dispatcherSourceExchange := getEnv("DISPATCHER_SOURCE_EXCHANGE", &missingVars)
	dispatcherSourceQueue := getEnv("DISPATCHER_SOURCE_QUEUE", &missingVars)
	dispatcherSourceRoutingKey := getEnv("DISPATCHER_SOURCE_ROUTING_KEY", &missingVars)
	dispatcherDeliveryExchange := getEnv("DISPATCHER_DELIVERY_EXCHANGE", &missingVars)
	dispatcherDeliveryRoutingKey := getEnv("DISPATCHER_DELIVERY_ROUTING_KEY", &missingVars)
	dispatcherDeliveryQueue := getEnv("DISPATCHER_DELIVERY_QUEUE", &missingVars)
	dispatcherPrefetchCount, intErr := getEnvInt("DISPATCHER_PREFETCH_COUNT", &missingVars)

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	if intErr != nil {
		return nil, intErr
	}

	config := &Config{
		Server: ServerConfig{
			Port: serverPort,
			Host: serverHost,
		},
		Database: DatabaseConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
			SSLMode:  dbSSLMode,
		},
		RabbitMQ: RabbitMQConfig{
			URL:      rabbitMQURL,
			Host:     rabbitMQHost,
			Port:     rabbitMQPort,
			User:     rabbitMQUser,
			Password: rabbitMQPassword,
			VHost:    rabbitMQVHost,
		},
		Dispatcher: DispatcherConfig{
			SourceExchange:     dispatcherSourceExchange,
			SourceQueue:        dispatcherSourceQueue,
			SourceRoutingKey:   dispatcherSourceRoutingKey,
			DeliveryExchange:   dispatcherDeliveryExchange,
			DeliveryRoutingKey: dispatcherDeliveryRoutingKey,
			DeliveryQueue:      dispatcherDeliveryQueue,
			PrefetchCount:      dispatcherPrefetchCount,
		},
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

// getEnv retrieves an environment variable and adds it to missingVars if it's not set
// This collects all missing variables to return a consolidated error
func getEnv(key string, missingVars *[]string) string {
	value := os.Getenv(key)
	if value == "" {
		*missingVars = append(*missingVars, key)
		return ""
	}
	return value
}

// getEnvInt retrieves an environment variable as an integer
// Returns the value, and an error if it's not set or invalid
func getEnvInt(key string, missingVars *[]string) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		*missingVars = append(*missingVars, key)
		return 0, nil
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s must be a valid integer: %w", key, err)
	}

	return result, nil
}
