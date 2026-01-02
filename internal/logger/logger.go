package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Init initializes the logger with the specified log level and returns it
// Valid levels: debug, info, warn, error, fatal, panic
// Defaults to "info" if an invalid level is provided
func Init(logLevel string) (*zap.Logger, error) {
	// Normalize log level to lowercase
	logLevel = strings.ToLower(strings.TrimSpace(logLevel))
	if logLevel == "" {
		logLevel = "info"
	}

	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		// Default to info if invalid level
		level = zapcore.InfoLevel
	}

	// Use production config (structured JSON logs)
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// Use development config for debug level (more readable console output)
	if level == zapcore.DebugLevel {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}
