package models

import (
	"time"

	"github.com/google/uuid"
)

type WebhookEvent struct {
	ID              uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WebhookConfigID uuid.UUID     `gorm:"type:uuid;not null" json:"webhook_config_id"`
	WebhookConfig   WebhookConfig `gorm:"foreignKey:WebhookConfigID" json:"webhook_config,omitempty"`
	ResourceID      string        `gorm:"not null" json:"resource_id"`
	ResourceURL     string        `gorm:"not null" json:"resource_url"`
	TenantID        string        `gorm:"not null" json:"tenant_id"`
	EventType       string        `gorm:"not null" json:"event_type"`
	EventTimestamp  time.Time     `gorm:"not null" json:"event_timestamp"`
	AttemptCount    int           `gorm:"not null;default:0" json:"attempt_count"`
	MaxAttempts     int           `gorm:"not null;default:8" json:"max_attempts"`
	Status          string        `gorm:"not null;default:'pending'" json:"status"`
	NextAttemptAt   time.Time     `gorm:"not null;default:now()" json:"next_attempt_at"`
	LastError       *string       `json:"last_error"`
	QueuedAt        *time.Time    `json:"queued_at"`
	CreatedAt       time.Time     `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time     `gorm:"not null;default:now()" json:"updated_at"`
}

func (WebhookEvent) TableName() string {
	return "webhook_events"
}
