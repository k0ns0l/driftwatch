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

func TestNewWebhookChannel(t *testing.T) {
	tests := []struct {
		name        string
		config      config.AlertChannelConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: config.AlertChannelConfig{
				Type:    "webhook",
				Name:    "test-webhook",
				Enabled: true,
				Settings: map[string]interface{}{
					"url":    "https://api.example.com/webhook",
					"method": "POST",
					"headers": map[string]interface{}{
						"Authorization": "Bearer token123",
						"X-Custom":      "value",
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing url",
			config: config.AlertChannelConfig{
				Type:     "webhook",
				Name:     "test-webhook",
				Enabled:  true,
				Settings: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "url is required",
		},
		{
			name: "empty url",
			config: config.AlertChannelConfig{
				Type:    "webhook",
				Name:    "test-webhook",
				Enabled: true,
				Settings: map[string]interface{}{
					"url": "",
				},
			},
			expectError: true,
			errorMsg:    "url is required",
		},
		{
			name: "minimal valid configuration",
			config: config.AlertChannelConfig{
				Type:    "webhook",
				Name:    "test-webhook",
				Enabled: true,
				Settings: map[string]interface{}{
					"url": "https://api.example.com/webhook",
				},
			},
			expectError: false,
		},
		{
			name: "custom method",
			config: config.AlertChannelConfig{
				Type:    "webhook",
				Name:    "test-webhook",
				Enabled: true,
				Settings: map[string]interface{}{
					"url":    "https://api.example.com/webhook",
					"method": "PUT",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := NewWebhookChannel(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, channel)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, channel)

				webhookChannel := channel.(*WebhookChannel)
				assert.Equal(t, tt.config.Name, webhookChannel.GetName())
				assert.Equal(t, "webhook", webhookChannel.GetType())
				assert.Equal(t, tt.config.Enabled, webhookChannel.IsEnabled())

				// Check method default
				if method, ok := tt.config.Settings["method"]; ok {
					assert.Equal(t, method.(string), webhookChannel.method)
				} else {
					assert.Equal(t, "POST", webhookChannel.method)
				}
			}
		})
	}
}

func TestWebhookChannelSend(t *testing.T) {
	// Create a test server to mock webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))

		// Parse the request body
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)

		// Verify payload structure
		assert.NotNil(t, payload.Alert)
		assert.Equal(t, "driftwatch", payload.Source)
		assert.Equal(t, "1.0.0", payload.Version)
		assert.NotEmpty(t, payload.Timestamp)
		assert.NotNil(t, payload.Metadata)

		// Verify alert content
		assert.Equal(t, "Test Alert", payload.Alert.Title)
		assert.Equal(t, "This is a test alert", payload.Alert.Summary)
		assert.Equal(t, "high", payload.Alert.Severity)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook channel with test server URL
	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
			"headers": map[string]interface{}{
				"Authorization": "Bearer token123",
				"X-Custom":      "custom-value",
			},
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
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

func TestWebhookChannelSendCustomMethod(t *testing.T) {
	// Create a test server that expects PUT method
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url":    server.URL,
			"method": "PUT",
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.NoError(t, err)
}

func TestWebhookChannelSendFailure(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{"success 200", http.StatusOK, false},
		{"success 201", http.StatusCreated, false},
		{"success 204", http.StatusNoContent, false},
		{"client error 400", http.StatusBadRequest, true},
		{"client error 404", http.StatusNotFound, true},
		{"server error 500", http.StatusInternalServerError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			channelConfig := config.AlertChannelConfig{
				Type:    "webhook",
				Name:    "test-webhook",
				Enabled: true,
				Settings: map[string]interface{}{
					"url": server.URL,
				},
			}

			channel, err := NewWebhookChannel(channelConfig)
			require.NoError(t, err)

			message := &AlertMessage{
				Title:   "Test Alert",
				Summary: "This is a test alert",
			}

			ctx := context.Background()
			err = channel.Send(ctx, message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "status")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookChannelTest(t *testing.T) {
	// Create a test server to mock webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)

		// Verify it's a test message
		assert.Contains(t, payload.Alert.Title, "Test Alert")
		assert.True(t, payload.Alert.Metadata["test"].(bool))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = channel.Test(ctx)
	assert.NoError(t, err)
}

func TestWebhookChannelTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than context timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
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

func TestWebhookChannelDefaultHeaders(t *testing.T) {
	// Create a test server to verify default headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify default Content-Type header is set
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
			// No headers specified, should use defaults
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.NoError(t, err)
}

func TestWebhookChannelCustomContentType(t *testing.T) {
	// Create a test server to verify custom Content-Type header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/custom+json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
			"headers": map[string]interface{}{
				"Content-Type": "application/custom+json",
			},
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:   "Test Alert",
		Summary: "This is a test alert",
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	assert.NoError(t, err)
}

func TestWebhookPayloadStructure(t *testing.T) {
	// Create a test server to capture and verify payload structure
	var capturedPayload WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&capturedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channelConfig := config.AlertChannelConfig{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": server.URL,
		},
	}

	channel, err := NewWebhookChannel(channelConfig)
	require.NoError(t, err)

	message := &AlertMessage{
		Title:       "API Drift Detected",
		Summary:     "Critical changes detected",
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
		},
		Metadata: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}

	ctx := context.Background()
	err = channel.Send(ctx, message)
	require.NoError(t, err)

	// Verify payload structure
	assert.Equal(t, "driftwatch", capturedPayload.Source)
	assert.Equal(t, "1.0.0", capturedPayload.Version)
	assert.NotZero(t, capturedPayload.Timestamp)

	// Verify metadata
	assert.Equal(t, "test-webhook", capturedPayload.Metadata["channel_name"])
	assert.Equal(t, "webhook", capturedPayload.Metadata["channel_type"])

	// Verify alert content
	assert.Equal(t, message.Title, capturedPayload.Alert.Title)
	assert.Equal(t, message.Summary, capturedPayload.Alert.Summary)
	assert.Equal(t, message.Severity, capturedPayload.Alert.Severity)
	assert.Equal(t, message.EndpointID, capturedPayload.Alert.EndpointID)
	assert.Equal(t, message.EndpointURL, capturedPayload.Alert.EndpointURL)
	assert.Equal(t, message.DetectedAt, capturedPayload.Alert.DetectedAt)
	assert.Len(t, capturedPayload.Alert.Changes, 1)
	assert.Equal(t, message.Changes[0].Type, capturedPayload.Alert.Changes[0].Type)
	assert.Equal(t, message.Changes[0].Path, capturedPayload.Alert.Changes[0].Path)
	assert.Equal(t, message.Changes[0].Breaking, capturedPayload.Alert.Changes[0].Breaking)
}
