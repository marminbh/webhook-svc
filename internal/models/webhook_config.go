package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WebhookConfig struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CustomerID     uuid.UUID      `gorm:"type:uuid;not null" json:"customer_id"`
	URL            string         `gorm:"not null" json:"url"`
	Secret         string         `json:"secret"` // secret for HMAC
	Active         bool           `gorm:"default:true" json:"active"`
	MaxConcurrency int            `gorm:"default:4" json:"max_concurrency"`
	MaxRatePerMin  int            `gorm:"default:60" json:"max_rate_per_min"`
	PausedUntil    *time.Time     `json:"paused_until"`
	CreatedAt      time.Time      `gorm:"default:now()" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"default:now()" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (WebhookConfig) TableName() string {
	return "webhook_config"
}
