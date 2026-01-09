package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/models"
	"github.com/marminbh/webhook-svc/internal/utils"
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
	Events  []models.WebhookEvent `json:"events"`
	HasMore bool                  `json:"has_more"`
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

	// Convert MongoDB ObjectID to UUID by prepending zeros
	customerUUID, err := utils.ConvertMongoIDToUUID(orgID)
	if err != nil {
		// If conversion fails, try parsing as UUID directly (in case it's already a UUID)
		customerUUID, err = uuid.Parse(orgID)
		if err != nil {
			h.Logger.Error("Invalid org_id format",
				zap.String("org_id", orgID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "org_id must be a valid MongoDB ObjectID or UUID",
			})
		}
	}

	// Query webhook events with join to webhook_config
	var events []models.WebhookEvent
	query := h.DB.Model(&models.WebhookEvent{}).
		Joins("JOIN webhook_config ON webhook_events.webhook_config_id = webhook_config.id").
		Where("webhook_config.customer_id = ?", customerUUID).
		Order("webhook_events.created_at DESC").
		Limit(limit + 1). // Fetch one extra to determine has_more
		Offset(offset)

	if err := query.Find(&events).Error; err != nil {
		h.Logger.Error("Failed to query webhook events",
			zap.String("org_id", orgID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch events",
		})
	}

	// Determine if there are more events and handle trimming
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit] // Remove the extra event
	}

	response := EventsResponse{
		Events:  events,
		HasMore: hasMore,
	}

	return c.JSON(response)
}
