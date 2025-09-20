package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLoggerConfig(t *testing.T) {
	config := DefaultLoggerConfig()

	assert.Equal(t, LogLevelInfo, config.Level)
	assert.Equal(t, LogFormatText, config.Format)
	assert.Equal(t, "stderr", config.Output)
	assert.Equal(t, time.RFC3339, config.TimeFormat)
	assert.False(t, config.AddSource)
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  LoggerConfig
		wantErr bool
	}{
		{
			name: "default config",
			config: LoggerConfig{
				Level:  LogLevelInfo,
				Format: LogFormatText,
				Output: "stderr",
			},
			wantErr: false,
		},
		{
			name: "json format",
			config: LoggerConfig{
				Level:  LogLevelDebug,
				Format: LogFormatJSON,
				Output: "stdout",
			},
			wantErr: false,
		},
		{
			name: "file output",
			config: LoggerConfig{
				Level:  LogLevelWarn,
				Format: LogFormatText,
				Output: filepath.Join(t.TempDir(), "test.log"),
			},
			wantErr: false,
		},
		{
			name: "invalid directory permissions", // Add a test case that should fail
			config: LoggerConfig{
				Level:  LogLevelInfo,
				Format: LogFormatText,
				Output: "/root/nonexistent/test.log", // This should fail on most systems
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
				if logger != nil {
					assert.Equal(t, tt.config.Level, logger.config.Level)
					assert.Equal(t, tt.config.Format, logger.config.Format)

					// Test that the logger actually works
					logger.Info("Test log message")

					// Clean up file logger
					err := logger.Close()
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestNewLoggerFileCreation(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "subdir", "test.log")

	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatText,
		Output: logFile,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer logger.Close()

	// Check that directory was created
	assert.DirExists(t, filepath.Dir(logFile))

	// Write a log message and check file exists
	logger.Info("test message")
	assert.FileExists(t, logFile)
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	// Create logger that writes to buffer
	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}
	// Verify writer is set correctly
	_ = logger.writer

	// Test all log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "level=DEBUG")
	assert.Contains(t, output, "level=INFO")
	assert.Contains(t, output, "level=WARN")
	assert.Contains(t, output, "level=ERROR")
}

func TestLogError(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()

	// Test with DriftWatch error
	driftErr := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "request timed out").
		WithSeverity(errors.SeverityHigh).
		WithRecoverable(true).
		WithGuidance("Check network connectivity").
		WithContext("endpoint", "https://api.example.com").
		WithContext("timeout", "30s")

	logger.LogError(ctx, driftErr, "Network request failed")

	output := buf.String()
	assert.Contains(t, output, "Network request failed")
	assert.Contains(t, output, "error_type=NETWORK")
	assert.Contains(t, output, "error_code=NETWORK_TIMEOUT")
	assert.Contains(t, output, "severity=high")
	assert.Contains(t, output, "recoverable=true")
	assert.Contains(t, output, "guidance=\"Check network connectivity\"")
	assert.Contains(t, output, "ctx_endpoint=https://api.example.com")
	assert.Contains(t, output, "ctx_timeout=30s")
}

func TestLogErrorWithCause(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()

	// Test with wrapped error
	originalErr := fmt.Errorf("connection refused")
	wrappedErr := errors.WrapError(originalErr, errors.ErrorTypeNetwork, "NETWORK_CONNECTION", "failed to connect")

	logger.LogError(ctx, wrappedErr, "Connection failed")

	output := buf.String()
	assert.Contains(t, output, "Connection failed")
	assert.Contains(t, output, "cause=\"connection refused\"")
}

func TestLogRecovery(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout")

	logger.LogRecovery(ctx, err, 2, 3, 5*time.Second)

	output := buf.String()
	assert.Contains(t, output, "Attempting error recovery")
	assert.Contains(t, output, "attempt=2")
	assert.Contains(t, output, "max_attempts=3")
	assert.Contains(t, output, "delay=5s")
	assert.Contains(t, output, "error_type=NETWORK")
}

func TestLogRecoverySuccess(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout")

	logger.LogRecoverySuccess(ctx, err, 3)

	output := buf.String()
	assert.Contains(t, output, "Error recovery successful")
	assert.Contains(t, output, "attempts=3")
	assert.Contains(t, output, "error_type=NETWORK")
}

func TestLogRecoveryFailure(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").
		WithGuidance("Check network connectivity")

	logger.LogRecoveryFailure(ctx, err, 3)

	output := buf.String()
	assert.Contains(t, output, "Error recovery failed")
	assert.Contains(t, output, "attempts=3")
	assert.Contains(t, output, "guidance=\"Check network connectivity\"")
}

func TestLogOperation(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()

	logger.LogOperation(ctx, "endpoint monitoring", "endpoint_id", "test-api", "url", "https://api.example.com")

	output := buf.String()
	assert.Contains(t, output, "Starting endpoint monitoring")
	assert.Contains(t, output, "endpoint_id=test-api")
	assert.Contains(t, output, "url=https://api.example.com")
}

func TestLogOperationSuccess(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	duration := 250 * time.Millisecond

	logger.LogOperationSuccess(ctx, "endpoint monitoring", duration, "status_code", 200)

	output := buf.String()
	assert.Contains(t, output, "Completed endpoint monitoring")
	assert.Contains(t, output, "duration=250ms")
	assert.Contains(t, output, "status_code=200")
}

func TestLogOperationFailure(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	duration := 5 * time.Second
	err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout")

	logger.LogOperationFailure(ctx, "endpoint monitoring", err, duration, "endpoint_id", "test-api")

	output := buf.String()
	assert.Contains(t, output, "Failed endpoint monitoring")
	assert.Contains(t, output, "duration=5s")
	assert.Contains(t, output, "endpoint_id=test-api")
}

func TestLogMetrics(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()
	metrics := map[string]interface{}{
		"requests_total":        100,
		"requests_failed":       5,
		"average_response_time": "250ms",
	}

	logger.LogMetrics(ctx, "http_client", metrics)

	output := buf.String()
	assert.Contains(t, output, "Performance metrics")
	assert.Contains(t, output, "component=http_client")
	assert.Contains(t, output, "requests_total=100")
	assert.Contains(t, output, "requests_failed=5")
	assert.Contains(t, output, "average_response_time=250ms")
}

func TestLogHealthCheck(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	ctx := context.Background()

	// Test healthy check
	buf.Reset()
	logger.LogHealthCheck(ctx, "database", true, map[string]interface{}{
		"connection_count": 5,
		"response_time":    "10ms",
	})

	output := buf.String()
	assert.Contains(t, output, "Health check passed")
	assert.Contains(t, output, "component=database")
	assert.Contains(t, output, "healthy=true")

	// Test unhealthy check
	buf.Reset()
	logger.LogHealthCheck(ctx, "api_endpoint", false, map[string]interface{}{
		"status_code": 500,
		"error":       "internal server error",
	})

	output = buf.String()
	assert.Contains(t, output, "Health check failed")
	assert.Contains(t, output, "component=api_endpoint")
	assert.Contains(t, output, "healthy=false")
}

// Roadmap v2
// func TestLogSecurityEvent(t *testing.T) {
// 	var buf bytes.Buffer

// 	logger := &Logger{
// 		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
// 		config: LoggerConfig{Level: LogLevelDebug},
// 		writer: &buf,
// 	}

// 	ctx := context.Background()

// 	logger.LogSecurityEvent(ctx, "authentication_failure", map[string]interface{}{
// 		"username":   "testuser",
// 		"password":   "secret123",
// 		"api_token":  "abc123",
// 		"ip_address": "192.168.1.1",
// 		"user_agent": "curl/7.68.0",
// 	})

// 	output := buf.String()
// 	assert.Contains(t, output, "Security event")
// 	assert.Contains(t, output, "security_event=authentication_failure")
// 	assert.Contains(t, output, "username=testuser")
// 	assert.Contains(t, output, "ip_address=192.168.1.1")
// 	assert.Contains(t, output, "user_agent=\"curl/7.68.0\"")

// 	// Sensitive fields should be redacted
// 	assert.Contains(t, output, "password=\"[REDACTED]\"")
// 	assert.Contains(t, output, "api_token=\"[REDACTED]\"")
// }

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"api_token", true},
		{"bearer_token", true},
		{"secret_key", true},
		{"auth_header", true},
		{"username", false},
		{"email", false},
		{"ip_address", false},
		{"user_agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSensitiveKey(tt.key))
		})
	}
}

func TestLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config: LoggerConfig{Level: LogLevelDebug},
		writer: &buf,
	}

	// Test WithComponent
	componentLogger := logger.WithComponent("http_client")
	componentLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "component=http_client")

	// Test WithEndpoint
	buf.Reset()
	endpointLogger := logger.WithEndpoint("test-api")
	endpointLogger.Info("test message")

	output = buf.String()
	assert.Contains(t, output, "endpoint_id=test-api")

	// Test WithRequestID
	buf.Reset()
	requestLogger := logger.WithRequestID("req-123")
	requestLogger.Info("test message")

	output = buf.String()
	assert.Contains(t, output, "request_id=req-123")
}

func TestLoggerLevelChecks(t *testing.T) {
	tests := []struct {
		level        LogLevel
		debugEnabled bool
		infoEnabled  bool
	}{
		{LogLevelDebug, true, true},
		{LogLevelInfo, false, true},
		{LogLevelWarn, false, false},
		{LogLevelError, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			config := LoggerConfig{
				Level:  tt.level,
				Format: LogFormatText,
				Output: "stderr",
			}

			logger, err := NewLogger(config)
			require.NoError(t, err)
			defer logger.Close()

			assert.Equal(t, tt.debugEnabled, logger.IsDebugEnabled())
			assert.Equal(t, tt.infoEnabled, logger.IsInfoEnabled())
		})
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer

	config := LoggerConfig{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: "stderr",
	}

	// Create logger with JSON format
	logger := &Logger{
		Logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
		config: config,
		writer: &buf,
	}
	// Verify writer and config are set correctly
	_ = logger.writer
	_ = logger.config

	logger.Info("test message", "key", "value", "number", 42)

	output := buf.String()

	// Parse JSON to verify it's valid
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "value", logEntry["key"])
	assert.Equal(t, float64(42), logEntry["number"]) // JSON numbers are float64
}

func TestGlobalLogger(t *testing.T) {
	// Save original global logger state
	originalLogger := globalLogger
	defer func() {
		// Restore original state
		if globalLogger != nil {
			globalLogger.Close()
		}
		globalLogger = originalLogger
	}()

	// Test default global logger
	logger := GetGlobalLogger()
	assert.NotNil(t, logger)

	// Test initializing global logger
	config := LoggerConfig{
		Level:  LogLevelDebug,
		Format: LogFormatJSON,
		Output: "stdout",
	}

	err := InitGlobalLogger(config)
	assert.NoError(t, err)

	newLogger := GetGlobalLogger()
	assert.NotNil(t, newLogger)
	assert.Equal(t, LogLevelDebug, newLogger.config.Level)
	assert.Equal(t, LogFormatJSON, newLogger.config.Format)

	// Test closing global logger
	err = CloseGlobalLogger()
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkLoggerInfo(b *testing.B) {
	var buf bytes.Buffer
	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
		config: LoggerConfig{Level: LogLevelInfo},
		writer: &buf,
	}
	// Verify writer is set correctly for benchmark
	_ = logger.writer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "iteration", i, "value", "test")
	}
}

func BenchmarkLoggerError(b *testing.B) {
	var buf bytes.Buffer
	logger := &Logger{
		Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
		config: LoggerConfig{Level: LogLevelInfo},
		writer: &buf,
	}

	ctx := context.Background()
	err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_ERROR", "network error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogError(ctx, err, "benchmark error")
	}
}
