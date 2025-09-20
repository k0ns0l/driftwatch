package recovery

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, RetryStrategyExponential, config.Strategy)
	assert.True(t, config.Jitter)
	assert.Equal(t, 0.1, config.JitterPercent)
}

func TestNewRecoveryManager(t *testing.T) {
	config := DefaultRecoveryConfig()
	logger, err := logging.NewLogger(logging.DefaultLoggerConfig())
	require.NoError(t, err)
	defer logger.Close()

	rm := NewRecoveryManager(config, logger)

	assert.NotNil(t, rm)
	assert.Equal(t, config, rm.config)
	assert.NotNil(t, rm.logger)
}

func TestRecoveryManager_Retry_Success(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempts < 3 {
			return errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
		}
		return nil
	}

	err := rm.Retry(ctx, operation, "test_operation")

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRecoveryManager_Retry_Failure(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
	}

	err := rm.Retry(ctx, operation, "test_operation")

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
	assert.Contains(t, err.Error(), "operation failed after 3 attempts")
}

func TestRecoveryManager_Retry_NonRecoverable(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return errors.NewError(errors.ErrorTypeConfig, "CONFIG_INVALID", "invalid config").WithRecoverable(false)
	}

	err := rm.Retry(ctx, operation, "test_operation")

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry non-recoverable errors
}

func TestRecoveryManager_Retry_ContextCancellation(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  5,
		InitialDelay: 200 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
	}

	err := rm.Retry(ctx, operation, "test_operation")

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Equal(t, 1, attempts) // Should stop after context cancellation
}

func TestRecoveryManager_RetryWithResult_Success(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attempts++
		if attempts < 3 {
			return "", errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
		}
		return "success", nil
	}

	result, err := rm.RetryWithResult(ctx, operation, "test_operation")

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attempts)
}

func TestRecoveryManager_RetryWithResult_Failure(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attempts++
		return 0, errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
	}

	result, err := rm.RetryWithResult(ctx, operation, "test_operation")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 2, attempts)
}

func TestRecoveryManager_RetryIf(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempts < 3 {
			return errors.NewError(errors.ErrorTypeNetwork, "TEMPORARY_ERROR", "temporary error").WithRecoverable(true)
		}
		return nil
	}

	// Custom condition: retry if error message contains "temporary"
	condition := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), "temporary error")
	}

	err := rm.RetryIf(ctx, operation, condition, "test_operation")

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRecoveryManager_RetryIf_ConditionFalse(t *testing.T) {
	config := RecoveryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return fmt.Errorf("permanent error")
	}

	// Custom condition: only retry temporary errors
	condition := func(err error) bool {
		return err != nil && err.Error() == "temporary error"
	}

	err := rm.RetryIf(ctx, operation, condition, "test_operation")

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry when condition is false
}

func TestRecoveryManager_CalculateDelay(t *testing.T) {
	tests := []struct {
		name     string
		config   RecoveryConfig
		attempt  int
		expected time.Duration
	}{
		{
			name: "fixed strategy",
			config: RecoveryConfig{
				InitialDelay: 1 * time.Second,
				MaxDelay:     10 * time.Second,
				Strategy:     RetryStrategyFixed,
				Jitter:       false,
			},
			attempt:  2,
			expected: 1 * time.Second,
		},
		{
			name: "linear strategy",
			config: RecoveryConfig{
				InitialDelay: 1 * time.Second,
				MaxDelay:     10 * time.Second,
				Strategy:     RetryStrategyLinear,
				Jitter:       false,
			},
			attempt:  2,
			expected: 3 * time.Second, // (attempt + 1) * initial_delay = 3 * 1s
		},
		{
			name: "exponential strategy",
			config: RecoveryConfig{
				InitialDelay: 1 * time.Second,
				MaxDelay:     10 * time.Second,
				Strategy:     RetryStrategyExponential,
				Jitter:       false,
			},
			attempt:  2,
			expected: 4 * time.Second, // 2^attempt * initial_delay = 4 * 1s
		},
		{
			name: "max delay limit",
			config: RecoveryConfig{
				InitialDelay: 1 * time.Second,
				MaxDelay:     3 * time.Second,
				Strategy:     RetryStrategyExponential,
				Jitter:       false,
			},
			attempt:  5,
			expected: 3 * time.Second, // Should be capped at max_delay
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewRecoveryManager(tt.config, nil)
			delay := rm.calculateDelay(tt.attempt)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestRecoveryManager_CalculateDelayWithJitter(t *testing.T) {
	config := RecoveryConfig{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		Strategy:      RetryStrategyFixed,
		Jitter:        true,
		JitterPercent: 0.1,
	}

	rm := NewRecoveryManager(config, nil)

	// Test multiple times to ensure jitter is applied
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = rm.calculateDelay(0)
	}

	// At least some delays should be different due to jitter
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	assert.False(t, allSame, "Jitter should cause some variation in delays")

	// All delays should be within expected range (base + 10% jitter)
	baseDelay := 1 * time.Second
	maxJitter := time.Duration(float64(baseDelay) * 0.1)

	for _, delay := range delays {
		assert.GreaterOrEqual(t, delay, baseDelay)
		assert.LessOrEqual(t, delay, baseDelay+maxJitter)
	}
}

func TestIsRecoverable(t *testing.T) {
	rm := NewRecoveryManager(DefaultRecoveryConfig(), nil)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "recoverable DriftWatch error",
			err:      errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true),
			expected: true,
		},
		{
			name:     "non-recoverable DriftWatch error",
			err:      errors.NewError(errors.ErrorTypeConfig, "CONFIG_INVALID", "invalid").WithRecoverable(false),
			expected: false,
		},
		{
			name:     "network error (recoverable by type)",
			err:      errors.NewError(errors.ErrorTypeNetwork, "NETWORK_ERROR", "network error"),
			expected: true,
		},
		{
			name:     "storage corruption (non-recoverable)",
			err:      errors.NewError(errors.ErrorTypeStorage, "STORAGE_CORRUPTION", "corruption"),
			expected: false,
		},
		{
			name:     "auth expired (recoverable)",
			err:      errors.NewError(errors.ErrorTypeAuth, "AUTH_EXPIRED", "expired"),
			expected: true,
		},
		{
			name:     "auth invalid (non-recoverable)",
			err:      errors.NewError(errors.ErrorTypeAuth, "AUTH_INVALID", "invalid"),
			expected: false,
		},
		{
			name:     "alert delivery error (recoverable)",
			err:      errors.NewError(errors.ErrorTypeAlert, "ALERT_DELIVERY", "delivery failed"),
			expected: true,
		},
		{
			name:     "standard error (non-recoverable)",
			err:      fmt.Errorf("standard error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rm.isRecoverable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 30*time.Second, config.RecoveryTimeout)
	assert.Equal(t, 3, config.HalfOpenMaxCalls)
}

func TestNewCircuitBreaker(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	logger, err := logging.NewLogger(logging.DefaultLoggerConfig())
	require.NoError(t, err)
	defer logger.Close()

	cb := NewCircuitBreaker(config, logger)

	assert.NotNil(t, cb)
	assert.Equal(t, config, cb.config)
	assert.Equal(t, CircuitStateClosed, cb.state)
	assert.NotNil(t, cb.logger)
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config, nil)
	ctx := context.Background()

	operation := func(ctx context.Context, attempt int) error {
		return nil
	}

	err := cb.Execute(ctx, operation, "test_operation")

	assert.NoError(t, err)
	assert.Equal(t, CircuitStateClosed, cb.GetState())
}

func TestCircuitBreaker_Execute_FailureThreshold(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config, nil)
	ctx := context.Background()

	operation := func(ctx context.Context, attempt int) error {
		return fmt.Errorf("operation failed")
	}

	// Execute operations until threshold is reached
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, operation, "test_operation")
		assert.Error(t, err)
	}

	// Circuit should now be open
	assert.Equal(t, CircuitStateOpen, cb.GetState())

	// Next execution should fail immediately
	err := cb.Execute(ctx, operation, "test_operation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config, nil)
	ctx := context.Background()

	failingOperation := func(ctx context.Context, attempt int) error {
		return fmt.Errorf("operation failed")
	}

	successOperation := func(ctx context.Context, attempt int) error {
		return nil
	}

	// Trigger circuit breaker to open
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, failingOperation, "test_operation")
	}
	assert.Equal(t, CircuitStateOpen, cb.GetState())

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Next execution should transition to half-open
	err := cb.Execute(ctx, successOperation, "test_operation")
	assert.NoError(t, err)
	assert.Equal(t, CircuitStateClosed, cb.GetState())
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config, nil)
	ctx := context.Background()

	failingOperation := func(ctx context.Context, attempt int) error {
		return fmt.Errorf("operation failed")
	}

	// Trigger circuit breaker to open
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, failingOperation, "test_operation")
	}
	assert.Equal(t, CircuitStateOpen, cb.GetState())

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Next execution should fail and reopen circuit
	err := cb.Execute(ctx, failingOperation, "test_operation")
	assert.Error(t, err)
	assert.Equal(t, CircuitStateOpen, cb.GetState())
}

func TestCircuitBreaker_GetMetrics(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(), nil)

	metrics := cb.GetMetrics()

	assert.Contains(t, metrics, "state")
	assert.Contains(t, metrics, "failures")
	assert.Contains(t, metrics, "last_fail_time")
	assert.Contains(t, metrics, "last_success_time")

	assert.Equal(t, CircuitStateClosed, metrics["state"])
	assert.Equal(t, 0, metrics["failures"])
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsNetworkError", func(t *testing.T) {
		networkErr := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_ERROR", "network error")
		configErr := errors.NewError(errors.ErrorTypeConfig, "CONFIG_ERROR", "config error")

		assert.True(t, IsNetworkError(networkErr))
		assert.False(t, IsNetworkError(configErr))
	})

	t.Run("IsTemporaryError", func(t *testing.T) {
		temporaryErr := errors.NewError(errors.ErrorTypeNetwork, "NETWORK_ERROR", "network error").WithRecoverable(true)
		permanentErr := errors.NewError(errors.ErrorTypeConfig, "CONFIG_ERROR", "config error").WithRecoverable(false)
		standardErr := fmt.Errorf("standard error")

		assert.True(t, IsTemporaryError(temporaryErr))
		assert.False(t, IsTemporaryError(permanentErr))
		assert.False(t, IsTemporaryError(standardErr))
	})
}

func TestWithRecovery(t *testing.T) {
	ctx := context.Background()
	config := RecoveryConfig{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempts < 2 {
			return errors.NewError(errors.ErrorTypeNetwork, "NETWORK_TIMEOUT", "timeout").WithRecoverable(true)
		}
		return nil
	}

	err := WithRecovery(ctx, operation, config, "test_operation")

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestWithCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
	}

	operation := func(ctx context.Context, attempt int) error {
		return nil
	}

	err := WithCircuitBreaker(ctx, operation, config, "test_operation")

	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkRecoveryManager_Retry(b *testing.B) {
	config := RecoveryConfig{
		MaxAttempts:  1,
		InitialDelay: 1 * time.Millisecond,
		Strategy:     RetryStrategyFixed,
		Jitter:       false,
	}

	rm := NewRecoveryManager(config, nil)
	ctx := context.Background()

	operation := func(ctx context.Context, attempt int) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Retry(ctx, operation, "benchmark_operation")
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(), nil)
	ctx := context.Background()

	operation := func(ctx context.Context, attempt int) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(ctx, operation, "benchmark_operation")
	}
}
