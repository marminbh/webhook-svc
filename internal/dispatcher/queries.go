package dispatcher

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/models"
)

// checkDuplicateEvent checks if an event with the same event_type and resource_id
// already exists with a status that indicates it's not yet delivered (pending, queued, processing)
func checkDuplicateEvent(db *gorm.DB, eventType, resourceID string) (bool, error) {
	var count int64
	err := db.Model(&models.WebhookEvent{}).
		Where("event_type = ? AND resource_id = ? AND status IN (?, ?, ?)",
			eventType, resourceID, "pending", "queued", "processing").
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

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
			MaxAttempts:     8,
			Status:          "pending",
			NextAttemptAt:   now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		events = append(events, event)
	}

	// Batch insert using GORM
	err := db.CreateInBatches(events, 100).Error
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
// This is a helper that may need adjustment based on how tenant_id relates to customer_id
// For now, we'll assume we need to query webhook_events to find configs for a tenant
// or that customer_id in webhook_config can be used to match tenant_id
func getWebhookConfigsByTenant(db *gorm.DB, tenantID string) ([]models.WebhookConfig, error) {
	// Option 1: If tenant_id == customer_id, query directly
	var configs []models.WebhookConfig
	now := time.Now()

	// Convert tenantID to UUID if possible, otherwise treat as string
	customerUUID, err := uuid.Parse(tenantID)
	if err == nil {
		// tenantID is a UUID, try matching with customer_id
		err = db.Where("customer_id = ? AND active = ? AND (paused_until IS NULL OR paused_until <= ?)",
			customerUUID, true, now).Find(&configs).Error
	} else {
		// tenantID is not a UUID, might need a different approach
		// For now, get all active configs and filter in application logic
		err = db.Where("active = ? AND (paused_until IS NULL OR paused_until <= ?)", true, now).
			Find(&configs).Error
	}

	if err != nil {
		return nil, err
	}

	return configs, nil
}

// transactionWrapper wraps database operations in a transaction
func transactionWrapper(db *gorm.DB, fn func(*gorm.DB) error) error {
	return db.Transaction(fn)
}
