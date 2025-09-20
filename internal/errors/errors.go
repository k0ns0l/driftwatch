// Package errors provides structured error handling for DriftWatch
package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeConfig     ErrorType = "CONFIG"
	ErrorTypeNetwork    ErrorType = "NETWORK"
	ErrorTypeValidation ErrorType = "VALIDATION"
	ErrorTypeStorage    ErrorType = "STORAGE"
	ErrorTypeAlert      ErrorType = "ALERT"
	ErrorTypeAuth       ErrorType = "AUTH"
	ErrorTypeSystem     ErrorType = "SYSTEM"
)

// Severity represents the severity level of an error
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// DriftWatchError represents a structured error with context and recovery guidance
type DriftWatchError struct {
	Type        ErrorType              `json:"type"`
	Severity    Severity               `json:"severity"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Guidance    string                 `json:"guidance,omitempty"`
	Cause       error                  `json:"cause,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Recoverable bool                   `json:"recoverable"`
}

// Error implements the error interface
func (e *DriftWatchError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[%s:%s]", e.Type, e.Code))
	parts = append(parts, e.Message)

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Cause))
	}

	return strings.Join(parts, " ")
}

// Unwrap returns the underlying cause error
func (e *DriftWatchError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error type
func (e *DriftWatchError) Is(target error) bool {
	if t, ok := target.(*DriftWatchError); ok {
		return e.Type == t.Type && e.Code == t.Code
	}
	return false
}

// WithContext adds context information to the error
func (e *DriftWatchError) WithContext(key string, value interface{}) *DriftWatchError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithGuidance adds recovery guidance to the error
func (e *DriftWatchError) WithGuidance(guidance string) *DriftWatchError {
	e.Guidance = guidance
	return e
}

// NewError creates a new DriftWatchError
func NewError(errorType ErrorType, code, message string) *DriftWatchError {
	return &DriftWatchError{
		Type:        errorType,
		Code:        code,
		Message:     message,
		Severity:    SeverityMedium,
		Recoverable: false,
		Context:     make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with DriftWatch error context
func WrapError(err error, errorType ErrorType, code, message string) *DriftWatchError {
	return &DriftWatchError{
		Type:        errorType,
		Code:        code,
		Message:     message,
		Cause:       err,
		Severity:    SeverityMedium,
		Recoverable: false,
		Context:     make(map[string]interface{}),
	}
}

// Configuration Errors
var (
	ErrConfigNotFound = NewError(ErrorTypeConfig, "CONFIG_NOT_FOUND", "configuration file not found").
				WithGuidance("Run 'driftwatch config init' to create a default configuration file")

	ErrConfigInvalid = NewError(ErrorTypeConfig, "CONFIG_INVALID", "configuration file is invalid").
				WithGuidance("Run 'driftwatch config validate' to see detailed validation errors")

	ErrConfigPermission = NewError(ErrorTypeConfig, "CONFIG_PERMISSION", "insufficient permissions to read configuration file").
				WithGuidance("Check file permissions and ensure the configuration file is readable")

	ErrEndpointNotFound = NewError(ErrorTypeConfig, "ENDPOINT_NOT_FOUND", "endpoint not found in configuration").
				WithGuidance("Use 'driftwatch list' to see available endpoints or 'driftwatch add' to add new ones")
)

// Network Errors
var (
	ErrNetworkTimeout = NewError(ErrorTypeNetwork, "NETWORK_TIMEOUT", "network request timed out").
				WithSeverity(SeverityHigh).
				WithRecoverable(true).
				WithGuidance("Check network connectivity and consider increasing timeout values")

	ErrNetworkConnection = NewError(ErrorTypeNetwork, "NETWORK_CONNECTION", "failed to establish network connection").
				WithSeverity(SeverityHigh).
				WithRecoverable(true).
				WithGuidance("Verify the endpoint URL is correct and the service is accessible")

	ErrNetworkDNS = NewError(ErrorTypeNetwork, "NETWORK_DNS", "DNS resolution failed").
			WithSeverity(SeverityHigh).
			WithRecoverable(true).
			WithGuidance("Check DNS settings and verify the hostname is correct")

	ErrNetworkTLS = NewError(ErrorTypeNetwork, "NETWORK_TLS", "TLS/SSL connection failed").
			WithSeverity(SeverityHigh).
			WithRecoverable(false).
			WithGuidance("Verify SSL certificate validity or consider using --insecure flag for testing")
)

// Validation Errors
var (
	ErrValidationSchema = NewError(ErrorTypeValidation, "VALIDATION_SCHEMA", "response validation against schema failed").
				WithSeverity(SeverityMedium).
				WithGuidance("Check if the API response format has changed or update the OpenAPI specification")

	ErrValidationSpec = NewError(ErrorTypeValidation, "VALIDATION_SPEC", "OpenAPI specification is invalid").
				WithSeverity(SeverityHigh).
				WithGuidance("Validate your OpenAPI specification file using an OpenAPI validator")

	ErrValidationFormat = NewError(ErrorTypeValidation, "VALIDATION_FORMAT", "response format is invalid").
				WithSeverity(SeverityMedium).
				WithGuidance("Ensure the API returns valid JSON or the expected content type")
)

// Storage Errors
var (
	ErrStorageConnection = NewError(ErrorTypeStorage, "STORAGE_CONNECTION", "failed to connect to database").
				WithSeverity(SeverityCritical).
				WithGuidance("Check database file permissions and available disk space")

	ErrStorageCorruption = NewError(ErrorTypeStorage, "STORAGE_CORRUPTION", "database corruption detected").
				WithSeverity(SeverityCritical).
				WithGuidance("Consider restoring from backup or reinitializing the database")

	ErrStorageDiskSpace = NewError(ErrorTypeStorage, "STORAGE_DISK_SPACE", "insufficient disk space").
				WithSeverity(SeverityCritical).
				WithGuidance("Free up disk space or configure data retention policies")

	ErrStoragePermission = NewError(ErrorTypeStorage, "STORAGE_PERMISSION", "insufficient permissions for database operations").
				WithSeverity(SeverityCritical).
				WithGuidance("Check file permissions for the database directory and files")
)

// Alert Errors
var (
	ErrAlertDelivery = NewError(ErrorTypeAlert, "ALERT_DELIVERY", "failed to deliver alert").
				WithSeverity(SeverityHigh).
				WithRecoverable(true).
				WithGuidance("Check alert channel configuration and network connectivity")

	ErrAlertConfig = NewError(ErrorTypeAlert, "ALERT_CONFIG", "alert configuration is invalid").
			WithSeverity(SeverityMedium).
			WithGuidance("Validate alert channel settings and credentials")

	ErrAlertAuth = NewError(ErrorTypeAlert, "ALERT_AUTH", "alert channel authentication failed").
			WithSeverity(SeverityHigh).
			WithGuidance("Check authentication credentials and permissions for the alert channel")
)

// Authentication Errors
var (
	ErrAuthInvalid = NewError(ErrorTypeAuth, "AUTH_INVALID", "authentication credentials are invalid").
			WithSeverity(SeverityHigh).
			WithGuidance("Check API tokens, usernames, and passwords in your configuration")

	ErrAuthExpired = NewError(ErrorTypeAuth, "AUTH_EXPIRED", "authentication credentials have expired").
			WithSeverity(SeverityHigh).
			WithRecoverable(true).
			WithGuidance("Refresh or renew your authentication credentials")

	ErrAuthPermission = NewError(ErrorTypeAuth, "AUTH_PERMISSION", "insufficient permissions for the requested operation").
				WithSeverity(SeverityHigh).
				WithGuidance("Ensure your credentials have the necessary permissions for this endpoint")
)

// System Errors
var (
	ErrSystemResource = NewError(ErrorTypeSystem, "SYSTEM_RESOURCE", "system resource exhausted").
				WithSeverity(SeverityCritical).
				WithGuidance("Check system resources (CPU, memory, file descriptors) and consider reducing concurrent operations")

	ErrSystemPermission = NewError(ErrorTypeSystem, "SYSTEM_PERMISSION", "insufficient system permissions").
				WithSeverity(SeverityCritical).
				WithGuidance("Run with appropriate permissions or check file/directory access rights")
)

// Helper methods for error creation

// WithSeverity sets the severity level of the error
func (e *DriftWatchError) WithSeverity(severity Severity) *DriftWatchError {
	e.Severity = severity
	return e
}

// WithRecoverable sets whether the error is recoverable
func (e *DriftWatchError) WithRecoverable(recoverable bool) *DriftWatchError {
	e.Recoverable = recoverable
	return e
}

// IsRecoverable checks if an error is recoverable
func IsRecoverable(err error) bool {
	if dwe, ok := err.(*DriftWatchError); ok {
		return dwe.Recoverable
	}
	return false
}

// GetSeverity returns the severity of an error
func GetSeverity(err error) Severity {
	if dwe, ok := err.(*DriftWatchError); ok {
		return dwe.Severity
	}
	return SeverityMedium
}

// GetErrorType returns the type of an error
func GetErrorType(err error) ErrorType {
	if dwe, ok := err.(*DriftWatchError); ok {
		return dwe.Type
	}
	return ErrorTypeSystem
}

// GetGuidance returns recovery guidance for an error
func GetGuidance(err error) string {
	if dwe, ok := err.(*DriftWatchError); ok {
		return dwe.Guidance
	}
	return "Check the error message and logs for more information"
}
