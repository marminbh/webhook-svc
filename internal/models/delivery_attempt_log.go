package models

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryAttemptLog struct {
	ID              int64                  `gorm:"primary_key;autoIncrement" json:"id"`
	WebhookEventID  uuid.UUID              `gorm:"type:uuid;not null" json:"webhook_event_id"`
	WebhookEvent    WebhookEvent           `gorm:"foreignKey:WebhookEventID" json:"webhook_event,omitempty"`
	AttemptNo       int                    `gorm:"not null" json:"attempt_no"`
	StartedAt       time.Time              `gorm:"not null" json:"started_at"`
	FinishedAt      time.Time              `gorm:"not null" json:"finished_at"`
	HTTPStatus      *int                   `gorm:"type:integer" json:"http_status"`
	LatencyMs       *int                   `gorm:"type:integer" json:"latency_ms"`
	ResponseSummary *string                `json:"response_summary"`
	RequestPayload  map[string]interface{} `gorm:"type:jsonb" json:"request_payload"`
	ResponsePayload *string                `gorm:"type:text" json:"response_payload"`
	CreatedAt       time.Time              `gorm:"default:now()" json:"created_at"`
}

func (DeliveryAttemptLog) TableName() string {
	return "delivery_attempt_log"
}
