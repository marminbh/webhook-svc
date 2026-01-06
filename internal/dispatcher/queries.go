package dispatcher

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/models"
	"github.com/marminbh/webhook-svc/internal/utils"
)

// getActiveWebhookConfigs retrieves all active webhook configurations for a tenant
// that are not paused
func getActiveWebhookConfigs(db *gorm.DB, tenantID string) ([]models.WebhookConfig, error) {
	var configs []models.WebhookConfig
	now := time.Now()

	err := db.Where("active = ? AND (paused_until IS NULL OR paused_until <= ?)", true, now).
		Find(&configs).Error

	if err != nil {
		return nil, err
	}

	// Filter by tenant_id if it's stored in webhook_config
	// Note: Based on the schema, tenant_id is in webhook_events, not webhook_config
	// So we need to check if customer_id matches tenant_id or if there's a relationship
	// For now, we'll return all active configs and let the caller filter if needed
	return configs, nil
}

// createWebhookEvents creates webhook_events records for each webhook config
// Returns the created events
func createWebhookEvents(
	db *gorm.DB,
	webhookConfigs []models.WebhookConfig,
	eventType, resourceID, resourceURL, tenantID string,
	eventTimestamp time.Time,
	maxAttempts, batchSize int,
) ([]models.WebhookEvent, error) {
	if len(webhookConfigs) == 0 {
		return []models.WebhookEvent{}, nil
	}

	events := make([]models.WebhookEvent, 0, len(webhookConfigs))
	now := time.Now()

	for _, config := range webhookConfigs {
		event := models.WebhookEvent{
			ID:              uuid.New(),
			WebhookConfigID: config.ID,
			ResourceID:      resourceID,
			ResourceURL:     resourceURL,
			TenantID:        tenantID,
			EventType:       eventType,
			EventTimestamp:  eventTimestamp,
			AttemptCount:    0,
			MaxAttempts:     maxAttempts,
			Status:          "pending",
			NextAttemptAt:   now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		events = append(events, event)
	}

	// Batch insert using GORM with configurable batch size
	err := db.CreateInBatches(events, batchSize).Error
	if err != nil {
		return nil, err
	}

	return events, nil
}

// updateEventOnPublishFailure updates the next_attempt_at field when RabbitMQ publish fails
func updateEventOnPublishFailure(db *gorm.DB, eventID uuid.UUID, delayMinutes int) error {
	nextAttempt := time.Now().Add(time.Duration(delayMinutes) * time.Minute)

	err := db.Model(&models.WebhookEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"next_attempt_at": nextAttempt,
			"updated_at":      time.Now(),
		}).Error

	return err
}

// getWebhookConfigsByTenant retrieves webhook configs that match a tenant
// Converts MongoDB ObjectID (tenantID) to UUID format by prepending zeros
func getWebhookConfigsByTenant(db *gorm.DB, tenantID string) ([]models.WebhookConfig, error) {
	var configs []models.WebhookConfig
	now := time.Now()

	// Convert MongoDB ObjectID to UUID by prepending zeros
	customerUUID, err := utils.ConvertMongoIDToUUID(tenantID)
	if err != nil {
		// If conversion fails, try parsing as UUID directly (in case it's already a UUID)
		customerUUID, err = uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
	}

	// Query with the converted UUID
	err = db.Where("customer_id = ? AND active = ? AND (paused_until IS NULL OR paused_until <= ?)",
		customerUUID, true, now).Find(&configs).Error

	if err != nil {
		return nil, err
	}

	return configs, nil
}

// transactionWrapper wraps database operations in a transaction
func transactionWrapper(db *gorm.DB, fn func(*gorm.DB) error) error {
	return db.Transaction(fn)
}

// checkAndCreateWebhookEvents creates webhook events and relies on database-level
// unique constraint to prevent duplicates. If a unique constraint violation occurs,
// it treats the event as a duplicate and returns early.
// The unique constraint is on (event_type, resource_id) WHERE status IN ('pending', 'queued', 'processing')
// Uses a transaction to ensure atomicity when creating multiple events
func checkAndCreateWebhookEvents(
	db *gorm.DB,
	webhookConfigs []models.WebhookConfig,
	eventType, resourceID, resourceURL, tenantID string,
	eventTimestamp time.Time,
	maxAttempts, batchSize int,
) ([]models.WebhookEvent, bool, error) {
	if len(webhookConfigs) == 0 {
		return []models.WebhookEvent{}, false, nil
	}

	var events []models.WebhookEvent

	// Wrap in transaction to ensure atomicity when creating multiple events
	err := db.Transaction(func(tx *gorm.DB) error {
		// Create events - database unique constraint will prevent duplicates
		createdEvents, err := createWebhookEvents(
			tx,
			webhookConfigs,
			eventType,
			resourceID,
			resourceURL,
			tenantID,
			eventTimestamp,
			maxAttempts,
			batchSize,
		)
		if err != nil {
			return err
		}
		events = createdEvents
		return nil
	})

	// Check if error is due to unique constraint violation (duplicate event)
	if err != nil {
		// Check for GORM's duplicate key error or PostgreSQL unique violation
		if errors.Is(err, gorm.ErrDuplicatedKey) || isUniqueConstraintViolation(err) {
			return nil, true, nil // Duplicate detected, return success with isDuplicate=true
		}
		return nil, false, err // Other error, return it
	}

	return events, false, nil
}

// isUniqueConstraintViolation checks if the error is a PostgreSQL unique constraint violation
// PostgreSQL error code 23505 indicates a unique_violation
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains unique constraint violation indicators
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "duplicate key value")
}
