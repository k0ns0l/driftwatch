package alerting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlackChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      config.AlertChannelConfig
		expectError bool
	}{
		{
			name: "valid configuration",
			config: config.AlertChannelConfig{
				Type:    "slack",
				Name:    "test-slack",
				Enabled: true,
				Settings: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/test",
					"channel":     "#alerts",
					"username":    "DriftWatch",
					"icon_emoji":  ":warning:",
				},
			},
			expectError: false,
		},
		{
			name: "missing webhook_url",
			config: config.AlertChannelConfig{
				Type:     "slack",
				Name:     "test-slack",
				Enabled:  true,
				Settings: map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "empty webhook_url",
			config: config.AlertChannelConfig{
				Type:    "slack",
				Name:    "test-slack",
				Enabled: true,
				Settings: map[string]interface{}{
					"webhook_url": "",
				},
			},
			expectError: true,
		},
		{
			name: "minimal valid configuration",
			config: config.AlertChannelConfig{
				Type:    "slack",
				Name:    "test-slack",
				Enabled: true,
				Settings: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/test",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := NewSlackChannel(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, channel)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, channel)

				slackChannel := channel.(*SlackChannel)
				assert.Equal(t, tt.config.Name, slackChannel.GetName())
				assert.Equal(t, "slack", slackChannel.GetType())
				assert.Equal(t, tt.config.Enabled, slackChannel.IsEnabled())
			}
		})
	}
}

func TestSlackChannelSend(t *testing.T) {
	// Create a test server to mock Slack webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse the request body
		var slackMessage SlackMessage
		err := json.NewDecoder(r.Body).Decode(&slackMessage)
		assert.NoError(t, err)

		// Verify message structure
		assert.NotEmpty(t, slackMessage.Text)
		assert.NotEmpty(t, slackMessage.Blocks)
		assert.Equal(t, "DriftWatch", slackMessage.Username)
		assert.Equal(t, ":warning:", slackMessage.IconEmoji)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create Slack channel with test server URL
	channelConfig := config.AlertChannelConfig{
		Type:    "slack",
		Name:    "test-slack",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewSlackChannel(channelConfig)
	require.NoError(t, err)

	// Create test alert message
	message := &AlertMessage{
		Title:       "Test Alert",
		Summary:     "This is a test alert",
		Severity:    "high",
		EndpointID:  "test-endpoint",
		EndpointURL: "https://api.example.com/test",
		DetectedAt:  time.Now(),
		Changes: []ChangeDetail{
			{
				Type:        "field_removed",
				Path:        "$.user.email",
				Description: "Email field was removed",
				Severity:    "high",
				Breaking:    true,
			},
		},
	}

	// Send the message
	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.NoError(t, err)
}

func TestSlackChannelSendFailure(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "slack",
		Name:    "test-slack",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewSlackChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestSlackChannelTest(t *testing.T) {
	// Create a test server to mock Slack webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var slackMessage SlackMessage
		err := json.NewDecoder(r.Body).Decode(&slackMessage)
		assert.NoError(t, err)

		// Verify it's a test message
		assert.Contains(t, slackMessage.Text, "Test Alert")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "slack",
		Name:    "test-slack",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewSlackChannel(channelConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = channel.Test(ctx)
	assert.NoError(t, err)
}

func TestSlackFormatMessage(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "slack",
		Name:    "test-slack",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": "https://hooks.slack.com/test",
			"channel":     "#alerts",
		},
	}

	channel, err := NewSlackChannel(channelConfig)
	require.NoError(t, err)

	slackChannel := channel.(*SlackChannel)

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

	slackMessage := slackChannel.formatMessage(message)

	// Verify basic structure
	assert.Equal(t, "#alerts", slackMessage.Channel)
	assert.Equal(t, "DriftWatch", slackMessage.Username)
	assert.Equal(t, ":warning:", slackMessage.IconEmoji)
	assert.Contains(t, slackMessage.Text, ":rotating_light:")
	assert.Contains(t, slackMessage.Text, "API Drift Detected")

	// Verify blocks structure
	assert.NotEmpty(t, slackMessage.Blocks)

	// First block should be the header
	headerBlock := slackMessage.Blocks[0]
	assert.Equal(t, "section", headerBlock.Type)
	assert.NotNil(t, headerBlock.Text)
	assert.Contains(t, headerBlock.Text.Text, "API Drift Detected")

	// Second block should be details
	detailsBlock := slackMessage.Blocks[1]
	assert.Equal(t, "section", detailsBlock.Type)
	assert.NotEmpty(t, detailsBlock.Fields)
	assert.Len(t, detailsBlock.Fields, 4) // Endpoint, Severity, Detected, Endpoint ID

	// Third block should be changes
	changesBlock := slackMessage.Blocks[2]
	assert.Equal(t, "section", changesBlock.Type)
	assert.NotNil(t, changesBlock.Text)
	assert.Contains(t, changesBlock.Text.Text, "Changes Detected")
	assert.Contains(t, changesBlock.Text.Text, "field_removed")
	assert.Contains(t, changesBlock.Text.Text, ":exclamation:") // Breaking change indicator
}

func TestSlackGetSeverityEmoji(t *testing.T) {
	channel := &SlackChannel{}

	tests := []struct {
		severity string
		expected string
	}{
		{"critical", ":rotating_light:"},
		{"high", ":warning:"},
		{"medium", ":information_source:"},
		{"low", ":white_circle:"},
		{"unknown", ":question:"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			emoji := channel.getSeverityEmoji(tt.severity)
			assert.Equal(t, tt.expected, emoji)
		})
	}
}

func TestSlackFormatSeverity(t *testing.T) {
	channel := &SlackChannel{}

	tests := []struct {
		severity string
		expected string
	}{
		{"critical", ":rotating_light: Critical"},
		{"high", ":warning: High"},
		{"medium", ":information_source: Medium"},
		{"low", ":white_circle: Low"},
		{"unknown", ":question: unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			formatted := channel.formatSeverity(tt.severity)
			assert.Equal(t, tt.expected, formatted)
		})
	}
}

func TestSlackChannelTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than context timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "slack",
		Name:    "test-slack",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewSlackChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = channel.Send(ctx, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
