package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// EventsHandler handles webhook events listing
type EventsHandler struct {
	DB     *gorm.DB
	Logger *zap.Logger
}

// NewEventsHandler creates a new events handler with dependencies
func NewEventsHandler(db *gorm.DB, logger *zap.Logger) *EventsHandler {
	return &EventsHandler{
		DB:     db,
		Logger: logger,
	}
}

// EventsResponse represents the response structure for GET /events
type EventsResponse struct {
	Events  []EventDTO `json:"events"`
	HasMore bool       `json:"has_more"`
}

// EventDTO represents a single webhook event in the response
type EventDTO struct {
	ID         string `json:"id"`
	Status     string `json:"status"`      // Display status (e.g., "200", "500", "pending")
	StatusCode *int   `json:"status_code"` // HTTP status code if available
	EventName  string `json:"event_name"`  // event_type
	Timestamp  string `json:"timestamp"`   // UTC ISO 8601 format
}

// GetEvents handles GET /events endpoint
// Query parameters:
//   - org_id (required): Organization ID (MongoDB ObjectID)
//   - limit (optional, default 25): Number of events to return
//   - offset (optional, default 0): Number of events to skip
//
// Note: org_id validation will be handled via header validation in the future
func (h *EventsHandler) GetEvents(c *fiber.Ctx) error {
	// Parse org_id from query parameter
	orgID := c.Query("org_id")
	if orgID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "org_id query parameter is required",
		})
	}

	// Parse pagination parameters
	limit := 25 // default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "limit must be a positive integer",
			})
		}
		limit = parsedLimit
	}

	offset := 0 // default offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "offset must be a non-negative integer",
			})
		}
		offset = parsedOffset
	}

	// Query events using GORM query builder
	type eventRow struct {
		ID         uuid.UUID
		EventType  string
		Status     string
		CreatedAt  time.Time
		HTTPStatus *int
	}

	var events []eventRow

	// First, query webhook events with join to webhook_config
	query := h.DB.Table("webhook_events").
		Select("webhook_events.id, webhook_events.event_type, webhook_events.status, webhook_events.created_at").
		Joins("JOIN webhook_config ON webhook_events.webhook_config_id = webhook_config.id").
		Where("webhook_config.customer_id = ?", orgID).
		Order("webhook_events.created_at DESC").
		Limit(limit + 1). // Fetch one extra to determine has_more
		Offset(offset)

	if err := query.Scan(&events).Error; err != nil {
		h.Logger.Error("Failed to query webhook events",
			zap.String("org_id", orgID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch events",
		})
	}

	// If no events found, return empty response
	if len(events) == 0 {
		response := EventsResponse{
			Events:  []EventDTO{},
			HasMore: false,
		}
		return c.JSON(response)
	}

	// Determine if there are more events before fetching attempt logs
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit] // Remove the extra event
	}

	// Collect event IDs to fetch latest attempt logs
	eventIDs := make([]uuid.UUID, len(events))
	for i, event := range events {
		eventIDs[i] = event.ID
	}

	// Fetch latest attempt log for each event
	// Use a subquery to get the max attempt_no per event, then join to get http_status
	type attemptLogRow struct {
		WebhookEventID uuid.UUID
		HTTPStatus     *int
	}

	var attemptLogs []attemptLogRow

	// Get the latest attempt log for each event using a subquery
	// This finds the max attempt_no per event, then gets the corresponding http_status
	subquery := h.DB.Table("delivery_attempt_log").
		Select("webhook_event_id, MAX(attempt_no) as max_attempt").
		Where("webhook_event_id IN ?", eventIDs).
		Group("webhook_event_id")

	if err := h.DB.Table("delivery_attempt_log AS dal").
		Select("dal.webhook_event_id, dal.http_status").
		Joins("INNER JOIN (?) AS latest ON dal.webhook_event_id = latest.webhook_event_id AND dal.attempt_no = latest.max_attempt", subquery).
		Scan(&attemptLogs).Error; err != nil {
		// Log error but continue - we can still return events without HTTP status
		h.Logger.Warn("Failed to fetch attempt logs, continuing without HTTP status",
			zap.Error(err),
		)
	}

	// Create a map of event ID to HTTP status for quick lookup
	httpStatusMap := make(map[uuid.UUID]*int)
	for _, log := range attemptLogs {
		httpStatusMap[log.WebhookEventID] = log.HTTPStatus
	}

	// Update events with HTTP status
	for i := range events {
		if status, ok := httpStatusMap[events[i].ID]; ok {
			events[i].HTTPStatus = status
		}
	}

	// Convert to DTOs
	eventDTOs := make([]EventDTO, 0, len(events))
	for _, event := range events {
		displayStatus, statusCode := mapEventStatus(event.Status, event.HTTPStatus)

		eventDTOs = append(eventDTOs, EventDTO{
			ID:         event.ID.String(),
			Status:     displayStatus,
			StatusCode: statusCode,
			EventName:  event.EventType,
			Timestamp:  event.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	response := EventsResponse{
		Events:  eventDTOs,
		HasMore: hasMore,
	}

	return c.JSON(response)
}

// mapEventStatus maps internal status and HTTP status code to display status
// Returns: (displayStatus string, statusCode *int)
// Rules:
//   - Green badge (Success): HTTP status 2xx OR status='succeeded'
//   - Red badge (Error): HTTP status 4xx/5xx OR status='failed' OR status='pending' with no attempts
//   - Display: Show HTTP status code if available, otherwise show internal status
func mapEventStatus(eventStatus string, httpStatus *int) (string, *int) {
	// If we have an HTTP status code, use it
	if httpStatus != nil {
		return strconv.Itoa(*httpStatus), httpStatus
	}

	// Otherwise, use the internal status
	// For pending/queued/processing, show the status as-is
	// For succeeded/failed, show the status
	return eventStatus, nil
}
