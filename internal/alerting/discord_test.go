package alerting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiscordChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      config.AlertChannelConfig
		expectError bool
	}{
		{
			name: "valid configuration",
			config: config.AlertChannelConfig{
				Type:    "discord",
				Name:    "test-discord",
				Enabled: true,
				Settings: map[string]interface{}{
					"webhook_url": "https://discord.com/api/webhooks/123/test",
					"username":    "DriftWatch Bot",
					"avatar_url":  "https://example.com/avatar.png",
				},
			},
			expectError: false,
		},
		{
			name: "missing webhook_url",
			config: config.AlertChannelConfig{
				Type:     "discord",
				Name:     "test-discord",
				Enabled:  true,
				Settings: map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "empty webhook_url",
			config: config.AlertChannelConfig{
				Type:    "discord",
				Name:    "test-discord",
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
				Type:    "discord",
				Name:    "test-discord",
				Enabled: true,
				Settings: map[string]interface{}{
					"webhook_url": "https://discord.com/api/webhooks/123/test",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := NewDiscordChannel(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, channel)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, channel)

				discordChannel := channel.(*DiscordChannel)
				assert.Equal(t, tt.config.Name, discordChannel.GetName())
				assert.Equal(t, "discord", discordChannel.GetType())
				assert.Equal(t, tt.config.Enabled, discordChannel.IsEnabled())
			}
		})
	}
}

func TestDiscordChannelSend(t *testing.T) {
	// Create a test server to mock Discord webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse the request body
		var discordMessage DiscordMessage
		err := json.NewDecoder(r.Body).Decode(&discordMessage)
		assert.NoError(t, err)

		// Verify message structure
		assert.Equal(t, "DriftWatch", discordMessage.Username)
		assert.NotEmpty(t, discordMessage.Embeds)

		embed := discordMessage.Embeds[0]
		assert.Equal(t, "Test Alert", embed.Title)
		assert.Equal(t, "This is a test alert", embed.Description)
		assert.Equal(t, 0xFF8C00, embed.Color) // High severity color
		assert.NotEmpty(t, embed.Fields)
		assert.NotNil(t, embed.Footer)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create Discord channel with test server URL
	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
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

func TestDiscordChannelSendFailure(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func TestDiscordChannelTest(t *testing.T) {
	// Create a test server to mock Discord webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var discordMessage DiscordMessage
		err := json.NewDecoder(r.Body).Decode(&discordMessage)
		assert.NoError(t, err)

		// Verify it's a test message
		assert.Contains(t, discordMessage.Embeds[0].Title, "Test Alert")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = channel.Test(ctx)
	assert.NoError(t, err)
}

func TestDiscordFormatMessage(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": "https://discord.com/api/webhooks/123/test",
			"username":    "Custom Bot",
			"avatar_url":  "https://example.com/avatar.png",
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
	require.NoError(t, err)

	discordChannel := channel.(*DiscordChannel)

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

	discordMessage := discordChannel.formatMessage(message)

	// Verify basic structure
	assert.Equal(t, "Custom Bot", discordMessage.Username)
	assert.Equal(t, "https://example.com/avatar.png", discordMessage.AvatarURL)
	assert.Len(t, discordMessage.Embeds, 1)

	embed := discordMessage.Embeds[0]
	assert.Equal(t, "API Drift Detected", embed.Title)
	assert.Equal(t, "Critical changes detected in user API", embed.Description)
	assert.Equal(t, 0xFF0000, embed.Color) // Critical severity color (red)
	assert.Equal(t, "2023-12-01T12:00:00Z", embed.Timestamp)
	assert.NotNil(t, embed.Footer)
	assert.Equal(t, "DriftWatch API Monitoring", embed.Footer.Text)

	// Verify fields
	assert.Len(t, embed.Fields, 4) // Endpoint, Severity, Endpoint ID, Changes

	// Check endpoint field
	assert.Equal(t, "Endpoint", embed.Fields[0].Name)
	assert.Equal(t, "https://api.example.com/users", embed.Fields[0].Value)

	// Check severity field
	assert.Equal(t, "Severity", embed.Fields[1].Name)
	assert.Equal(t, "🚨 Critical", embed.Fields[1].Value)

	// Check endpoint ID field
	assert.Equal(t, "Endpoint ID", embed.Fields[2].Name)
	assert.Equal(t, "users-api", embed.Fields[2].Value)

	// Check changes field
	assert.Equal(t, "Changes Detected", embed.Fields[3].Name)
	assert.Contains(t, embed.Fields[3].Value, "field_removed")
	assert.Contains(t, embed.Fields[3].Value, "$.user.email")
	assert.Contains(t, embed.Fields[3].Value, "⚠️") // Breaking change indicator
	assert.Contains(t, embed.Fields[3].Value, "field_added")
	assert.Contains(t, embed.Fields[3].Value, "$.user.phone")
}

func TestDiscordGetSeverityColor(t *testing.T) {
	channel := &DiscordChannel{}

	tests := []struct {
		severity string
		expected int
	}{
		{"critical", 0xFF0000},
		{"high", 0xFF8C00},
		{"medium", 0xFFD700},
		{"low", 0x32CD32},
		{"unknown", 0x808080},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			color := channel.getSeverityColor(tt.severity)
			assert.Equal(t, tt.expected, color)
		})
	}
}

func TestDiscordFormatSeverity(t *testing.T) {
	channel := &DiscordChannel{}

	tests := []struct {
		severity string
		expected string
	}{
		{"critical", "🚨 Critical"},
		{"high", "⚠️ High"},
		{"medium", "ℹ️ Medium"},
		{"low", "✅ Low"},
		{"unknown", "❓ unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			formatted := channel.formatSeverity(tt.severity)
			assert.Equal(t, tt.expected, formatted)
		})
	}
}

func TestDiscordChannelTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than context timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": server.URL,
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
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

func TestDiscordChannelManyChanges(t *testing.T) {
	channelConfig := config.AlertChannelConfig{
		Type:    "discord",
		Name:    "test-discord",
		Enabled: true,
		Settings: map[string]interface{}{
			"webhook_url": "https://discord.com/api/webhooks/123/test",
		},
	}

	channel, err := NewDiscordChannel(channelConfig)
	require.NoError(t, err)

	discordChannel := channel.(*DiscordChannel)

	// Create message with many changes (more than the limit of 5)
	changes := make([]ChangeDetail, 8)
	for i := 0; i < 8; i++ {
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

	discordMessage := discordChannel.formatMessage(message)

	// Find the changes field
	var changesField *DiscordEmbedField
	for _, field := range discordMessage.Embeds[0].Fields {
		if field.Name == "Changes Detected" {
			changesField = &field
			break
		}
	}

	require.NotNil(t, changesField)

	// Should include first 5 changes and indicate there are more
	for i := 0; i < 5; i++ {
		assert.Contains(t, changesField.Value, fmt.Sprintf("$.field_%d", i))
	}
	assert.Contains(t, changesField.Value, "... and 3 more changes")
}
