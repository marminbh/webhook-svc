package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"

	"github.com/marminbh/webhook-svc/internal/config"
	"github.com/marminbh/webhook-svc/internal/database"
	"github.com/marminbh/webhook-svc/internal/dispatcher"
	"github.com/marminbh/webhook-svc/internal/logger"
	"github.com/marminbh/webhook-svc/internal/rabbitmq"
	"github.com/marminbh/webhook-svc/internal/routes"
)

func main() {
	// Initialize logger (production mode by default, can be changed via env)
	devMode := os.Getenv("LOG_LEVEL") == "debug"
	if err := logger.Init(devMode); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Connect to PostgreSQL
	if err := database.Connect(&cfg.Database); err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("Error closing database", zap.Error(err))
		}
	}()

	// Connect to RabbitMQ
	if err := rabbitmq.Connect(&cfg.RabbitMQ); err != nil {
		logger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}
	defer rabbitmq.Close()

	// Initialize and start dispatcher
	disp := dispatcher.NewDispatcher(&cfg.Dispatcher)
	if err := disp.Start(); err != nil {
		logger.Fatal("Failed to start dispatcher", zap.Error(err))
	}
	defer func() {
		if err := disp.Stop(); err != nil {
			logger.Error("Error stopping dispatcher", zap.Error(err))
		}
	}()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Webhook Service",
		ServerHeader: "Fiber",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Setup routes
	routes.SetupRoutes(app)

	// Start server in a goroutine
	go func() {
		addr := cfg.Server.Host + ":" + cfg.Server.Port
		logger.Info("Server starting",
			zap.String("address", addr),
		)
		if err := app.Listen(addr); err != nil {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server")
	if err := app.Shutdown(); err != nil {
		logger.Error("Error during server shutdown", zap.Error(err))
	}

	// Stop dispatcher
	if err := disp.Stop(); err != nil {
		logger.Error("Error stopping dispatcher", zap.Error(err))
	}

	logger.Info("Server stopped")
}
