package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/marminbh/webhook-svc/internal/config"
	"github.com/marminbh/webhook-svc/internal/logger"
	"github.com/marminbh/webhook-svc/internal/models"
	"github.com/marminbh/webhook-svc/internal/rabbitmq"
)

// Dispatcher handles consuming events and creating webhook_events
type Dispatcher struct {
	cfg         *config.DispatcherConfig
	ctx         context.Context
	cancel      context.CancelFunc
	consumerTag string
}

// NewDispatcher creates a new dispatcher instance
func NewDispatcher(cfg *config.DispatcherConfig) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		cfg:         cfg,
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
	if err := rabbitmq.SetQoS(d.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	logger.Info("Dispatcher initialized",
		zap.String("source_queue", d.cfg.SourceQueue),
		zap.String("delivery_queue", d.cfg.DeliveryQueue),
	)

	// Start consuming messages (assumes queue exists - will fail if it doesn't)
	messages, err := rabbitmq.ConsumeMessages(
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

	logger.Info("Dispatcher started and consuming messages",
		zap.String("source_queue", d.cfg.SourceQueue),
		zap.String("consumer_tag", d.consumerTag),
	)
	return nil
}

// Stop gracefully stops the dispatcher
func (d *Dispatcher) Stop() error {
	logger.Info("Stopping dispatcher",
		zap.String("consumer_tag", d.consumerTag),
	)
	d.cancel()

	// Cancel consumer
	if rabbitmq.Channel != nil {
		if err := rabbitmq.Channel.Cancel(d.consumerTag, false); err != nil {
			logger.Error("Failed to cancel consumer",
				zap.String("consumer_tag", d.consumerTag),
				zap.Error(err),
			)
		}
	}

	logger.Info("Dispatcher stopped")
	return nil
}

// processMessages processes messages from the queue
func (d *Dispatcher) processMessages(messages <-chan amqp.Delivery) {
	for {
		select {
		case <-d.ctx.Done():
			logger.Info("Dispatcher context cancelled, stopping message processing")
			return
		case msg, ok := <-messages:
			if !ok {
				logger.Warn("Message channel closed")
				return
			}
			d.handleMessage(msg)
		}
	}
}

// handleMessage processes a single message
func (d *Dispatcher) handleMessage(msg amqp.Delivery) {
	// Unmarshal source event
	var sourceEvent models.SourceEvent
	if err := json.Unmarshal(msg.Body, &sourceEvent); err != nil {
		logger.Error("Failed to unmarshal source event",
			zap.Error(err),
			zap.ByteString("message_body", msg.Body),
		)
		// NACK and requeue for retry (might be transient)
		msg.Nack(false, true)
		return
	}

	logger.Info("Processing event",
		zap.String("event_type", sourceEvent.EventType),
		zap.String("resource_id", sourceEvent.ResourceID),
		zap.String("tenant_id", sourceEvent.TenantID),
	)

	// Check for duplicate events
	isDuplicate, err := checkDuplicateEvent(sourceEvent.EventType, sourceEvent.ResourceID)
	if err != nil {
		logger.Error("Error checking duplicate event",
			zap.String("event_type", sourceEvent.EventType),
			zap.String("resource_id", sourceEvent.ResourceID),
			zap.Error(err),
		)
		// NACK and requeue
		msg.Nack(false, true)
		return
	}

	if isDuplicate {
		logger.Info("Skipping duplicate event",
			zap.String("event_type", sourceEvent.EventType),
			zap.String("resource_id", sourceEvent.ResourceID),
		)
		// ACK the message since we've processed it (even though we skipped it)
		msg.Ack(false)
		return
	}

	// Get active webhook configs for the tenant
	webhookConfigs, err := getWebhookConfigsByTenant(sourceEvent.TenantID)
	if err != nil {
		logger.Error("Error getting webhook configs",
			zap.String("tenant_id", sourceEvent.TenantID),
			zap.Error(err),
		)
		// NACK and requeue
		msg.Nack(false, true)
		return
	}

	if len(webhookConfigs) == 0 {
		logger.Info("No active webhook configs found for tenant",
			zap.String("tenant_id", sourceEvent.TenantID),
		)
		// ACK the message - no webhooks to deliver
		msg.Ack(false)
		return
	}

	// Create webhook_events for each config
	events, err := createWebhookEvents(
		webhookConfigs,
		sourceEvent.EventType,
		sourceEvent.ResourceID,
		sourceEvent.ResourceURL,
		sourceEvent.TenantID,
		sourceEvent.EventTimestamp,
	)
	if err != nil {
		logger.Error("Error creating webhook events",
			zap.String("event_type", sourceEvent.EventType),
			zap.String("resource_id", sourceEvent.ResourceID),
			zap.String("tenant_id", sourceEvent.TenantID),
			zap.Error(err),
		)
		// NACK and requeue
		msg.Nack(false, true)
		return
	}

	logger.Info("Created webhook events",
		zap.Int("event_count", len(events)),
		zap.String("event_type", sourceEvent.EventType),
		zap.String("resource_id", sourceEvent.ResourceID),
	)

	// Publish each event_id to delivery queue
	failedPublishes := 0
	for _, event := range events {
		if err := d.publishToDeliveryQueue(event.ID.String()); err != nil {
			logger.Error("Failed to publish event to delivery queue",
				zap.String("event_id", event.ID.String()),
				zap.Error(err),
			)
			// Update next_attempt_at so CronJob picks it up
			if updateErr := updateEventOnPublishFailure(event.ID, 1); updateErr != nil {
				logger.Error("Failed to update event on publish failure",
					zap.String("event_id", event.ID.String()),
					zap.Error(updateErr),
				)
			}
			failedPublishes++
		}
	}

	if failedPublishes > 0 {
		logger.Warn("Failed to publish some events to delivery queue",
			zap.Int("failed_count", failedPublishes),
			zap.Int("total_count", len(events)),
		)
		// Still ACK the source message since events are in DB and will be picked up by CronJob
	} else {
		logger.Info("Successfully published events to delivery queue",
			zap.Int("event_count", len(events)),
		)
	}

	// ACK the source message
	msg.Ack(false)
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

	err = rabbitmq.PublishMessage(
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
