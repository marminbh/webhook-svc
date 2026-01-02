package rabbitmq

import (
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/marminbh/webhook-svc/internal/config"
)

// Connection manages RabbitMQ connection and channel with automatic recovery
type Connection struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	config       *config.RabbitMQConfig
	logger       *zap.Logger
	connClose    chan *amqp.Error
	channelClose chan *amqp.Error
	stopChan     chan struct{}
	mu           sync.RWMutex
	reconnecting bool
	reconnectMu  sync.Mutex
}

// NewConnection creates a new Connection instance
func NewConnection(rabbitMQConfig *config.RabbitMQConfig, logger *zap.Logger) *Connection {
	return &Connection{
		config:   rabbitMQConfig,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Connect establishes a connection to RabbitMQ and starts monitoring for reconnection
func (c *Connection) Connect() error {
	// Retry initial connection with exponential backoff
	backoff := time.Second
	maxBackoff := 30 * time.Second
	attempt := 0
	maxInitialAttempts := 10

	for attempt < maxInitialAttempts {
		attempt++
		c.logger.Info("Attempting initial connection to RabbitMQ",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxInitialAttempts),
		)

		if err := c.connect(); err != nil {
			if attempt >= maxInitialAttempts {
				return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxInitialAttempts, err)
			}

			c.logger.Warn("Initial connection to RabbitMQ failed, retrying...",
				zap.Error(err),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			time.Sleep(backoff)
			// Exponential backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Connection successful
		c.logger.Info("Initial connection to RabbitMQ established",
			zap.Int("attempt", attempt),
		)
		break
	}

	// Start monitoring connection for automatic reconnection
	go c.monitorConnection()

	return nil
}

// connect performs the actual connection logic
func (c *Connection) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	// Close existing connection if any
	if c.conn != nil && !c.conn.IsClosed() {
		c.conn.Close()
	}
	if c.channel != nil && !c.channel.IsClosed() {
		c.channel.Close()
	}

	// Create connection
	// Heartbeat: 10 seconds (helps detect dead connections quickly)
	amqpConfig := amqp.Config{
		Heartbeat:  10 * time.Second,
		Locale:     "en_US",
		ChannelMax: 0, // 0 = unlimited
		FrameSize:  0, // 0 = unlimited
		Vhost:      c.config.VHost,
		Properties: amqp.Table{
			"connection_name": "webhook-svc",
		},
	}

	// Parse connection URL and create connection with config
	c.conn, err = amqp.DialConfig(c.config.ConnectionURL(), amqpConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	c.logger.Info("Successfully connected to RabbitMQ",
		zap.String("host", c.config.Host),
		zap.String("port", c.config.Port),
		zap.String("vhost", c.config.VHost),
		zap.Duration("heartbeat", amqpConfig.Heartbeat),
	)
	return nil
}

// monitorConnection monitors the connection and automatically reconnects on failure
func (c *Connection) monitorConnection() {
	for {
		// Set up close notification channels
		c.mu.RLock()
		if c.conn == nil || c.channel == nil {
			c.mu.RUnlock()
			c.logger.Error("Connection or channel not initialized, cannot monitor connection")
			return
		}

		connClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))
		channelClose := c.channel.NotifyClose(make(chan *amqp.Error, 1))
		c.mu.RUnlock()

		select {
		case <-c.stopChan:
			return
		case err := <-connClose:
			if err != nil {
				c.logger.Error("RabbitMQ connection closed, attempting to reconnect",
					zap.Error(err),
					zap.String("reason", err.Reason),
				)
				c.reconnect()
				// Continue monitoring after reconnection
				continue
			}
		case err := <-channelClose:
			if err != nil {
				c.logger.Error("RabbitMQ channel closed, attempting to reconnect",
					zap.Error(err),
					zap.String("reason", err.Reason),
				)
				c.reconnect()
				// Continue monitoring after reconnection
				continue
			}
		}
	}
}

// reconnect attempts to reconnect with exponential backoff
func (c *Connection) reconnect() {
	c.reconnectMu.Lock()
	if c.reconnecting {
		c.reconnectMu.Unlock()
		return // Already reconnecting
	}
	c.reconnecting = true
	c.reconnectMu.Unlock()

	defer func() {
		c.reconnectMu.Lock()
		c.reconnecting = false
		c.reconnectMu.Unlock()
	}()

	backoff := time.Second
	maxBackoff := 30 * time.Second
	attempt := 0

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		attempt++
		c.logger.Info("Attempting to reconnect to RabbitMQ",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
		)

		if err := c.connect(); err != nil {
			c.logger.Warn("Failed to reconnect to RabbitMQ, retrying...",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			time.Sleep(backoff)
			// Exponential backoff with max limit
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Reconnection successful
		c.logger.Info("Successfully reconnected to RabbitMQ",
			zap.Int("attempt", attempt),
		)

		// Reset backoff for next time
		backoff = time.Second
		return
	}
}

// Close closes the RabbitMQ connection and channel and stops reconnection monitoring
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Signal to stop reconnection monitoring
	select {
	case <-c.stopChan:
		// Already closed
	default:
		close(c.stopChan)
	}

	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.logger.Info("RabbitMQ connection closed")
	}
}

// PublishMessage publishes a message to a queue with retry on connection loss
func (c *Connection) PublishMessage(exchange, routingKey string, mandatory, immediate bool, body []byte) error {
	maxRetries := 3
	retryDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.mu.RLock()
		ch := c.channel
		conn := c.conn
		c.mu.RUnlock()

		if ch == nil || ch.IsClosed() || conn == nil || conn.IsClosed() {
			if attempt < maxRetries-1 {
				c.logger.Warn("RabbitMQ channel not available for publish, retrying...",
					zap.Int("attempt", attempt+1),
					zap.Int("max_retries", maxRetries),
				)
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff for retries
				continue
			}
			return fmt.Errorf("RabbitMQ channel is not initialized or closed after %d attempts", maxRetries)
		}

		err := ch.Publish(
			exchange,   // exchange
			routingKey, // routing key
			mandatory,  // mandatory
			immediate,  // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
				Body:         body,
			},
		)

		if err != nil {
			// Check if it's a connection error that might be recoverable
			isConnectionError := ch.IsClosed() || conn == nil
			if !isConnectionError {
				isConnectionError = conn.IsClosed()
			}
			if attempt < maxRetries-1 && isConnectionError {
				c.logger.Warn("Publish failed due to connection issue, retrying...",
					zap.Error(err),
					zap.Int("attempt", attempt+1),
				)
				time.Sleep(retryDelay)
				retryDelay *= 2
				continue
			}
			return fmt.Errorf("failed to publish message: %w", err)
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to publish message after %d attempts", maxRetries)
}

// ConsumeMessages starts consuming messages from a queue
func (c *Connection) ConsumeMessages(queue, consumer string, autoAck, exclusive, noLocal, noWait bool) (<-chan amqp.Delivery, error) {
	c.mu.RLock()
	ch := c.channel
	c.mu.RUnlock()

	if ch == nil || ch.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ channel is not initialized or closed")
	}

	messages, err := ch.Consume(
		queue,     // queue
		consumer,  // consumer
		autoAck,   // auto-ack
		exclusive, // exclusive
		noLocal,   // no-local
		noWait,    // no-wait
		nil,       // args
	)

	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	return messages, nil
}

// SetQoS sets the quality of service (prefetch count) for the channel
func (c *Connection) SetQoS(prefetchCount, prefetchSize int, global bool) error {
	c.mu.RLock()
	ch := c.channel
	c.mu.RUnlock()

	if ch == nil || ch.IsClosed() {
		return fmt.Errorf("RabbitMQ channel is not initialized or closed")
	}

	err := ch.Qos(
		prefetchCount, // prefetch count
		prefetchSize,  // prefetch size (0 = unlimited)
		global,        // global (apply to all consumers on this channel)
	)

	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	return nil
}

// GetChannel returns the current channel (for backward compatibility if needed)
func (c *Connection) GetChannel() *amqp.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channel
}

// IsHealthy checks if the connection and channel are healthy
func (c *Connection) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.conn.IsClosed() && c.channel != nil && !c.channel.IsClosed()
}
