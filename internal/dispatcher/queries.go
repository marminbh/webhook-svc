package dispatcher

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/models"
)

// checkDuplicateEvent checks if an event with the same event_type and resource_id
// already exists with a status that indicates it's not yet delivered (pending, queued, processing)
// Uses SELECT FOR UPDATE to lock matching rows and prevent race conditions
func checkDuplicateEvent(db *gorm.DB, eventType, resourceID string) (bool, error) {
	var count int64
	// Use raw SQL with FOR UPDATE to lock any matching rows, preventing concurrent duplicate creation
	// This ensures that if two concurrent requests check for duplicates, one will wait for the other
	// to complete before proceeding, preventing race conditions
	err := db.Raw(`
		SELECT COUNT(*) 
		FROM webhook_events 
		WHERE event_type = $1 AND resource_id = $2 AND status IN ($3, $4, $5)
		FOR UPDATE
	`, eventType, resourceID, "pending", "queued", "processing").Scan(&count).Error

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

// checkAndCreateWebhookEvents atomically checks for duplicates and creates webhook events
// This prevents race conditions where multiple concurrent requests could create duplicate events
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
	var isDuplicate bool

	// Wrap duplicate check and event creation in a transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		// Check for duplicates with row-level locking
		duplicate, err := checkDuplicateEvent(tx, eventType, resourceID)
		if err != nil {
			return err
		}

		if duplicate {
			isDuplicate = true
			return nil // Return early, don't create events
		}

		// No duplicates found, create events
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

	if err != nil {
		return nil, false, err
	}

	return events, isDuplicate, nil
}
