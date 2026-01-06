package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/config"
	"github.com/marminbh/webhook-svc/internal/consumer"
	"github.com/marminbh/webhook-svc/internal/models"
	"github.com/marminbh/webhook-svc/internal/rabbitmq"
)

// Worker handles consuming delivery messages and performing webhook HTTP POST requests
type Worker struct {
	cfg         *config.WorkerConfig
	conn        *rabbitmq.Connection
	db          *gorm.DB
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	consumerTag string
	started     bool
}

// NewWorker creates a new worker instance with dependencies
func NewWorker(cfg *config.WorkerConfig, conn *rabbitmq.Connection, db *gorm.DB, logger *zap.Logger) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		cfg:         cfg,
		conn:        conn,
		db:          db,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		consumerTag: fmt.Sprintf("webhook-worker-%d", time.Now().Unix()),
	}
}

// Start initializes RabbitMQ setup and starts consuming messages
func (w *Worker) Start() error {
	// Validate required configuration
	if w.cfg.DeliveryQueue == "" {
		return fmt.Errorf("delivery queue is required")
	}

	// Set QoS for prefetch count
	if err := w.conn.SetQoS(w.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	w.logger.Info("Worker initialized",
		zap.String("delivery_queue", w.cfg.DeliveryQueue),
		zap.Int("prefetch_count", w.cfg.PrefetchCount),
	)

	// Start consuming messages
	if err := w.startConsuming(); err != nil {
		return err
	}

	w.started = true
	w.logger.Info("Worker started and consuming messages",
		zap.String("delivery_queue", w.cfg.DeliveryQueue),
		zap.String("consumer_tag", w.consumerTag),
	)
	return nil
}

// startConsuming starts consuming messages from the queue
func (w *Worker) startConsuming() error {
	// Set QoS for prefetch count
	if err := w.conn.SetQoS(w.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming messages (assumes queue exists - will fail if it doesn't)
	messages, err := w.conn.ConsumeMessages(
		w.cfg.DeliveryQueue,
		w.consumerTag,
		false, // autoAck (we'll manually ACK)
		false, // exclusive
		false, // noLocal
		false, // noWait
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from queue %s: %w", w.cfg.DeliveryQueue, err)
	}

	w.logger.Info("Consumer registered successfully",
		zap.String("queue", w.cfg.DeliveryQueue),
		zap.String("consumer_tag", w.consumerTag),
	)

	// Process messages in a goroutine
	go w.processMessages(messages)

	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() error {
	w.logger.Info("Stopping worker",
		zap.String("consumer_tag", w.consumerTag),
	)
	w.cancel()

	// Cancel consumer
	ch := w.conn.GetChannel()
	if ch != nil {
		if err := ch.Cancel(w.consumerTag, false); err != nil {
			w.logger.Error("Failed to cancel consumer",
				zap.String("consumer_tag", w.consumerTag),
				zap.Error(err),
			)
		}
	}

	w.logger.Info("Worker stopped")
	return nil
}

// processMessages processes messages from the queue
func (w *Worker) processMessages(messages <-chan amqp.Delivery) {
	w.logger.Info("Started message processing goroutine",
		zap.String("queue", w.cfg.DeliveryQueue),
		zap.String("consumer_tag", w.consumerTag),
	)

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Worker context cancelled, stopping message processing")
			return
		case msg, ok := <-messages:
			if !ok {
				w.logger.Warn("Message channel closed, attempting to restart consumer...",
					zap.String("delivery_queue", w.cfg.DeliveryQueue),
				)
				// Channel closed - need to restart consuming
				// Keep retrying until successful or context is cancelled
				for w.started {
					select {
					case <-w.ctx.Done():
						return
					default:
					}

					// Wait for connection to be ready
					time.Sleep(2 * time.Second)

					// Check if channel is healthy before retrying
					if !w.conn.IsHealthy() {
						w.logger.Debug("Connection not healthy yet, waiting...",
							zap.String("delivery_queue", w.cfg.DeliveryQueue),
						)
						continue
					}

					// Try to restart consuming
					if err := w.startConsuming(); err != nil {
						w.logger.Error("Failed to restart consuming after channel close, will retry",
							zap.String("delivery_queue", w.cfg.DeliveryQueue),
							zap.Error(err),
						)
						time.Sleep(5 * time.Second)
						continue
					}

					// Successfully restarted - exit this goroutine (new one was started)
					w.logger.Info("Successfully restarted consumer after channel close",
						zap.String("delivery_queue", w.cfg.DeliveryQueue),
					)
					return
				}
				return
			}
			// Log that we received a message (for debugging)
			w.logger.Info("Received message from queue",
				zap.String("queue", w.cfg.DeliveryQueue),
				zap.Uint64("delivery_tag", msg.DeliveryTag),
			)
			// Use abstract consumer pattern to process message
			consumer.ProcessMessage(w.logger, w.cfg.DeliveryQueue, msg, w)
		}
	}
}

// HandleEvent implements the consumer.EventHandler interface
// This method is called by the abstract consumer after base64 decoding
func (w *Worker) HandleEvent(decodedMessage string) error {
	// Unmarshal delivery message from decoded JSON string
	var deliveryMsg models.DeliveryMessage
	if err := json.Unmarshal([]byte(decodedMessage), &deliveryMsg); err != nil {
		w.logger.Error("Failed to unmarshal delivery message",
			zap.Error(err),
			zap.String("decoded_message", decodedMessage),
		)
		return fmt.Errorf("failed to unmarshal delivery message: %w", err)
	}

	w.logger.Info("Processing delivery message",
		zap.String("event_id", deliveryMsg.EventID),
	)

	// Process the delivery
	return HandleDeliveryMessage(w.db, w.cfg, w.logger, deliveryMsg.EventID)
}
