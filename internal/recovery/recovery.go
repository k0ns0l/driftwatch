// Package recovery provides error recovery strategies for transient failures
package recovery

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
)

// RetryStrategy defines the strategy for retrying failed operations
type RetryStrategy string

const (
	RetryStrategyFixed       RetryStrategy = "fixed"
	RetryStrategyExponential RetryStrategy = "exponential"
	RetryStrategyLinear      RetryStrategy = "linear"
)

// RecoveryConfig holds configuration for error recovery
type RecoveryConfig struct {
	MaxAttempts   int           `yaml:"max_attempts" mapstructure:"max_attempts"`
	InitialDelay  time.Duration `yaml:"initial_delay" mapstructure:"initial_delay"`
	MaxDelay      time.Duration `yaml:"max_delay" mapstructure:"max_delay"`
	Strategy      RetryStrategy `yaml:"strategy" mapstructure:"strategy"`
	Jitter        bool          `yaml:"jitter" mapstructure:"jitter"`
	JitterPercent float64       `yaml:"jitter_percent" mapstructure:"jitter_percent"`
}

// DefaultRecoveryConfig returns a default recovery configuration
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		Strategy:      RetryStrategyExponential,
		Jitter:        true,
		JitterPercent: 0.1, // 10% jitter
	}
}

// RecoveryManager handles error recovery operations
type RecoveryManager struct {
	config RecoveryConfig
	logger *logging.Logger
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(config RecoveryConfig, logger *logging.Logger) *RecoveryManager {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}

	return &RecoveryManager{
		config: config,
		logger: logger.WithComponent("recovery"),
	}
}

// Operation represents a function that can be retried
type Operation func(ctx context.Context, attempt int) error

// OperationWithResult represents a function that returns a result and can be retried
type OperationWithResult func(ctx context.Context, attempt int) (interface{}, error)

// Retry executes an operation with retry logic
func (rm *RecoveryManager) Retry(ctx context.Context, operation Operation, operationName string) error {
	var lastErr error

	for attempt := 1; attempt <= rm.config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rm.logger.Debug("Attempting operation",
			"operation", operationName,
			"attempt", attempt,
			"max_attempts", rm.config.MaxAttempts)

		err := operation(ctx, attempt)
		if err == nil {
			if attempt > 1 {
				rm.logger.LogRecoverySuccess(ctx, lastErr, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if error is recoverable
		if !rm.isRecoverable(err) {
			rm.logger.LogError(ctx, err, "Operation failed with non-recoverable error",
				"operation", operationName,
				"attempt", attempt)
			return err
		}

		// Don't sleep after the last attempt
		if attempt < rm.config.MaxAttempts {
			delay := rm.calculateDelay(attempt - 1)
			rm.logger.LogRecovery(ctx, err, attempt, rm.config.MaxAttempts, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	rm.logger.LogRecoveryFailure(ctx, lastErr, rm.config.MaxAttempts)
	return fmt.Errorf("operation failed after %d attempts: %w", rm.config.MaxAttempts, lastErr)
}

// RetryWithResult executes an operation that returns a result with retry logic
func (rm *RecoveryManager) RetryWithResult(ctx context.Context, operation OperationWithResult, operationName string) (interface{}, error) {
	var lastErr error

	for attempt := 1; attempt <= rm.config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		rm.logger.Debug("Attempting operation with result",
			"operation", operationName,
			"attempt", attempt,
			"max_attempts", rm.config.MaxAttempts)

		result, err := operation(ctx, attempt)
		if err == nil {
			if attempt > 1 {
				rm.logger.LogRecoverySuccess(ctx, lastErr, attempt)
			}
			return result, nil
		}

		lastErr = err

		// Check if error is recoverable
		if !rm.isRecoverable(err) {
			rm.logger.LogError(ctx, err, "Operation failed with non-recoverable error",
				"operation", operationName,
				"attempt", attempt)
			return nil, err
		}

		// Don't sleep after the last attempt
		if attempt < rm.config.MaxAttempts {
			delay := rm.calculateDelay(attempt - 1)
			rm.logger.LogRecovery(ctx, err, attempt, rm.config.MaxAttempts, delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	rm.logger.LogRecoveryFailure(ctx, lastErr, rm.config.MaxAttempts)
	return nil, fmt.Errorf("operation failed after %d attempts: %w", rm.config.MaxAttempts, lastErr)
}

// RetryIf executes an operation with retry logic only if the condition is met
func (rm *RecoveryManager) RetryIf(ctx context.Context, operation Operation, condition func(error) bool, operationName string) error {
	var lastErr error

	for attempt := 1; attempt <= rm.config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation(ctx, attempt)
		if err == nil {
			if attempt > 1 {
				rm.logger.LogRecoverySuccess(ctx, lastErr, attempt)
			}
			return nil
		}

		lastErr = err

		// Check custom condition and recoverability
		if !condition(err) || !rm.isRecoverable(err) {
			rm.logger.LogError(ctx, err, "Operation failed with non-retryable error",
				"operation", operationName,
				"attempt", attempt)
			return err
		}

		if attempt < rm.config.MaxAttempts {
			delay := rm.calculateDelay(attempt - 1)
			rm.logger.LogRecovery(ctx, err, attempt, rm.config.MaxAttempts, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	rm.logger.LogRecoveryFailure(ctx, lastErr, rm.config.MaxAttempts)
	return fmt.Errorf("operation failed after %d attempts: %w", rm.config.MaxAttempts, lastErr)
}

// CircuitBreaker implements a circuit breaker pattern for error recovery
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	logger          *logging.Logger
	state           CircuitState
	failures        int
	lastFailTime    time.Time
	lastSuccessTime time.Time
}

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold" mapstructure:"failure_threshold"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout" mapstructure:"recovery_timeout"`
	HalfOpenMaxCalls int           `yaml:"half_open_max_calls" mapstructure:"half_open_max_calls"`
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, logger *logging.Logger) *CircuitBreaker {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}

	return &CircuitBreaker{
		config: config,
		logger: logger.WithComponent("circuit_breaker"),
		state:  CircuitStateClosed,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation Operation, operationName string) error {
	if !cb.canExecute() {
		return errors.NewError(errors.ErrorTypeSystem, "CIRCUIT_BREAKER_OPEN",
			"circuit breaker is open").
			WithGuidance("Wait for the circuit breaker to recover or check system health")
	}

	err := operation(ctx, 1)
	cb.recordResult(err, operationName)
	return err
}

// canExecute checks if the operation can be executed based on circuit breaker state
func (cb *CircuitBreaker) canExecute() bool {
	now := time.Now()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		if now.Sub(cb.lastFailTime) >= cb.config.RecoveryTimeout {
			cb.state = CircuitStateHalfOpen
			cb.logger.Info("Circuit breaker transitioning to half-open state")
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an operation execution
func (cb *CircuitBreaker) recordResult(err error, operationName string) {
	now := time.Now()

	if err == nil {
		cb.onSuccess(now, operationName)
	} else {
		cb.onFailure(now, operationName, err)
	}
}

// onSuccess handles successful operation execution
func (cb *CircuitBreaker) onSuccess(timestamp time.Time, operationName string) {
	cb.lastSuccessTime = timestamp

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.state = CircuitStateClosed
		cb.failures = 0
		cb.logger.Info("Circuit breaker closed after successful recovery",
			"operation", operationName)
	case CircuitStateClosed:
		cb.failures = 0
	}
}

// onFailure handles failed operation execution
func (cb *CircuitBreaker) onFailure(timestamp time.Time, operationName string, err error) {
	cb.failures++
	cb.lastFailTime = timestamp

	switch cb.state {
	case CircuitStateClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitStateOpen
			cb.logger.Warn("Circuit breaker opened due to failures",
				"operation", operationName,
				"failures", cb.failures,
				"threshold", cb.config.FailureThreshold,
				"error", err)
		}
	case CircuitStateHalfOpen:
		cb.state = CircuitStateOpen
		cb.logger.Warn("Circuit breaker reopened after failed recovery attempt",
			"operation", operationName,
			"error", err)
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"state":             cb.state,
		"failures":          cb.failures,
		"last_fail_time":    cb.lastFailTime,
		"last_success_time": cb.lastSuccessTime,
	}
}

// Helper methods

// calculateDelay calculates the delay for the next retry attempt
func (rm *RecoveryManager) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch rm.config.Strategy {
	case RetryStrategyFixed:
		delay = rm.config.InitialDelay
	case RetryStrategyLinear:
		delay = rm.config.InitialDelay * time.Duration(attempt+1)
	case RetryStrategyExponential:
		delay = rm.config.InitialDelay * time.Duration(math.Pow(2, float64(attempt)))
	default:
		delay = rm.config.InitialDelay
	}

	// Apply maximum delay limit
	if rm.config.MaxDelay > 0 && delay > rm.config.MaxDelay {
		delay = rm.config.MaxDelay
	}

	// Add jitter if enabled
	if rm.config.Jitter {
		// Use crypto/rand for secure random number generation
		maxJitter := int64(float64(delay) * rm.config.JitterPercent)
		if maxJitter > 0 {
			jitterBig, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
			if err == nil {
				jitter := time.Duration(jitterBig.Int64())
				delay += jitter
			}
			// If crypto/rand fails, continue without jitter (safer than using weak rand)
		}
	}

	return delay
}

// isRecoverable checks if an error is recoverable
func (rm *RecoveryManager) isRecoverable(err error) bool {
	// Check if it's a DriftWatch error with recoverable flag
	if errors.IsRecoverable(err) {
		return true
	}

	// Check error type for known recoverable errors
	errorType := errors.GetErrorType(err)
	switch errorType {
	case errors.ErrorTypeNetwork:
		return true // Most network errors are recoverable
	case errors.ErrorTypeStorage:
		// Some storage errors are recoverable
		if dwe, ok := err.(*errors.DriftWatchError); ok {
			return dwe.Code != "STORAGE_CORRUPTION"
		}
		return false
	case errors.ErrorTypeAlert:
		return true
	case errors.ErrorTypeAuth:
		// Auth errors might be recoverable if tokens can be refreshed -
		if dwe, ok := err.(*errors.DriftWatchError); ok {
			return dwe.Code == "AUTH_EXPIRED"
		}
		return false
	default:
		return false
	}
}

// IsNetworkError checks if an error is a network-related error
func IsNetworkError(err error) bool {
	return errors.GetErrorType(err) == errors.ErrorTypeNetwork
}

// IsTemporaryError checks if an error is temporary and should be retried
func IsTemporaryError(err error) bool {
	if dwe, ok := err.(*errors.DriftWatchError); ok {
		return dwe.Recoverable
	}
	return false
}

// WithRecovery wraps an operation with recovery logic
func WithRecovery(ctx context.Context, operation Operation, config RecoveryConfig, operationName string) error {
	rm := NewRecoveryManager(config, nil)
	return rm.Retry(ctx, operation, operationName)
}

// WithCircuitBreaker wraps an operation with circuit breaker logic
func WithCircuitBreaker(ctx context.Context, operation Operation, config CircuitBreakerConfig, operationName string) error {
	cb := NewCircuitBreaker(config, nil)
	return cb.Execute(ctx, operation, operationName)
}
