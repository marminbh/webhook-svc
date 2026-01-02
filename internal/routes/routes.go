package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/marminbh/webhook-svc/internal/handlers"
)

// SetupRoutes configures all application routes with dependencies
func SetupRoutes(app *fiber.App, healthHandler *handlers.HealthHandler) {
	// Health check endpoint
	app.Get("/health", healthHandler.HealthCheck)

	// API v1 routes
	api := app.Group("/api/v1")
	{
		// Example endpoint
		api.Get("/", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"message": "Webhook Service API v1",
				"status":  "running",
			})
		})
	}
}
