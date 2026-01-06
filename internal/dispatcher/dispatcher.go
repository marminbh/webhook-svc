package dispatcher

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

// Dispatcher handles consuming events and creating webhook_events
type Dispatcher struct {
	cfg         *config.DispatcherConfig
	conn        *rabbitmq.Connection
	db          *gorm.DB
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	consumerTag string
	started     bool
}

// NewDispatcher creates a new dispatcher instance with dependencies
func NewDispatcher(cfg *config.DispatcherConfig, conn *rabbitmq.Connection, db *gorm.DB, logger *zap.Logger) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		cfg:         cfg,
		conn:        conn,
		db:          db,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		consumerTag: fmt.Sprintf("webhook-dispatcher-%d", time.Now().Unix()),
	}
}

// Start initializes RabbitMQ setup and starts consuming messages
// Assumes exchanges and queues already exist - will fail if they don't
func (d *Dispatcher) Start() error {
	// Validate required configuration
	if d.cfg.SourceQueue == "" {
		return fmt.Errorf("source queue is required")
	}
	if d.cfg.DeliveryExchange == "" {
		return fmt.Errorf("delivery exchange is required")
	}
	if d.cfg.DeliveryRoutingKey == "" {
		return fmt.Errorf("delivery routing key is required")
	}
	if d.cfg.DeliveryQueue == "" {
		return fmt.Errorf("delivery queue is required")
	}

	// Set QoS for prefetch count
	if err := d.conn.SetQoS(d.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	d.logger.Info("Dispatcher initialized",
		zap.String("source_queue", d.cfg.SourceQueue),
		zap.String("delivery_queue", d.cfg.DeliveryQueue),
	)

	// Start consuming messages
	if err := d.startConsuming(); err != nil {
		return err
	}

	d.started = true
	d.logger.Info("Dispatcher started and consuming messages",
		zap.String("source_queue", d.cfg.SourceQueue),
		zap.String("consumer_tag", d.consumerTag),
	)
	return nil
}

// startConsuming starts consuming messages from the queue
func (d *Dispatcher) startConsuming() error {
	// Set QoS for prefetch count
	if err := d.conn.SetQoS(d.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming messages (assumes queue exists - will fail if it doesn't)
	messages, err := d.conn.ConsumeMessages(
		d.cfg.SourceQueue,
		d.consumerTag,
		false, // autoAck (we'll manually ACK)
		false, // exclusive
		false, // noLocal
		false, // noWait
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from queue %s (queue may not exist): %w", d.cfg.SourceQueue, err)
	}

	// Process messages in a goroutine
	go d.processMessages(messages)

	return nil
}

// Stop gracefully stops the dispatcher
func (d *Dispatcher) Stop() error {
	d.logger.Info("Stopping dispatcher",
		zap.String("consumer_tag", d.consumerTag),
	)
	d.cancel()

	// Cancel consumer
	ch := d.conn.GetChannel()
	if ch != nil {
		if err := ch.Cancel(d.consumerTag, false); err != nil {
			d.logger.Error("Failed to cancel consumer",
				zap.String("consumer_tag", d.consumerTag),
				zap.Error(err),
			)
		}
	}

	d.logger.Info("Dispatcher stopped")
	return nil
}

// processMessages processes messages from the queue
func (d *Dispatcher) processMessages(messages <-chan amqp.Delivery) {
	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("Dispatcher context cancelled, stopping message processing")
			return
		case msg, ok := <-messages:
			if !ok {
				d.logger.Warn("Message channel closed, waiting for reconnection...",
					zap.String("source_queue", d.cfg.SourceQueue),
				)
				// Channel closed - wait a bit and try to restart consuming
				// The connection will automatically reconnect, so we retry after a delay
				time.Sleep(2 * time.Second)
				if d.started {
					if err := d.startConsuming(); err != nil {
						d.logger.Error("Failed to restart consuming after channel close",
							zap.String("source_queue", d.cfg.SourceQueue),
							zap.Error(err),
						)
						// Retry again after a longer delay
						time.Sleep(5 * time.Second)
						if err := d.startConsuming(); err != nil {
							d.logger.Error("Failed to restart consuming after second attempt",
								zap.String("source_queue", d.cfg.SourceQueue),
								zap.Error(err),
							)
						}
					}
				}
				return
			}
			// Use abstract consumer pattern to process message
			consumer.ProcessMessage(d.logger, d.cfg.SourceQueue, msg, d)
		}
	}
}

// HandleEvent implements the consumer.EventHandler interface
// This method is called by the abstract consumer after base64 decoding
func (d *Dispatcher) HandleEvent(decodedMessage string) error {
	// Unmarshal source event from decoded JSON string
	var sourceEvent models.NotificationEvent
	if err := json.Unmarshal([]byte(decodedMessage), &sourceEvent); err != nil {
		d.logger.Error("Failed to unmarshal source event",
			zap.Error(err),
			zap.String("decoded_message", decodedMessage),
		)
		return fmt.Errorf("failed to unmarshal source event: %w", err)
	}

	d.logger.Info("Processing event",
		zap.String("event_type", string(sourceEvent.EventType)),
		zap.String("resource_id", sourceEvent.ResourceID),
		zap.String("org_id", sourceEvent.OrgID),
	)

	// Get active webhook configs for the org
	webhookConfigs, err := getWebhookConfigsByTenant(d.db, sourceEvent.OrgID)
	if err != nil {
		d.logger.Error("Error getting webhook configs",
			zap.String("org_id", sourceEvent.OrgID),
			zap.Error(err),
		)
		return fmt.Errorf("error getting webhook configs: %w", err)
	}

	if len(webhookConfigs) == 0 {
		d.logger.Info("No active webhook configs found for org",
			zap.String("org_id", sourceEvent.OrgID),
		)
		// Return nil to ACK - no webhooks to deliver
		return nil
	}

	// Atomically check for duplicates and create webhook events in a transaction
	// This prevents race conditions where concurrent requests could create duplicate events
	events, isDuplicate, err := checkAndCreateWebhookEvents(
		d.db,
		webhookConfigs,
		string(sourceEvent.EventType),
		sourceEvent.ResourceID,
		sourceEvent.ResourceURL,
		sourceEvent.OrgID,
		sourceEvent.Timestamp,
		d.cfg.MaxAttempts,
		d.cfg.BatchSize,
	)
	if err != nil {
		d.logger.Error("Error in atomic duplicate check and event creation",
			zap.String("event_type", string(sourceEvent.EventType)),
			zap.String("resource_id", sourceEvent.ResourceID),
			zap.String("org_id", sourceEvent.OrgID),
			zap.Error(err),
		)
		return fmt.Errorf("error in atomic duplicate check and event creation: %w", err)
	}

	if isDuplicate {
		d.logger.Info("Skipping duplicate event",
			zap.String("event_type", string(sourceEvent.EventType)),
			zap.String("resource_id", sourceEvent.ResourceID),
		)
		// Return nil to ACK - we've processed it (even though we skipped it)
		return nil
	}

	d.logger.Info("Created webhook events",
		zap.Int("event_count", len(events)),
		zap.String("event_type", string(sourceEvent.EventType)),
		zap.String("resource_id", sourceEvent.ResourceID),
	)

	// Publish each event_id to delivery queue
	failedPublishes := 0
	for _, event := range events {
		if err := d.publishToDeliveryQueue(event.ID.String()); err != nil {
			d.logger.Error("Failed to publish event to delivery queue",
				zap.String("event_id", event.ID.String()),
				zap.Error(err),
			)
			// Update next_attempt_at so CronJob picks it up
			if updateErr := updateEventOnPublishFailure(d.db, event.ID, 1); updateErr != nil {
				d.logger.Error("Failed to update event on publish failure",
					zap.String("event_id", event.ID.String()),
					zap.Error(updateErr),
				)
			}
			failedPublishes++
		}
	}

	if failedPublishes > 0 {
		d.logger.Warn("Failed to publish some events to delivery queue",
			zap.Int("failed_count", failedPublishes),
			zap.Int("total_count", len(events)),
		)
		// Still return nil to ACK since events are in DB and will be picked up by CronJob
	} else {
		d.logger.Info("Successfully published events to delivery queue",
			zap.Int("event_count", len(events)),
		)
	}

	// Return nil to ACK the message
	return nil
}

// publishToDeliveryQueue publishes an event_id to the delivery queue
func (d *Dispatcher) publishToDeliveryQueue(eventID string) error {
	deliveryMsg := models.DeliveryMessage{
		EventID: eventID,
	}

	body, err := json.Marshal(deliveryMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery message: %w", err)
	}

	err = d.conn.PublishMessage(
		d.cfg.DeliveryExchange,
		d.cfg.DeliveryRoutingKey,
		false, // mandatory
		false, // immediate
		body,
	)

	if err != nil {
		return fmt.Errorf("failed to publish to delivery queue: %w", err)
	}

	return nil
}
