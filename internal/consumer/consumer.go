package consumer

import (
	"encoding/base64"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/marminbh/webhook-svc/internal/logger"
)

// EventHandler is the interface that consumers must implement
// to handle decoded events
type EventHandler interface {
	HandleEvent(decodedMessage string) error
}

// ProcessMessage processes a RabbitMQ message following the abstract consumer pattern:
// 1. Decodes base64-encoded message
// 2. Calls the handler's HandleEvent method
// 3. ACKs on success, NACKs (no requeue) on failure
func ProcessMessage(
	queue string,
	msg amqp.Delivery,
	handler EventHandler,
) {
	logger.Info("Received message from queue",
		zap.String("queue", queue),
		zap.Uint64("delivery_tag", msg.DeliveryTag),
	)

	// Decode the base64-encoded message
	decodedMessage, err := base64.StdEncoding.DecodeString(string(msg.Body))
	if err != nil {
		logger.Error("Failed to decode base64 message from queue",
			zap.String("queue", queue),
			zap.Uint64("delivery_tag", msg.DeliveryTag),
			zap.Error(err),
		)
		rejectMessage(msg)
		return
	}

	// Process the decoded message
	if err := handler.HandleEvent(string(decodedMessage)); err != nil {
		logger.Error("Failed to process message from queue",
			zap.String("queue", queue),
			zap.Uint64("delivery_tag", msg.DeliveryTag),
			zap.String("decoded_message", string(decodedMessage)),
			zap.Error(err),
		)
		rejectMessage(msg)
		return
	}

	// ACK the message on success
	if err := msg.Ack(false); err != nil {
		logger.Error("Failed to ack message from queue",
			zap.String("queue", queue),
			zap.Uint64("delivery_tag", msg.DeliveryTag),
			zap.Error(err),
		)
		rejectMessage(msg)
		return
	}

	logger.Info("Message from queue processed successfully",
		zap.String("queue", queue),
		zap.Uint64("delivery_tag", msg.DeliveryTag),
	)
}

// rejectMessage rejects a message (NACK with requeue=false)
func rejectMessage(msg amqp.Delivery) {
	logger.Debug("Rejecting message",
		zap.Uint64("delivery_tag", msg.DeliveryTag),
	)
	if err := msg.Nack(false, false); err != nil {
		logger.Error("Failed to nack a message",
			zap.Uint64("delivery_tag", msg.DeliveryTag),
			zap.Error(err),
		)
		// Panic here to match Java behavior (throws RuntimeException)
		panic(fmt.Sprintf("failed to nack message: %v", err))
	}
}
