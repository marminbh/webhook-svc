package worker

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/config"
	"github.com/marminbh/webhook-svc/internal/models"
)

// HandleDeliveryMessage processes a delivery message for a given event_id
func HandleDeliveryMessage(
	db *gorm.DB,
	cfg *config.WorkerConfig,
	logger *zap.Logger,
	eventIDStr string,
) error {
	// Parse event ID
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		logger.Error("Invalid event_id in delivery message",
			zap.String("event_id", eventIDStr),
			zap.Error(err),
		)
		// Return nil to ACK - invalid message, skip it
		return nil
	}

	// Step 1: BEGIN transaction and lock event row
	var eventWithConfig *EventWithConfig
	err = db.Transaction(func(tx *gorm.DB) error {
		eventWithConfig, err = lockAndLoadWebhookEvent(tx, eventIDStr)
		if err != nil {
			return fmt.Errorf("failed to lock and load webhook event: %w", err)
		}
		return nil
	})

	if err != nil {
		logger.Error("Failed to lock webhook event",
			zap.String("event_id", eventIDStr),
			zap.Error(err),
		)
		// Return error to NACK and retry
		return err
	}

	// If event not found or not in queued status, skip
	if eventWithConfig == nil {
		logger.Info("Event not found or not in queued status, skipping",
			zap.String("event_id", eventIDStr),
		)
		// Return nil to ACK - event already processed or doesn't exist
		return nil
	}

	event := eventWithConfig.Event
	config := eventWithConfig.Config

	// Step 2: Check if webhook config is active and not paused
	now := time.Now()
	if !config.Active {
		logger.Info("Webhook config is not active, setting event to pending",
			zap.String("event_id", eventIDStr),
			zap.String("webhook_config_id", config.ID.String()),
		)
		// Set event to pending - it will be retried later when config becomes active
		err = db.Model(&models.WebhookEvent{}).
			Where("id = ?", eventID).
			Updates(map[string]interface{}{
				"status":          "pending",
				"next_attempt_at": now.Add(1 * time.Hour), // Retry in 1 hour
				"updated_at":      now,
			}).Error
		if err != nil {
			logger.Error("Failed to update event for inactive config",
				zap.String("event_id", eventIDStr),
				zap.Error(err),
			)
			return err
		}
		// Return nil to ACK - we've handled it
		return nil
	}

	if config.PausedUntil != nil && config.PausedUntil.After(now) {
		// Config is paused, set event to pending with paused_until
		err = handlePausedConfig(db, eventID, *config.PausedUntil, logger)
		if err != nil {
			logger.Error("Failed to handle paused config",
				zap.String("event_id", eventIDStr),
				zap.Error(err),
			)
			return err
		}
		// Return nil to ACK - we've handled it
		return nil
	}

	// Step 3: Update event status to 'processing' and commit (release lock)
	err = db.Transaction(func(tx *gorm.DB) error {
		return updateEventToProcessing(tx, eventID)
	})
	if err != nil {
		logger.Error("Failed to update event to processing",
			zap.String("event_id", eventIDStr),
			zap.Error(err),
		)
		return err
	}

	// Step 4: Perform HTTP call (outside transaction)
	// Build payload from event data
	payload := map[string]interface{}{
		"event_type":       event.EventType,
		"resource_id":      event.ResourceID,
		"resource_url":     event.ResourceURL,
		"tenant_id":        event.TenantID,
		"event_timestamp":  event.EventTimestamp,
		"webhook_event_id": event.ID.String(),
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal webhook payload",
			zap.String("event_id", eventIDStr),
			zap.Error(err),
		)
		// Return error to NACK and retry
		return err
	}

	// Record start time for attempt log
	attemptStartedAt := time.Now()

	// Perform HTTP POST
	result := DeliverWebhook(
		config.URL,
		payload,
		config.Secret,
		cfg.HTTPTimeout,
		cfg.MaxResponseBodySize,
		logger,
	)

	// Record finish time
	attemptFinishedAt := time.Now()

	// Step 5: Process delivery result
	newAttemptCount := event.AttemptCount + 1
	deliveryStatus := ProcessDeliveryResult(result, newAttemptCount, event.MaxAttempts)

	// Step 6: Start new transaction, insert attempt log, update event status
	err = db.Transaction(func(tx *gorm.DB) error {
		// Create attempt log
		err := createDeliveryAttemptLog(
			tx,
			eventID,
			newAttemptCount,
			attemptStartedAt,
			attemptFinishedAt,
			result.HTTPStatus,
			&result.LatencyMs,
			result.ResponseSummary,
			jsonPayload,
			&result.ResponseBody,
		)
		if err != nil {
			return fmt.Errorf("failed to create delivery attempt log: %w", err)
		}

		// Update event status
		err = updateEventAfterDelivery(
			tx,
			eventID,
			deliveryStatus.Status,
			newAttemptCount,
			deliveryStatus.NextAttemptAt,
			deliveryStatus.LastError,
		)
		if err != nil {
			return fmt.Errorf("failed to update event after delivery: %w", err)
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to update event and create attempt log",
			zap.String("event_id", eventIDStr),
			zap.Error(err),
		)
		return err
	}

	// Log the result
	if deliveryStatus.Status == "succeeded" {
		logger.Info("Webhook delivery succeeded",
			zap.String("event_id", eventIDStr),
			zap.Int("attempt_count", newAttemptCount),
			zap.Int("http_status", *result.HTTPStatus),
			zap.Int("latency_ms", result.LatencyMs),
		)
	} else if deliveryStatus.Status == "failed" {
		logger.Warn("Webhook delivery failed (max attempts reached)",
			zap.String("event_id", eventIDStr),
			zap.Int("attempt_count", newAttemptCount),
			zap.String("last_error", *deliveryStatus.LastError),
		)
	} else {
		logger.Info("Webhook delivery will be retried",
			zap.String("event_id", eventIDStr),
			zap.Int("attempt_count", newAttemptCount),
			zap.Time("next_attempt_at", deliveryStatus.NextAttemptAt),
			zap.String("last_error", *deliveryStatus.LastError),
		)
	}

	// Return nil to ACK the message
	return nil
}
