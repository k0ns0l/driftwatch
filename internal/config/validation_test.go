package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateConfig_ValidConfig(t *testing.T) {
	config := DefaultConfig()
	config.Endpoints = []EndpointConfig{
		{
			ID:       "test-endpoint",
			URL:      "https://api.test.com/v1/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
			Enabled:  true,
		},
	}

	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateProject(t *testing.T) {
	tests := []struct {
		name        string
		project     ProjectConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid project",
			project: ProjectConfig{
				Name:        "Test Project",
				Description: "Test description",
				Version:     "1.0.0",
			},
			expectError: false,
		},
		{
			name: "empty name",
			project: ProjectConfig{
				Name: "",
			},
			expectError: true,
			errorMsg:    "project name cannot be empty",
		},
		{
			name: "name too long",
			project: ProjectConfig{
				Name: string(make([]byte, 101)), // 101 characters
			},
			expectError: true,
			errorMsg:    "project name cannot exceed 100 characters",
		},
		{
			name: "description too long",
			project: ProjectConfig{
				Name:        "Test",
				Description: string(make([]byte, 501)), // 501 characters
			},
			expectError: true,
			errorMsg:    "project description cannot exceed 500 characters",
		},
		{
			name: "invalid version format",
			project: ProjectConfig{
				Name:    "Test",
				Version: "invalid-version",
			},
			expectError: true,
			errorMsg:    "invalid version format",
		},
		{
			name: "valid version with v prefix",
			project: ProjectConfig{
				Name:    "Test",
				Version: "v1.0.0",
			},
			expectError: false,
		},
		{
			name: "valid version with prerelease",
			project: ProjectConfig{
				Name:    "Test",
				Version: "1.0.0-alpha.1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProject(&tt.project)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGlobal(t *testing.T) {
	tests := []struct {
		name        string
		global      GlobalConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid global config",
			global: GlobalConfig{
				UserAgent:   "test-agent/1.0",
				Timeout:     30 * time.Second,
				RetryCount:  3,
				RetryDelay:  5 * time.Second,
				MaxWorkers:  10,
				DatabaseURL: "./test.db",
			},
			expectError: false,
		},
		{
			name: "empty user agent",
			global: GlobalConfig{
				UserAgent: "",
			},
			expectError: true,
			errorMsg:    "user agent cannot be empty",
		},
		{
			name: "negative timeout",
			global: GlobalConfig{
				UserAgent: "test",
				Timeout:   -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "timeout too large",
			global: GlobalConfig{
				UserAgent: "test",
				Timeout:   6 * time.Minute,
			},
			expectError: true,
			errorMsg:    "timeout cannot exceed 5 minutes",
		},
		{
			name: "negative retry count",
			global: GlobalConfig{
				UserAgent:  "test",
				Timeout:    30 * time.Second,
				RetryCount: -1,
			},
			expectError: true,
			errorMsg:    "retry count cannot be negative",
		},
		{
			name: "retry count too large",
			global: GlobalConfig{
				UserAgent:  "test",
				Timeout:    30 * time.Second,
				RetryCount: 11,
			},
			expectError: true,
			errorMsg:    "retry count cannot exceed 10",
		},
		{
			name: "negative retry delay",
			global: GlobalConfig{
				UserAgent:  "test",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				RetryDelay: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "retry delay must be positive",
		},
		{
			name: "invalid max workers",
			global: GlobalConfig{
				UserAgent:   "test",
				Timeout:     30 * time.Second,
				RetryCount:  3,
				RetryDelay:  5 * time.Second,
				MaxWorkers:  0,
				DatabaseURL: "./test.db",
			},
			expectError: true,
			errorMsg:    "max workers must be positive",
		},
		{
			name: "max workers too large",
			global: GlobalConfig{
				UserAgent:   "test",
				Timeout:     30 * time.Second,
				RetryCount:  3,
				RetryDelay:  5 * time.Second,
				MaxWorkers:  101,
				DatabaseURL: "./test.db",
			},
			expectError: true,
			errorMsg:    "max workers cannot exceed 100",
		},
		{
			name: "empty database URL",
			global: GlobalConfig{
				UserAgent:   "test",
				Timeout:     30 * time.Second,
				RetryCount:  3,
				RetryDelay:  5 * time.Second,
				MaxWorkers:  10,
				DatabaseURL: "",
			},
			expectError: true,
			errorMsg:    "database URL cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGlobal(&tt.global)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    EndpointConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid endpoint",
			endpoint: EndpointConfig{
				ID:       "test-endpoint",
				URL:      "https://api.test.com/v1/users",
				Method:   "GET",
				Interval: 5 * time.Minute,
				Enabled:  true,
			},
			expectError: false,
		},
		{
			name: "empty ID",
			endpoint: EndpointConfig{
				ID: "",
			},
			expectError: true,
			errorMsg:    "endpoint ID cannot be empty",
		},
		{
			name: "invalid ID characters",
			endpoint: EndpointConfig{
				ID: "test endpoint!",
			},
			expectError: true,
			errorMsg:    "endpoint ID can only contain letters, numbers, underscores, and hyphens",
		},
		{
			name: "empty URL",
			endpoint: EndpointConfig{
				ID:  "test",
				URL: "",
			},
			expectError: true,
			errorMsg:    "endpoint URL cannot be empty",
		},
		{
			name: "invalid URL format",
			endpoint: EndpointConfig{
				ID:       "test",
				URL:      "not-a-url",
				Method:   "GET",
				Interval: 5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "invalid URL format",
		},
		{
			name: "invalid URL scheme",
			endpoint: EndpointConfig{
				ID:  "test",
				URL: "ftp://api.test.com/users",
			},
			expectError: true,
			errorMsg:    "URL scheme must be http or https",
		},
		{
			name: "empty method",
			endpoint: EndpointConfig{
				ID:     "test",
				URL:    "https://api.test.com/users",
				Method: "",
			},
			expectError: true,
			errorMsg:    "HTTP method cannot be empty",
		},
		{
			name: "invalid method",
			endpoint: EndpointConfig{
				ID:     "test",
				URL:    "https://api.test.com/users",
				Method: "INVALID",
			},
			expectError: true,
			errorMsg:    "invalid HTTP method",
		},
		{
			name: "negative interval",
			endpoint: EndpointConfig{
				ID:       "test",
				URL:      "https://api.test.com/users",
				Method:   "GET",
				Interval: -1 * time.Minute,
			},
			expectError: true,
			errorMsg:    "monitoring interval must be positive",
		},
		{
			name: "interval too small",
			endpoint: EndpointConfig{
				ID:       "test",
				URL:      "https://api.test.com/users",
				Method:   "GET",
				Interval: 30 * time.Second,
			},
			expectError: true,
			errorMsg:    "monitoring interval cannot be less than 1 minute",
		},
		{
			name: "interval too large",
			endpoint: EndpointConfig{
				ID:       "test",
				URL:      "https://api.test.com/users",
				Method:   "GET",
				Interval: 25 * time.Hour,
			},
			expectError: true,
			errorMsg:    "monitoring interval cannot exceed 24 hours",
		},
		{
			name: "timeout too large",
			endpoint: EndpointConfig{
				ID:       "test",
				URL:      "https://api.test.com/users",
				Method:   "GET",
				Interval: 5 * time.Minute,
				Timeout:  6 * time.Minute,
			},
			expectError: true,
			errorMsg:    "endpoint timeout cannot exceed 5 minutes",
		},
		{
			name: "negative retry count",
			endpoint: EndpointConfig{
				ID:         "test",
				URL:        "https://api.test.com/users",
				Method:     "GET",
				Interval:   5 * time.Minute,
				RetryCount: -1,
			},
			expectError: true,
			errorMsg:    "retry count cannot be negative",
		},
		{
			name: "retry count too large",
			endpoint: EndpointConfig{
				ID:         "test",
				URL:        "https://api.test.com/users",
				Method:     "GET",
				Interval:   5 * time.Minute,
				RetryCount: 11,
			},
			expectError: true,
			errorMsg:    "retry count cannot exceed 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoint(&tt.endpoint, "endpoint")
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAlerting(t *testing.T) {
	tests := []struct {
		name        string
		alerting    AlertingConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid alerting config",
			alerting: AlertingConfig{
				Enabled: true,
				Channels: []AlertChannelConfig{
					{
						Type:    "slack",
						Name:    "dev-alerts",
						Enabled: true,
						Settings: map[string]interface{}{
							"webhook_url": "https://hooks.slack.com/test",
						},
					},
				},
				Rules: []AlertRuleConfig{
					{
						Name:     "critical-alerts",
						Severity: []string{"high", "critical"},
						Channels: []string{"dev-alerts"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty channel name",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type: "slack",
						Name: "",
					},
				},
			},
			expectError: true,
			errorMsg:    "alert channel name cannot be empty",
		},
		{
			name: "duplicate channel names",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{Type: "slack", Name: "alerts"},
					{Type: "email", Name: "alerts"},
				},
			},
			expectError: true,
			errorMsg:    "duplicate alert channel name",
		},
		{
			name: "invalid channel type",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type: "invalid",
						Name: "test",
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid alert channel type",
		},
		{
			name: "slack channel missing webhook_url",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type:     "slack",
						Name:     "test",
						Settings: map[string]interface{}{},
					},
				},
			},
			expectError: true,
			errorMsg:    "Slack channel requires webhook_url setting",
		},
		{
			name: "slack channel invalid webhook_url",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type: "slack",
						Name: "test",
						Settings: map[string]interface{}{
							"webhook_url": "not-a-url",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid Slack webhook URL format",
		},
		{
			name: "email channel missing required settings",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type:     "email",
						Name:     "test",
						Settings: map[string]interface{}{},
					},
				},
			},
			expectError: true,
			errorMsg:    "email channel requires smtp_host setting",
		},
		{
			name: "webhook channel missing url",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{
						Type:     "webhook",
						Name:     "test",
						Settings: map[string]interface{}{},
					},
				},
			},
			expectError: true,
			errorMsg:    "webhook channel requires url setting",
		},
		{
			name: "empty rule name",
			alerting: AlertingConfig{
				Rules: []AlertRuleConfig{
					{
						Name: "",
					},
				},
			},
			expectError: true,
			errorMsg:    "alert rule name cannot be empty",
		},
		{
			name: "invalid severity level",
			alerting: AlertingConfig{
				Rules: []AlertRuleConfig{
					{
						Name:     "test",
						Severity: []string{"invalid"},
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid severity level",
		},
		{
			name: "rule references non-existent channel",
			alerting: AlertingConfig{
				Channels: []AlertChannelConfig{
					{Type: "slack", Name: "existing"},
				},
				Rules: []AlertRuleConfig{
					{
						Name:     "test",
						Severity: []string{"high"},
						Channels: []string{"non-existent"},
					},
				},
			},
			expectError: true,
			errorMsg:    "referenced channel 'non-existent' does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAlerting(&tt.alerting)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateReporting(t *testing.T) {
	tests := []struct {
		name        string
		reporting   ReportingConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid reporting config",
			reporting: ReportingConfig{
				RetentionDays: 30,
				ExportFormat:  "json",
				IncludeBody:   false,
			},
			expectError: false,
		},
		{
			name: "negative retention days",
			reporting: ReportingConfig{
				RetentionDays: -1,
			},
			expectError: true,
			errorMsg:    "retention days must be positive",
		},
		{
			name: "retention days too large",
			reporting: ReportingConfig{
				RetentionDays: 366,
			},
			expectError: true,
			errorMsg:    "retention days cannot exceed 365",
		},
		{
			name: "invalid export format",
			reporting: ReportingConfig{
				RetentionDays: 30,
				ExportFormat:  "invalid",
			},
			expectError: true,
			errorMsg:    "invalid export format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReporting(&tt.reporting)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	// Test ValidationError.Error()
	err := ValidationError{
		Field:   "test.field",
		Value:   "test-value",
		Message: "test message",
	}
	expected := "validation error in field 'test.field': test message (value: test-value)"
	assert.Equal(t, expected, err.Error())

	// Test ValidationErrors.Error() with multiple errors
	errors := ValidationErrors{
		ValidationError{Field: "field1", Message: "error1"},
		ValidationError{Field: "field2", Message: "error2"},
	}
	result := errors.Error()
	assert.Contains(t, result, "configuration validation failed with 2 error(s)")
	assert.Contains(t, result, "field1")
	assert.Contains(t, result, "field2")

	// Test ValidationErrors.Error() with no errors
	emptyErrors := ValidationErrors{}
	assert.Equal(t, "no validation errors", emptyErrors.Error())
}

func TestValidateConfig_DuplicateEndpointIDs(t *testing.T) {
	config := DefaultConfig()
	config.Endpoints = []EndpointConfig{
		{
			ID:       "duplicate-id",
			URL:      "https://api1.test.com/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
		},
		{
			ID:       "duplicate-id",
			URL:      "https://api2.test.com/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
		},
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate endpoint ID")
}
