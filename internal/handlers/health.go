package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/marminbh/webhook-svc/internal/database"
	"github.com/marminbh/webhook-svc/internal/rabbitmq"
)

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// HealthCheck handles the health check endpoint
func HealthCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := make(map[string]string)
	status := "healthy"

	// Check database
	if err := database.HealthCheck(ctx); err != nil {
		services["database"] = "unhealthy: " + err.Error()
		status = "unhealthy"
	} else {
		services["database"] = "healthy"
	}

	// Check RabbitMQ
	if rabbitmq.Conn == nil || rabbitmq.Conn.IsClosed() {
		services["rabbitmq"] = "unhealthy: connection closed"
		status = "unhealthy"
	} else {
		services["rabbitmq"] = "healthy"
	}

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  services,
	}

	if status == "unhealthy" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(response)
	}

	return c.JSON(response)
}
