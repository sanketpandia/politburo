package logging

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.SugaredLogger

// Init initializes the global logger with JSON output
func Init(appEnv string) error {
	var config zap.Config

	if appEnv == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// Ensure output is JSON
	config.Encoding = "json"

	logger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	globalLogger = logger.Sugar()
	return nil
}

// GetLogger returns the global SugaredLogger for structured logging
func GetLogger() *zap.SugaredLogger {
	if globalLogger == nil {
		// Fallback logger if Init wasn't called
		logger, _ := zap.NewProduction()
		globalLogger = logger.Sugar()
	}
	return globalLogger
}

// Close flushes any buffered logs
func Close() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// LogLevel represents the log level
type LogLevel string

const (
	DebugLevel LogLevel = "DEBUG"
	InfoLevel  LogLevel = "INFO"
	WarnLevel  LogLevel = "WARN"
	ErrorLevel LogLevel = "ERROR"
)

// Fields is a convenience type for passing structured fields
type Fields map[string]interface{}

// Info logs an info message with optional fields
func Info(message string, fields ...interface{}) {
	globalLogger.Infow(message, fields...)
}

// Debug logs a debug message with optional fields
func Debug(message string, fields ...interface{}) {
	globalLogger.Debugw(message, fields...)
}

// Warn logs a warning message with optional fields
func Warn(message string, fields ...interface{}) {
	globalLogger.Warnw(message, fields...)
}

// Error logs an error message with optional fields
func Error(message string, fields ...interface{}) {
	globalLogger.Errorw(message, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(message string, fields ...interface{}) {
	globalLogger.Fatalw(message, fields...)
	os.Exit(1)
}

// WithRequest creates a logger with request context fields
func WithRequest(requestID string, serverID string, userID string, endpoint string) *zap.SugaredLogger {
	return globalLogger.With(
		"request_id", requestID,
		"server_id", serverID,
		"user_id", userID,
		"endpoint", endpoint,
	)
}
