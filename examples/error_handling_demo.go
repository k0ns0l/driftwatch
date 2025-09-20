// Package main demonstrates the comprehensive error handling and logging system
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/k0ns0l/driftwatch/internal/recovery"
)

func main() {
	fmt.Println("=== DriftWatch Error Handling and Logging Demo ===")

	// Initialize logger
	logConfig := logging.LoggerConfig{
		Level:  logging.LogLevelDebug,
		Format: logging.LogFormatText,
		Output: "stdout",
	}

	logger, err := logging.NewLogger(logConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logger.Close()

	// Demo 1: Error Creation and Classification
	fmt.Println("1. Error Creation and Classification")
	fmt.Println("-----------------------------------")

	demoErrorCreation(logger)

	// Demo 2: Error Recovery with Retry Logic
	fmt.Println("\n2. Error Recovery with Retry Logic")
	fmt.Println("----------------------------------")

	demoErrorRecovery(logger)

	// Demo 3: Circuit Breaker Pattern
	fmt.Println("\n3. Circuit Breaker Pattern")
	fmt.Println("--------------------------")

	demoCircuitBreaker(logger)

	// Demo 4: Structured Logging
	fmt.Println("\n4. Structured Logging")
	fmt.Println("---------------------")

	demoStructuredLogging(logger)

	fmt.Println("\n=== Demo Complete ===")
}

func demoErrorCreation(logger *logging.Logger) {
	ctx := context.Background()

	// Create different types of errors
	errors := []*errors.DriftWatchError{
		errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "request timed out").
			WithSeverity(errors.SeverityHigh).
			WithRecoverable(true).
			WithGuidance("Check network connectivity and increase timeout").
			WithContext("endpoint", "https://api.example.com").
			WithContext("timeout", "30s"),

		errors.NewError(errors.ErrorTypeConfig, "CONFIG_INVALID", "configuration file is invalid").
			WithSeverity(errors.SeverityCritical).
			WithRecoverable(false).
			WithGuidance("Run 'driftwatch config validate' to see detailed errors").
			WithContext("config_file", ".driftwatch.yaml"),

		errors.NewError(errors.ErrorTypeStorage, "STORAGE_CONNECTION", "failed to connect to database").
			WithSeverity(errors.SeverityCritical).
			WithRecoverable(false).
			WithGuidance("Check database file permissions and disk space").
			WithContext("database_path", "./driftwatch.db"),
	}

	for i, err := range errors {
		fmt.Printf("Error %d:\n", i+1)
		fmt.Printf("  Type: %s\n", err.Type)
		fmt.Printf("  Code: %s\n", err.Code)
		fmt.Printf("  Message: %s\n", err.Message)
		fmt.Printf("  Severity: %s\n", err.Severity)
		fmt.Printf("  Recoverable: %t\n", err.Recoverable)
		fmt.Printf("  Guidance: %s\n", err.Guidance)

		// Log the error
		logger.LogError(ctx, err, "Demonstrating error logging")
		fmt.Println()
	}
}

func demoErrorRecovery(logger *logging.Logger) {
	ctx := context.Background()

	// Configure recovery manager
	recoveryConfig := recovery.RecoveryConfig{
		MaxAttempts:   4,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		Strategy:      recovery.RetryStrategyExponential,
		Jitter:        true,
		JitterPercent: 0.1,
	}

	rm := recovery.NewRecoveryManager(recoveryConfig, logger)

	// Simulate an operation that fails initially but succeeds on retry
	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		fmt.Printf("Attempt %d: ", attempt)

		if attempts < 3 {
			err := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "request timed out").
				WithSeverity(errors.SeverityHigh).
				WithRecoverable(true).
				WithGuidance("Check network connectivity").
				WithContext("endpoint", "https://api.example.com")

			fmt.Printf("Failed - %s\n", err.Message)
			return err
		}

		fmt.Println("Success!")
		return nil
	}

	fmt.Println("Starting operation with retry logic...")
	start := time.Now()

	err := rm.Retry(ctx, operation, "demo_api_call")
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Operation failed after all retries: %v\n", err)
	} else {
		fmt.Printf("Operation succeeded after %d attempts in %s\n", attempts, duration)
	}
}

func demoCircuitBreaker(logger *logging.Logger) {
	ctx := context.Background()

	// Configure circuit breaker
	cbConfig := recovery.CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  2 * time.Second,
	}

	cb := recovery.NewCircuitBreaker(cbConfig, logger)

	// Simulate operations that will trigger the circuit breaker
	fmt.Println("Executing operations to trigger circuit breaker...")

	for i := 1; i <= 8; i++ {
		fmt.Printf("Operation %d: ", i)

		var operation recovery.Operation
		if i <= 4 {
			// First 4 operations fail
			operation = func(ctx context.Context, attempt int) error {
				return errors.NewError(errors.ErrorTypeNetwork, "NETWORK_ERROR", "service unavailable").
					WithSeverity(errors.SeverityHigh)
			}
		} else if i <= 6 {
			// Operations 5-6 should be blocked by circuit breaker
			operation = func(ctx context.Context, attempt int) error {
				return nil // This won't be executed
			}
		} else {
			// After recovery timeout, operations should succeed
			time.Sleep(2100 * time.Millisecond) // Wait for recovery timeout
			operation = func(ctx context.Context, attempt int) error {
				return nil
			}
		}

		err := cb.Execute(ctx, operation, "demo_service_call")
		if err != nil {
			if errors.GetErrorType(err) == errors.ErrorTypeSystem {
				fmt.Printf("Blocked by circuit breaker\n")
			} else {
				fmt.Printf("Failed - %s\n", err.Error())
			}
		} else {
			fmt.Printf("Success\n")
		}

		// Log circuit breaker metrics
		metrics := cb.GetMetrics()
		fmt.Printf("  Circuit State: %s, Failures: %d\n", metrics["state"], metrics["failures"])

		if i == 4 {
			fmt.Println("  Circuit breaker should now be OPEN")
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func demoStructuredLogging(logger *logging.Logger) {
	ctx := context.Background()

	// Basic logging
	logger.Info("Application started", "version", "1.0.0", "environment", "demo")

	// Operation logging
	start := time.Now()
	logger.LogOperation(ctx, "endpoint monitoring", "endpoint_id", "users-api", "url", "https://api.example.com/users")

	// Simulate operation
	time.Sleep(150 * time.Millisecond)

	duration := time.Since(start)
	logger.LogOperationSuccess(ctx, "endpoint monitoring", duration, "status_code", 200, "response_size", "1.2KB")

	// Metrics logging
	metrics := map[string]interface{}{
		"requests_total":        1000,
		"requests_successful":   950,
		"requests_failed":       50,
		"average_response_time": "250ms",
		"p95_response_time":     "500ms",
		"error_rate":            "5%",
	}
	logger.LogMetrics(ctx, "api_monitor", metrics)

	// Health check logging
	healthDetails := map[string]interface{}{
		"database_connections": 10,
		"memory_usage":         "256MB",
		"cpu_usage":            "15%",
		"disk_usage":           "45%",
	}
	logger.LogHealthCheck(ctx, "system", true, healthDetails)

	// Security event logging (demonstrates sensitive data redaction)
	// securityDetails := map[string]interface{}{
	// 	"username":     "demo_user",
	// 	"password":     "secret123",    // This will be redacted
	// 	"api_token":    "abc123def456", // This will be redacted
	// 	"ip_address":   "192.168.1.100",
	// 	"user_agent":   "DriftWatch/1.0.0",
	// 	"login_result": "success",
	// }

	// TODO :: v1.1.x
	// logger.LogSecurityEvent(ctx, "authentication_success", securityDetails)

	// Contextual logging
	endpointLogger := logger.WithEndpoint("users-api")
	requestLogger := endpointLogger.WithRequestID("req-demo-12345")

	requestLogger.Info("Processing API request", "method", "GET", "path", "/users/123")
	requestLogger.Info("Request completed", "status_code", 200, "response_time", "125ms")

	// Error logging with recovery context
	networkErr := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "request timed out").
		WithSeverity(errors.SeverityHigh).
		WithRecoverable(true).
		WithGuidance("Check network connectivity and retry").
		WithContext("endpoint", "https://api.example.com").
		WithContext("timeout", "30s").
		WithContext("retry_count", 2)

	logger.LogError(ctx, networkErr, "API request failed")

	// Recovery logging simulation
	logger.LogRecovery(ctx, networkErr, 2, 3, 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	logger.LogRecoverySuccess(ctx, networkErr, 3)
}
