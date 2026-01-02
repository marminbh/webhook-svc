package service

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/marminbh/webhook-svc/internal/rabbitmq"
)

// Service holds all application dependencies
// This eliminates global state and enables proper dependency injection
type Service struct {
	DB     *gorm.DB
	Logger *zap.Logger
	RMQ    *rabbitmq.Connection
}

// NewService creates a new service instance with all dependencies
func NewService(db *gorm.DB, logger *zap.Logger, rmq *rabbitmq.Connection) *Service {
	return &Service{
		DB:     db,
		Logger: logger,
		RMQ:    rmq,
	}
}
