package worker

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/models"
)

// EventWithConfig represents a webhook event with its associated config
type EventWithConfig struct {
	Event  models.WebhookEvent
	Config models.WebhookConfig
}

// lockAndLoadWebhookEvent locks and loads a webhook event with its config
// Returns nil if event not found or status is not 'queued'
func lockAndLoadWebhookEvent(db *gorm.DB, eventID string) (*EventWithConfig, error) {
	var event models.WebhookEvent

	// Use raw SQL with FOR UPDATE to lock the row
	err := db.Raw(`
		SELECT we.*
		FROM webhook_events we
		WHERE we.id = $1 AND we.status = 'queued'
		FOR UPDATE
	`, eventID).Scan(&event).Error

	if err != nil {
		return nil, fmt.Errorf("failed to lock and load webhook event: %w", err)
	}

	// Check if we got a result (event might not exist or not be in queued status)
	// Raw SQL doesn't return ErrRecordNotFound, so we check if ID is nil
	if event.ID == uuid.Nil {
		return nil, nil
	}

	// Load the webhook config
	var config models.WebhookConfig
	err = db.Where("id = ?", event.WebhookConfigID).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("webhook config not found for event %s", eventID)
		}
		return nil, fmt.Errorf("failed to load webhook config: %w", err)
	}

	return &EventWithConfig{
		Event:  event,
		Config: config,
	}, nil
}

// updateEventToProcessing updates the event status to 'processing'
func updateEventToProcessing(db *gorm.DB, eventID uuid.UUID) error {
	return db.Model(&models.WebhookEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status":     "processing",
			"updated_at": time.Now(),
		}).Error
}

// updateEventAfterDelivery updates the event after a delivery attempt
func updateEventAfterDelivery(
	db *gorm.DB,
	eventID uuid.UUID,
	status string,
	attemptCount int,
	nextAttemptAt time.Time,
	lastError *string,
) error {
	updates := map[string]interface{}{
		"status":          status,
		"attempt_count":   attemptCount,
		"next_attempt_at": nextAttemptAt,
		"updated_at":      time.Now(),
	}

	if lastError != nil {
		updates["last_error"] = *lastError
	}

	return db.Model(&models.WebhookEvent{}).
		Where("id = ?", eventID).
		Updates(updates).Error
}

// createDeliveryAttemptLog creates a delivery attempt log record
func createDeliveryAttemptLog(
	db *gorm.DB,
	webhookEventID uuid.UUID,
	attemptNo int,
	startedAt, finishedAt time.Time,
	httpStatus *int,
	latencyMs *int,
	responseSummary *string,
	requestPayload map[string]interface{},
	responsePayload *string,
) error {
	attemptLog := models.DeliveryAttemptLog{
		WebhookEventID:  webhookEventID,
		AttemptNo:       attemptNo,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		HTTPStatus:      httpStatus,
		LatencyMs:       latencyMs,
		ResponseSummary: responseSummary,
		RequestPayload:  requestPayload,
		ResponsePayload: responsePayload,
		CreatedAt:       time.Now(),
	}

	return db.Create(&attemptLog).Error
}

// handlePausedConfig sets the event to pending with paused_until from config
func handlePausedConfig(db *gorm.DB, eventID uuid.UUID, pausedUntil time.Time, logger *zap.Logger) error {
	logger.Info("Webhook config is paused, setting event to pending",
		zap.String("event_id", eventID.String()),
		zap.Time("paused_until", pausedUntil),
	)

	return db.Model(&models.WebhookEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status":          "pending",
			"next_attempt_at": pausedUntil,
			"updated_at":      time.Now(),
		}).Error
}
