package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// Init initializes the logger with the specified log level
// Valid levels: debug, info, warn, error, fatal, panic
// Defaults to "info" if an invalid level is provided
func Init(logLevel string) error {
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

	var err error
	Logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// Info logs an info message with optional fields
func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
	}
}

// Error logs an error message with optional fields
func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// Warn logs a warning message with optional fields
func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// Debug logs a debug message with optional fields
func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	}
}
