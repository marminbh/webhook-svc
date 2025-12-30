package rabbitmq

import (
	"fmt"
	"log"
	"time"

	"github.com/marminbh/webhook-svc/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
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

	log.Println("Successfully connected to RabbitMQ")
	return nil
}

// Close closes the RabbitMQ connection and channel
func Close() {
	if Channel != nil {
		Channel.Close()
	}
	if Conn != nil {
		Conn.Close()
		log.Println("RabbitMQ connection closed")
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
