package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	err := NewError(ErrorTypeConfig, "TEST_CODE", "test message")

	assert.Equal(t, ErrorTypeConfig, err.Type)
	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, SeverityMedium, err.Severity)
	assert.False(t, err.Recoverable)
	assert.NotNil(t, err.Context)
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapError(originalErr, ErrorTypeNetwork, "NETWORK_ERROR", "network failed")

	assert.Equal(t, ErrorTypeNetwork, wrappedErr.Type)
	assert.Equal(t, "NETWORK_ERROR", wrappedErr.Code)
	assert.Equal(t, "network failed", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.Equal(t, originalErr, wrappedErr.Unwrap())
}

func TestDriftWatchError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DriftWatchError
		expected string
	}{
		{
			name: "simple error",
			err: &DriftWatchError{
				Type:    ErrorTypeConfig,
				Code:    "CONFIG_INVALID",
				Message: "configuration is invalid",
			},
			expected: "[CONFIG:CONFIG_INVALID] configuration is invalid",
		},
		{
			name: "error with cause",
			err: &DriftWatchError{
				Type:    ErrorTypeNetwork,
				Code:    "NETWORK_TIMEOUT",
				Message: "request timed out",
				Cause:   errors.New("connection timeout"),
			},
			expected: "[NETWORK:NETWORK_TIMEOUT] request timed out caused by: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestDriftWatchError_WithContext(t *testing.T) {
	err := NewError(ErrorTypeStorage, "STORAGE_ERROR", "storage failed")
	err.WithContext("endpoint_id", "test-endpoint")
	err.WithContext("attempt", 3)

	assert.Equal(t, "test-endpoint", err.Context["endpoint_id"])
	assert.Equal(t, 3, err.Context["attempt"])
}

func TestDriftWatchError_WithGuidance(t *testing.T) {
	err := NewError(ErrorTypeConfig, "CONFIG_NOT_FOUND", "config file not found")
	err.WithGuidance("Run 'driftwatch config init' to create a config file")

	assert.Equal(t, "Run 'driftwatch config init' to create a config file", err.Guidance)
}

func TestDriftWatchError_WithSeverity(t *testing.T) {
	err := NewError(ErrorTypeSystem, "SYSTEM_ERROR", "system error")
	err.WithSeverity(SeverityCritical)

	assert.Equal(t, SeverityCritical, err.Severity)
}

func TestDriftWatchError_WithRecoverable(t *testing.T) {
	err := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error")
	err.WithRecoverable(true)

	assert.True(t, err.Recoverable)
}

func TestDriftWatchError_Is(t *testing.T) {
	err1 := NewError(ErrorTypeConfig, "CONFIG_INVALID", "config invalid")
	err2 := NewError(ErrorTypeConfig, "CONFIG_INVALID", "different message")
	err3 := NewError(ErrorTypeConfig, "CONFIG_NOT_FOUND", "config not found")
	err4 := NewError(ErrorTypeNetwork, "CONFIG_INVALID", "network error")

	assert.True(t, err1.Is(err2))
	assert.False(t, err1.Is(err3))
	assert.False(t, err1.Is(err4))
	assert.False(t, err1.Is(errors.New("standard error")))
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *DriftWatchError
		wantType ErrorType
		wantCode string
	}{
		{"ErrConfigNotFound", ErrConfigNotFound, ErrorTypeConfig, "CONFIG_NOT_FOUND"},
		{"ErrConfigInvalid", ErrConfigInvalid, ErrorTypeConfig, "CONFIG_INVALID"},
		{"ErrNetworkTimeout", ErrNetworkTimeout, ErrorTypeNetwork, "NETWORK_TIMEOUT"},
		{"ErrNetworkConnection", ErrNetworkConnection, ErrorTypeNetwork, "NETWORK_CONNECTION"},
		{"ErrValidationSchema", ErrValidationSchema, ErrorTypeValidation, "VALIDATION_SCHEMA"},
		{"ErrStorageConnection", ErrStorageConnection, ErrorTypeStorage, "STORAGE_CONNECTION"},
		{"ErrAlertDelivery", ErrAlertDelivery, ErrorTypeAlert, "ALERT_DELIVERY"},
		{"ErrAuthInvalid", ErrAuthInvalid, ErrorTypeAuth, "AUTH_INVALID"},
		{"ErrSystemResource", ErrSystemResource, ErrorTypeSystem, "SYSTEM_RESOURCE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.err.Type)
			assert.Equal(t, tt.wantCode, tt.err.Code)
			assert.NotEmpty(t, tt.err.Message)
			assert.NotEmpty(t, tt.err.Guidance)
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsRecoverable", func(t *testing.T) {
		recoverableErr := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error").WithRecoverable(true)
		nonRecoverableErr := NewError(ErrorTypeConfig, "CONFIG_ERROR", "config error").WithRecoverable(false)
		standardErr := errors.New("standard error")

		assert.True(t, IsRecoverable(recoverableErr))
		assert.False(t, IsRecoverable(nonRecoverableErr))
		assert.False(t, IsRecoverable(standardErr))
	})

	t.Run("GetSeverity", func(t *testing.T) {
		criticalErr := NewError(ErrorTypeSystem, "SYSTEM_ERROR", "system error").WithSeverity(SeverityCritical)
		standardErr := errors.New("standard error")

		assert.Equal(t, SeverityCritical, GetSeverity(criticalErr))
		assert.Equal(t, SeverityMedium, GetSeverity(standardErr))
	})

	t.Run("GetErrorType", func(t *testing.T) {
		networkErr := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error")
		standardErr := errors.New("standard error")

		assert.Equal(t, ErrorTypeNetwork, GetErrorType(networkErr))
		assert.Equal(t, ErrorTypeSystem, GetErrorType(standardErr))
	})

	t.Run("GetGuidance", func(t *testing.T) {
		guidanceErr := NewError(ErrorTypeConfig, "CONFIG_ERROR", "config error").WithGuidance("Check your config")
		noGuidanceErr := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error")
		standardErr := errors.New("standard error")

		assert.Equal(t, "Check your config", GetGuidance(guidanceErr))
		assert.Empty(t, GetGuidance(noGuidanceErr))
		assert.Equal(t, "Check the error message and logs for more information", GetGuidance(standardErr))
	})
}

func TestErrorChaining(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapError(originalErr, ErrorTypeNetwork, "NETWORK_ERROR", "network failed")

	// Test error unwrapping
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.Equal(t, originalErr, errors.Unwrap(wrappedErr))

	// Test error chain
	var driftWatchErr *DriftWatchError
	assert.True(t, errors.As(wrappedErr, &driftWatchErr))
	assert.Equal(t, ErrorTypeNetwork, driftWatchErr.Type)
}

func TestErrorSeverityLevels(t *testing.T) {
	severityLevels := []Severity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, severity := range severityLevels {
		err := NewError(ErrorTypeSystem, "TEST_ERROR", "test error").WithSeverity(severity)
		assert.Equal(t, severity, err.Severity)
		assert.Equal(t, severity, GetSeverity(err))
	}
}

func TestErrorTypes(t *testing.T) {
	errorTypes := []ErrorType{
		ErrorTypeConfig,
		ErrorTypeNetwork,
		ErrorTypeValidation,
		ErrorTypeStorage,
		ErrorTypeAlert,
		ErrorTypeAuth,
		ErrorTypeSystem,
	}

	for _, errorType := range errorTypes {
		err := NewError(errorType, "TEST_ERROR", "test error")
		assert.Equal(t, errorType, err.Type)
		assert.Equal(t, errorType, GetErrorType(err))
	}
}

func TestContextManipulation(t *testing.T) {
	err := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error")

	// Add various types of context
	err.WithContext("string_value", "test")
	err.WithContext("int_value", 42)
	err.WithContext("bool_value", true)
	err.WithContext("float_value", 3.14)

	assert.Equal(t, "test", err.Context["string_value"])
	assert.Equal(t, 42, err.Context["int_value"])
	assert.Equal(t, true, err.Context["bool_value"])
	assert.Equal(t, 3.14, err.Context["float_value"])
}

func TestMethodChaining(t *testing.T) {
	err := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error").
		WithSeverity(SeverityHigh).
		WithRecoverable(true).
		WithGuidance("Check network connectivity").
		WithContext("endpoint", "https://api.example.com").
		WithContext("timeout", "30s")

	assert.Equal(t, ErrorTypeNetwork, err.Type)
	assert.Equal(t, "NETWORK_ERROR", err.Code)
	assert.Equal(t, SeverityHigh, err.Severity)
	assert.True(t, err.Recoverable)
	assert.Equal(t, "Check network connectivity", err.Guidance)
	assert.Equal(t, "https://api.example.com", err.Context["endpoint"])
	assert.Equal(t, "30s", err.Context["timeout"])
}

// Benchmark tests
func BenchmarkNewError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error")
	}
}

func BenchmarkWrapError(b *testing.B) {
	originalErr := errors.New("original error")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		WrapError(originalErr, ErrorTypeNetwork, "NETWORK_ERROR", "network failed")
	}
}

func BenchmarkErrorString(b *testing.B) {
	err := NewError(ErrorTypeNetwork, "NETWORK_ERROR", "network error").
		WithContext("endpoint", "https://api.example.com")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}
