package models

import (
	"time"
)

// NotificationEvent represents an incoming event from the api service
type NotificationEvent struct {
	EventType   NotificationEventType `json:"event_type"`
	ResourceID  string                `json:"resource_id"`
	ResourceURL string                `json:"resource_url"`
	OrgID       string                `json:"org_id"`
	Timestamp   time.Time             `json:"timestamp"`
}

// DeliveryMessage represents the message published to the delivery queue
type DeliveryMessage struct {
	EventID string `json:"event_id"`
}
