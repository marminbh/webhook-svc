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
	"github.com/marminbh/webhook-svc/internal/handlers"
	"github.com/marminbh/webhook-svc/internal/logger"
	"github.com/marminbh/webhook-svc/internal/rabbitmq"
	"github.com/marminbh/webhook-svc/internal/routes"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// Initialize logger with log level from config
	appLogger, err := logger.Init(cfg.Server.LogLevel)
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			// Ignore sync errors on close
		}
	}()

	// Run database migrations
	if err := database.RunMigrations(&cfg.Database, appLogger); err != nil {
		appLogger.Fatal("Failed to run database migrations", zap.Error(err))
	}

	// Connect to PostgreSQL
	db, err := database.Connect(&cfg.Database, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(db, appLogger); err != nil {
			appLogger.Error("Error closing database", zap.Error(err))
		}
	}()

	// Connect to RabbitMQ
	rmqConn := rabbitmq.NewConnection(&cfg.RabbitMQ, appLogger)
	if err := rmqConn.Connect(); err != nil {
		appLogger.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}
	defer rmqConn.Close()

	// Create health handler with dependencies
	healthHandler := handlers.NewHealthHandler(db, rmqConn)

	// Initialize and start dispatcher with dependencies
	disp := dispatcher.NewDispatcher(&cfg.Dispatcher, rmqConn, db, appLogger)
	if err := disp.Start(); err != nil {
		appLogger.Fatal("Failed to start dispatcher", zap.Error(err))
	}
	defer func() {
		if err := disp.Stop(); err != nil {
			appLogger.Error("Error stopping dispatcher", zap.Error(err))
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

	// Setup routes with dependencies
	routes.SetupRoutes(app, healthHandler)

	// Start server in a goroutine
	go func() {
		addr := cfg.Server.Host + ":" + cfg.Server.Port
		appLogger.Info("Server starting",
			zap.String("address", addr),
		)
		if err := app.Listen(addr); err != nil {
			appLogger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server")
	if err := app.Shutdown(); err != nil {
		appLogger.Error("Error during server shutdown", zap.Error(err))
	}

	// Stop dispatcher
	if err := disp.Stop(); err != nil {
		appLogger.Error("Error stopping dispatcher", zap.Error(err))
	}

	appLogger.Info("Server stopped")
}
