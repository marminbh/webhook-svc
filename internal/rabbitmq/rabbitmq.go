package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/marminbh/webhook-svc/internal/config"
	"github.com/marminbh/webhook-svc/internal/logger"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

// Connect establishes a connection to RabbitMQ
func Connect(cfg *config.RabbitMQConfig) error {
	var err error

	// Create connection
	Conn, err = amqp.Dial(cfg.ConnectionURL())
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	Channel, err = Conn.Channel()
	if err != nil {
		Conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	logger.Info("Successfully connected to RabbitMQ",
		zap.String("host", cfg.Host),
		zap.String("port", cfg.Port),
		zap.String("vhost", cfg.VHost),
	)
	return nil
}

// Close closes the RabbitMQ connection and channel
func Close() {
	if Channel != nil {
		Channel.Close()
	}
	if Conn != nil {
		Conn.Close()
		logger.Info("RabbitMQ connection closed")
	}
}

// DeclareQueue declares a queue if it doesn't exist
func DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool) (amqp.Queue, error) {
	if Channel == nil {
		return amqp.Queue{}, fmt.Errorf("RabbitMQ channel is not initialized")
	}

	queue, err := Channel.QueueDeclare(
		name,       // queue name
		durable,    // durable
		autoDelete, // delete when unused
		exclusive,  // exclusive
		noWait,     // no-wait
		nil,        // arguments
	)

	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to declare queue: %w", err)
	}

	return queue, nil
}

// PublishMessage publishes a message to a queue
func PublishMessage(exchange, routingKey string, mandatory, immediate bool, body []byte) error {
	if Channel == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}

	return Channel.Publish(
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
}

// ConsumeMessages starts consuming messages from a queue
func ConsumeMessages(queue, consumer string, autoAck, exclusive, noLocal, noWait bool) (<-chan amqp.Delivery, error) {
	if Channel == nil {
		return nil, fmt.Errorf("RabbitMQ channel is not initialized")
	}

	messages, err := Channel.Consume(
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

// DeclareExchange declares an exchange if it doesn't exist
func DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool) error {
	if Channel == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}

	err := Channel.ExchangeDeclare(
		name,       // exchange name
		kind,       // exchange type (direct, topic, fanout, headers)
		durable,    // durable
		autoDelete, // delete when unused
		internal,   // internal
		noWait,     // no-wait
		nil,        // arguments
	)

	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", name, err)
	}

	return nil
}

// BindQueue binds a queue to an exchange with a routing key
func BindQueue(queue, routingKey, exchange string, noWait bool) error {
	if Channel == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}

	err := Channel.QueueBind(
		queue,      // queue name
		routingKey, // routing key
		exchange,   // exchange name
		noWait,     // no-wait
		nil,        // arguments
	)

	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s with routing key %s: %w", queue, exchange, routingKey, err)
	}

	return nil
}

// SetQoS sets the quality of service (prefetch count) for the channel
func SetQoS(prefetchCount, prefetchSize int, global bool) error {
	if Channel == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}

	err := Channel.Qos(
		prefetchCount, // prefetch count
		prefetchSize,  // prefetch size (0 = unlimited)
		global,        // global (apply to all consumers on this channel)
	)

	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	return nil
}
