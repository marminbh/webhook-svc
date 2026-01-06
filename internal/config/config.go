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
	Worker     WorkerConfig
}

type ServerConfig struct {
	Port     string
	Host     string
	LogLevel string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // in minutes
	ConnMaxIdleTime int // in minutes
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
	MaxAttempts        int // Maximum retry attempts for webhook events
	BatchSize          int // Batch size for creating webhook events
}

type WorkerConfig struct {
	DeliveryQueue       string // Queue to consume from
	PrefetchCount       int    // RabbitMQ QoS prefetch
	HTTPTimeout         int    // HTTP request timeout in seconds (default 10)
	MaxResponseBodySize int    // Max bytes to store in attempt log (default 1000)
}

func Load() (*Config, error) {
	var missingVars []string

	serverPort := getEnv("SERVER_PORT", &missingVars)
	serverHost := getEnv("SERVER_HOST", &missingVars)
	serverLogLevel := getEnvWithDefault("LOG_LEVEL", "info")

	dbHost := getEnv("DB_HOST", &missingVars)
	dbPort := getEnv("DB_PORT", &missingVars)
	dbUser := getEnv("DB_USER", &missingVars)
	dbPassword := getEnv("DB_PASSWORD", &missingVars)
	dbName := getEnv("DB_NAME", &missingVars)
	dbSSLMode := getEnv("DB_SSLMODE", &missingVars)
	dbMaxOpenConns, intErr := getEnvIntWithDefault("DB_MAX_OPEN_CONNS", 25)
	if intErr != nil {
		return nil, intErr
	}
	dbMaxIdleConns, intErr := getEnvIntWithDefault("DB_MAX_IDLE_CONNS", 5)
	if intErr != nil {
		return nil, intErr
	}
	dbConnMaxLifetime, intErr := getEnvIntWithDefault("DB_CONN_MAX_LIFETIME_MIN", 5)
	if intErr != nil {
		return nil, intErr
	}
	dbConnMaxIdleTime, intErr := getEnvIntWithDefault("DB_CONN_MAX_IDLE_TIME_MIN", 1)
	if intErr != nil {
		return nil, intErr
	}

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
	if intErr != nil {
		return nil, intErr
	}
	dispatcherMaxAttempts, intErr := getEnvIntWithDefault("DISPATCHER_MAX_ATTEMPTS", 8)
	if intErr != nil {
		return nil, intErr
	}
	dispatcherBatchSize, intErr := getEnvIntWithDefault("DISPATCHER_BATCH_SIZE", 100)
	if intErr != nil {
		return nil, intErr
	}

	workerDeliveryQueue := getEnv("WORKER_DELIVERY_QUEUE", &missingVars)
	workerPrefetchCount, intErr := getEnvInt("WORKER_PREFETCH_COUNT", &missingVars)
	if intErr != nil {
		return nil, intErr
	}
	workerHTTPTimeout, intErr := getEnvIntWithDefault("WORKER_HTTP_TIMEOUT", 10)
	if intErr != nil {
		return nil, intErr
	}
	workerMaxResponseBodySize, intErr := getEnvIntWithDefault("WORKER_MAX_RESPONSE_BODY_SIZE", 1000)
	if intErr != nil {
		return nil, intErr
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	config := &Config{
		Server: ServerConfig{
			Port:     serverPort,
			Host:     serverHost,
			LogLevel: serverLogLevel,
		},
		Database: DatabaseConfig{
			Host:            dbHost,
			Port:            dbPort,
			User:            dbUser,
			Password:        dbPassword,
			DBName:          dbName,
			SSLMode:         dbSSLMode,
			MaxOpenConns:    dbMaxOpenConns,
			MaxIdleConns:    dbMaxIdleConns,
			ConnMaxLifetime: dbConnMaxLifetime,
			ConnMaxIdleTime: dbConnMaxIdleTime,
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
			MaxAttempts:        dispatcherMaxAttempts,
			BatchSize:          dispatcherBatchSize,
		},
		Worker: WorkerConfig{
			DeliveryQueue:       workerDeliveryQueue,
			PrefetchCount:       workerPrefetchCount,
			HTTPTimeout:         workerHTTPTimeout,
			MaxResponseBodySize: workerMaxResponseBodySize,
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

// getEnvWithDefault retrieves an environment variable with a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvIntWithDefault retrieves an environment variable as an integer with a default value
func getEnvIntWithDefault(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s must be a valid integer: %w", key, err)
	}

	return result, nil
}
