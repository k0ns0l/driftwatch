package alerting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      config.AlertChannelConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": 587,
					"from":      "alerts@example.com",
					"to":        []string{"admin@example.com"},
					"username":  "alerts@example.com",
					"password":  "secret",
				},
			},
			expectError: false,
		},
		{
			name: "missing smtp_host",
			config: config.AlertChannelConfig{
				Type:     "email",
				Name:     "test-email",
				Enabled:  true,
				Settings: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "smtp_host is required",
		},
		{
			name: "missing smtp_port",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
				},
			},
			expectError: true,
			errorMsg:    "smtp_port is required",
		},
		{
			name: "invalid smtp_port type",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": "invalid",
				},
			},
			expectError: true,
			errorMsg:    "invalid smtp_port",
		},
		{
			name: "missing from address",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": 587,
				},
			},
			expectError: true,
			errorMsg:    "from address is required",
		},
		{
			name: "missing to addresses",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": 587,
					"from":      "alerts@example.com",
				},
			},
			expectError: true,
			errorMsg:    "to addresses are required",
		},
		{
			name: "single to address as string",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": 587,
					"from":      "alerts@example.com",
					"to":        "admin@example.com",
				},
			},
			expectError: false,
		},
		{
			name: "port as string",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": "587",
					"from":      "alerts@example.com",
					"to":        "admin@example.com",
				},
			},
			expectError: false,
		},
		{
			name: "port as float64",
			config: config.AlertChannelConfig{
				Type:    "email",
				Name:    "test-email",
				Enabled: true,
				Settings: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": 587.0,
					"from":      "alerts@example.com",
					"to":        "admin@example.com",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := NewEmailChannel(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, channel)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, channel)

				emailChannel := channel.(*EmailChannel)
				assert.Equal(t, tt.config.Name, emailChannel.GetName())
				assert.Equal(t, "email", emailChannel.GetType())
				assert.Equal(t, tt.config.Enabled, emailChannel.IsEnabled())
			}
		})
	}
}

func TestEmailChannelBuildMessage(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "email",
		Name:    "test-email",
		Enabled: true,
		Settings: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"from":      "alerts@example.com",
			"to":        []string{"admin@example.com", "dev@example.com"},
		},
	}

	channel, err := NewEmailChannel(channelConfig)
	require.NoError(t, err)

	emailChannel := channel.(*EmailChannel)

	subject := "Test Alert"
	body := "<html><body>Test body</body></html>"

	message := emailChannel.buildEmailMessage(subject, body)

	// Verify headers
	assert.Contains(t, message, "From: alerts@example.com")
	assert.Contains(t, message, "To: admin@example.com, dev@example.com")
	assert.Contains(t, message, "Subject: Test Alert")
	assert.Contains(t, message, "MIME-Version: 1.0")
	assert.Contains(t, message, "Content-Type: text/html; charset=UTF-8")

	// Verify body
	assert.Contains(t, message, "<html><body>Test body</body></html>")
}

func TestEmailFormatMessage(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "email",
		Name:    "test-email",
		Enabled: true,
		Settings: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"from":      "alerts@example.com",
			"to":        "admin@example.com",
		},
	}

	channel, err := NewEmailChannel(channelConfig)
	require.NoError(t, err)

	emailChannel := channel.(*EmailChannel)

	message := &AlertMessage{
		Title:       "API Drift Detected",
		Summary:     "Critical changes detected in user API",
		Severity:    "critical",
		EndpointID:  "users-api",
		EndpointURL: "https://api.example.com/users",
		DetectedAt:  time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC),
		Changes: []ChangeDetail{
			{
				Type:        "field_removed",
				Path:        "$.user.email",
				Description: "Email field was removed",
				Severity:    "critical",
				Breaking:    true,
				OldValue:    "test@example.com",
			},
			{
				Type:        "field_added",
				Path:        "$.user.phone",
				Description: "Phone field was added",
				Severity:    "low",
				Breaking:    false,
				NewValue:    "+1234567890",
			},
		},
	}

	html := emailChannel.formatMessage(message)

	// Verify HTML structure
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "<html>")
	assert.Contains(t, html, "</html>")

	// Verify content
	assert.Contains(t, html, "API Drift Detected")
	assert.Contains(t, html, "Critical changes detected in user API")
	assert.Contains(t, html, "https://api.example.com/users")
	assert.Contains(t, html, "users-api")
	assert.Contains(t, html, "2023-12-01 12:00:00 UTC")

	// Verify severity styling
	assert.Contains(t, html, "severity-critical")

	// Verify changes section
	assert.Contains(t, html, "Changes Detected")
	assert.Contains(t, html, "field_removed")
	assert.Contains(t, html, "$.user.email")
	assert.Contains(t, html, "[BREAKING]")
	assert.Contains(t, html, "test@example.com")
	assert.Contains(t, html, "+1234567890")

	// Verify CSS classes
	assert.Contains(t, html, "breaking")
	assert.Contains(t, html, "change-item")
}

func TestEmailFormatSeverity(t *testing.T) {
	channel := &EmailChannel{}

	tests := []struct {
		severity string
		expected string
	}{
		{"critical", "🚨 Critical"},
		{"high", "⚠️ High"},
		{"medium", "ℹ️ Medium"},
		{"low", "⚪ Low"},
		{"unknown", "❓ unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			formatted := channel.formatSeverity(tt.severity)
			assert.Equal(t, tt.expected, formatted)
		})
	}
}

func TestEmailChannelTest(t *testing.T) {
	// Note: This test doesn't actually send an email since we don't have a real SMTP server
	// In a real scenario, you might use a mock SMTP server or skip this test in unit tests

	channelConfig := config.AlertChannelConfig{
		Type:    "email",
		Name:    "test-email",
		Enabled: true,
		Settings: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"from":      "alerts@example.com",
			"to":        "admin@example.com",
		},
	}

	channel, err := NewEmailChannel(channelConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// This will fail because we don't have a real SMTP server
	// but we can verify the channel was created correctly
	err = channel.Test(ctx)
	assert.Error(t, err) // Expected to fail without real SMTP server
}

func TestEmailChannelSendTimeout(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "email",
		Name:    "test-email",
		Enabled: true,
		Settings: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"from":      "alerts@example.com",
			"to":        "admin@example.com",
		},
	}

	channel, err := NewEmailChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = channel.Send(ctx, message)
	assert.Error(t, err)
	// The error could be context deadline exceeded or connection failure
	// Both are acceptable for this test
}

func TestEmailChannelLongChanges(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "email",
		Name:    "test-email",
		Enabled: true,
		Settings: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"from":      "alerts@example.com",
			"to":        "admin@example.com",
		},
	}

	channel, err := NewEmailChannel(channelConfig)
	require.NoError(t, err)

	emailChannel := channel.(*EmailChannel)

	// Create message with many changes
	changes := make([]ChangeDetail, 10)
	for i := 0; i < 10; i++ {
		changes[i] = ChangeDetail{
			Type:        "field_modified",
			Path:        fmt.Sprintf("$.field_%d", i),
			Description: fmt.Sprintf("Field %d was modified", i),
			Severity:    "medium",
			Breaking:    false,
		}
	}

	message := &AlertMessage{
		Title:       "API Drift Detected",
		Summary:     "Multiple changes detected",
		Severity:    "medium",
		EndpointID:  "test-api",
		EndpointURL: "https://api.example.com/test",
		DetectedAt:  time.Now(),
		Changes:     changes,
	}

	html := emailChannel.formatMessage(message)

	// Verify all changes are included
	for i := 0; i < 10; i++ {
		assert.Contains(t, html, fmt.Sprintf("$.field_%d", i))
		assert.Contains(t, html, fmt.Sprintf("Field %d was modified", i))
	}
}
