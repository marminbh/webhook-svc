package models

import (
	"encoding/json"
	"time"
)

// Timestamp is a custom type that can unmarshal both JSON strings (RFC3339) and numbers (Unix milliseconds)
type Timestamp struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler to handle both string and numeric timestamps
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a number first (Unix timestamp in milliseconds)
	var timestampMs int64
	if err := json.Unmarshal(data, &timestampMs); err == nil {
		t.Time = time.Unix(0, timestampMs*int64(time.Millisecond))
		return nil
	}

	// If both fail, return the original error
	return json.Unmarshal(data, &t.Time)
}

// MarshalJSON implements json.Marshaler to ensure consistent output format
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format(time.RFC3339))
}

// NotificationEvent represents an incoming event from the api service
type NotificationEvent struct {
	EventType   NotificationEventType `json:"event_type"`
	ResourceID  string                `json:"resource_id"`
	ResourceURL string                `json:"resource_url"`
	OrgID       string                `json:"org_id"`
	Timestamp   Timestamp             `json:"timestamp"`
}

// DeliveryMessage represents the message published to the delivery queue
type DeliveryMessage struct {
	EventID string `json:"event_id"`
}
