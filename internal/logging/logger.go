// Package logging provides structured logging functionality for DriftWatch
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat represents the log output format
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Level      LogLevel  `yaml:"level" mapstructure:"level"`
	Format     LogFormat `yaml:"format" mapstructure:"format"`
	Output     string    `yaml:"output" mapstructure:"output"` // "stdout", "stderr", or file path
	TimeFormat string    `yaml:"time_format" mapstructure:"time_format"`
	AddSource  bool      `yaml:"add_source" mapstructure:"add_source"`
}

// DefaultLoggerConfig returns a default logger configuration
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:      LogLevelInfo,
		Format:     LogFormatText,
		Output:     "stderr",
		TimeFormat: time.RFC3339,
		AddSource:  false,
	}
}

// Logger struct - add a file field to track open files
type Logger struct {
	*slog.Logger
	writer io.Writer
	file   *os.File // Add this field to track open files
	config LoggerConfig
}

func NewLogger(config LoggerConfig) (*Logger, error) {
	// Determine output writer
	var writer io.Writer
	var file *os.File // Track the file handle

	switch config.Output {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "":
		writer = os.Stderr
	default:
		// File output
		dir := filepath.Dir(config.Output)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		f, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writer = f
		file = f // Store the file handle
	}

	// Convert log level
	var level slog.Level
	switch config.Level {
	case LogLevelDebug:
		level = slog.LevelDebug
	case LogLevelInfo:
		level = slog.LevelInfo
	case LogLevelWarn:
		level = slog.LevelWarn
	case LogLevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// Create handler based on format
	var handler slog.Handler
	switch config.Format {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(writer, opts)
	case LogFormatText:
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		config: config,
		writer: writer,
		file:   file, // Store the file handle
	}, nil
}

// Add Close method to properly close file handles
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogError logs a DriftWatch error with appropriate context
func (l *Logger) LogError(ctx context.Context, err error, msg string, args ...interface{}) {
	if dwe, ok := err.(*errors.DriftWatchError); ok {
		attrs := []slog.Attr{
			slog.String("error_type", string(dwe.Type)),
			slog.String("error_code", dwe.Code),
			slog.String("severity", string(dwe.Severity)),
			slog.Bool("recoverable", dwe.Recoverable),
		}

		if dwe.Guidance != "" {
			attrs = append(attrs, slog.String("guidance", dwe.Guidance))
		}

		if len(dwe.Context) > 0 {
			for key, value := range dwe.Context {
				attrs = append(attrs, slog.Any(fmt.Sprintf("ctx_%s", key), value))
			}
		}

		if dwe.Cause != nil {
			attrs = append(attrs, slog.String("cause", dwe.Cause.Error()))
		}

		l.LogAttrs(ctx, slog.LevelError, msg, attrs...)
	} else {
		l.Error(msg, "error", err)
	}
}

// LogRecovery logs error recovery attempts
func (l *Logger) LogRecovery(ctx context.Context, err error, attempt int, maxAttempts int, delay time.Duration) {
	l.Warn("Attempting error recovery",
		"error", err.Error(),
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"delay", delay,
		"error_type", errors.GetErrorType(err),
		"recoverable", errors.IsRecoverable(err))
}

// LogRecoverySuccess logs successful error recovery
func (l *Logger) LogRecoverySuccess(ctx context.Context, err error, attempts int) {
	l.Info("Error recovery successful",
		"original_error", err.Error(),
		"attempts", attempts,
		"error_type", errors.GetErrorType(err))
}

// LogRecoveryFailure logs failed error recovery
func (l *Logger) LogRecoveryFailure(ctx context.Context, err error, attempts int) {
	l.Error("Error recovery failed",
		"error", err.Error(),
		"attempts", attempts,
		"error_type", errors.GetErrorType(err),
		"guidance", errors.GetGuidance(err))
}

// LogOperation logs the start of an operation
func (l *Logger) LogOperation(ctx context.Context, operation string, args ...interface{}) {
	l.Info(fmt.Sprintf("Starting %s", operation), args...)
}

// LogOperationSuccess logs successful completion of an operation
func (l *Logger) LogOperationSuccess(ctx context.Context, operation string, duration time.Duration, args ...interface{}) {
	allArgs := append([]interface{}{"duration", duration}, args...)
	l.Info(fmt.Sprintf("Completed %s", operation), allArgs...)
}

// LogOperationFailure logs failed completion of an operation
func (l *Logger) LogOperationFailure(ctx context.Context, operation string, err error, duration time.Duration, args ...interface{}) {
	allArgs := append([]interface{}{"duration", duration, "error", err}, args...)
	l.Error(fmt.Sprintf("Failed %s", operation), allArgs...)
}

// LogMetrics logs performance metrics
func (l *Logger) LogMetrics(ctx context.Context, component string, metrics map[string]interface{}) {
	args := []interface{}{"component", component}
	for key, value := range metrics {
		args = append(args, key, value)
	}
	l.Info("Performance metrics", args...)
}

// LogHealthCheck logs health check results
func (l *Logger) LogHealthCheck(ctx context.Context, component string, healthy bool, details map[string]interface{}) {
	args := []interface{}{"component", component, "healthy", healthy}
	for key, value := range details {
		args = append(args, key, value)
	}

	if healthy {
		l.Info("Health check passed", args...)
	} else {
		l.Warn("Health check failed", args...)
	}
}

// LogConfigChange logs configuration changes
func (l *Logger) LogConfigChange(ctx context.Context, component string, changes map[string]interface{}) {
	args := []interface{}{"component", component}
	for key, value := range changes {
		args = append(args, key, value)
	}
	l.Info("Configuration changed", args...)
}

// LogSecurityEvent logs security-related events :: v1.1.x
// func (l *Logger) LogSecurityEvent(ctx context.Context, event string, details map[string]interface{}) {
// 	args := []interface{}{"security_event", event}
// 	for key, value := range details {
// 		if isSensitiveKey(key) {
// 			args = append(args, key, "[REDACTED]")
// 		} else {
// 			args = append(args, key, value)
// 		}
// 	}
// 	l.Warn("Security event", args...)
// }

// isSensitiveKey checks if a key contains sensitive information
func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"bearer", "api_key", "access_token", "refresh_token",
	}

	lowerKey := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// WithContext returns a logger with additional context
func (l *Logger) WithContext(ctx context.Context, args ...interface{}) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
		config: l.config,
		writer: l.writer,
	}
}

// WithComponent returns a logger with component context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
		config: l.config,
		writer: l.writer,
	}
}

// WithEndpoint returns a logger with endpoint context
func (l *Logger) WithEndpoint(endpointID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("endpoint_id", endpointID),
		config: l.config,
		writer: l.writer,
	}
}

// WithRequestID returns a logger with request ID context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("request_id", requestID),
		config: l.config,
		writer: l.writer,
	}
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.config.Level
}

// SetLevel updates the log level (note: this creates a new logger)
func (l *Logger) SetLevel(level LogLevel) (*Logger, error) {
	newConfig := l.config
	newConfig.Level = level
	return NewLogger(newConfig)
}

// IsDebugEnabled returns true if debug logging is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.config.Level == LogLevelDebug
}

// IsInfoEnabled returns true if info logging is enabled
func (l *Logger) IsInfoEnabled() bool {
	return l.config.Level == LogLevelDebug || l.config.Level == LogLevelInfo
}

// Global logger instance
var globalLogger *Logger

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(config LoggerConfig) error {
	logger, err := NewLogger(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		// Create default logger if none exists
		config := DefaultLoggerConfig()
		logger, err := NewLogger(config)
		if err != nil {
			// Fallback to a basic logger if we can't create the configured one
			panic(fmt.Sprintf("Failed to create default logger: %v", err))
		}
		globalLogger = logger
	}
	return globalLogger
}

// CloseGlobalLogger closes the global logger
func CloseGlobalLogger() error {
	if globalLogger != nil {
		err := globalLogger.Close()
		globalLogger = nil // Reset the global logger
		return err
	}
	return nil
}
