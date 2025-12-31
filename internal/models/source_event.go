package models

import (
	"encoding/json"
	"time"
)

// SourceEvent represents an incoming event from the event producer
type SourceEvent struct {
	EventType      string          `json:"event_type"`
	ResourceID     string          `json:"resource_id"`
	ResourceURL    string          `json:"resource_url"`
	TenantID       string          `json:"tenant_id"`
	EventTimestamp time.Time       `json:"event_timestamp"`
	Payload        json.RawMessage `json:"payload"`
}

// DeliveryMessage represents the message published to the delivery queue
type DeliveryMessage struct {
	EventID string `json:"event_id"`
}
