package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
)

// WebhookChannel implements AlertChannel for generic webhook integration
type WebhookChannel struct {
	name    string
	url     string
	method  string
	headers map[string]string
	enabled bool
	client  *http.Client
}

// WebhookPayload represents the payload sent to webhook endpoints
type WebhookPayload struct {
	Alert     *AlertMessage          `json:"alert"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewWebhookChannel creates a new webhook alert channel
func NewWebhookChannel(channelConfig config.AlertChannelConfig) (AlertChannel, error) {
	settings := channelConfig.Settings

	url, ok := settings["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required for webhook channel")
	}

	channel := &WebhookChannel{
		name:    channelConfig.Name,
		url:     url,
		method:  "POST", // Default method
		headers: make(map[string]string),
		enabled: channelConfig.Enabled,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Optional settings
	if method, ok := settings["method"].(string); ok {
		channel.method = method
	}

	// Parse headers
	if headersInterface, ok := settings["headers"]; ok {
		if headersMap, ok := headersInterface.(map[string]interface{}); ok {
			for key, value := range headersMap {
				if strValue, ok := value.(string); ok {
					channel.headers[key] = strValue
				}
			}
		}
	}

	// Set default Content-Type if not specified
	if _, exists := channel.headers["Content-Type"]; !exists {
		channel.headers["Content-Type"] = "application/json"
	}

	return channel, nil
}

// Send sends an alert message to the webhook endpoint
func (wc *WebhookChannel) Send(ctx context.Context, message *AlertMessage) error {
	payload := &WebhookPayload{
		Alert:     message,
		Timestamp: time.Now(),
		Source:    "driftwatch",
		Version:   "1.0.0", // This could be made configurable
		Metadata: map[string]interface{}{
			"channel_name": wc.name,
			"channel_type": "webhook",
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, wc.method, wc.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range wc.headers {
		req.Header.Set(key, value)
	}

	resp, err := wc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful response (2xx status codes)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test message to verify the webhook configuration
func (wc *WebhookChannel) Test(ctx context.Context) error {
	testMessage := &AlertMessage{
		Title:       "DriftWatch Test Alert",
		Summary:     "This is a test message to verify webhook integration is working correctly.",
		Severity:    "low",
		EndpointID:  "test-endpoint",
		EndpointURL: "https://api.example.com/test",
		DetectedAt:  time.Now(),
		Changes: []ChangeDetail{
			{
				Type:        "test_change",
				Path:        "$.test.field",
				Description: "Test change for configuration verification",
				Severity:    "low",
				Breaking:    false,
			},
		},
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	return wc.Send(ctx, testMessage)
}

// GetType returns the channel type
func (wc *WebhookChannel) GetType() string {
	return "webhook"
}

// GetName returns the channel name
func (wc *WebhookChannel) GetName() string {
	return wc.name
}

// IsEnabled returns whether the channel is enabled
func (wc *WebhookChannel) IsEnabled() bool {
	return wc.enabled
}
