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

// SlackChannel implements AlertChannel for Slack webhook integration
type SlackChannel struct {
	name       string
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	enabled    bool
	client     *http.Client
}

// SlackMessage represents a Slack webhook message
type SlackMessage struct {
	Channel   string       `json:"channel,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
	Text      string       `json:"text,omitempty"`
	Blocks    []SlackBlock `json:"blocks,omitempty"`
}

// SlackBlock represents a Slack block element
type SlackBlock struct {
	Type   string       `json:"type"`
	Text   *SlackText   `json:"text,omitempty"`
	Fields []SlackField `json:"fields,omitempty"`
}

// SlackText represents Slack text element
type SlackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackField represents a Slack field element
type SlackField struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewSlackChannel creates a new Slack alert channel
func NewSlackChannel(channelConfig config.AlertChannelConfig) (AlertChannel, error) {
	settings := channelConfig.Settings

	webhookURL, ok := settings["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required for Slack channel")
	}

	channel := &SlackChannel{
		name:       channelConfig.Name,
		webhookURL: webhookURL,
		enabled:    channelConfig.Enabled,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Optional settings
	if ch, ok := settings["channel"].(string); ok {
		channel.channel = ch
	}
	if username, ok := settings["username"].(string); ok {
		channel.username = username
	} else {
		channel.username = "DriftWatch"
	}
	if iconEmoji, ok := settings["icon_emoji"].(string); ok {
		channel.iconEmoji = iconEmoji
	} else {
		channel.iconEmoji = ":warning:"
	}

	return channel, nil
}

// Send sends an alert message to Slack
func (sc *SlackChannel) Send(ctx context.Context, message *AlertMessage) error {
	slackMessage := sc.formatMessage(message)

	payload, err := json.Marshal(slackMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sc.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := sc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test message to verify the Slack configuration
func (sc *SlackChannel) Test(ctx context.Context) error {
	testMessage := &AlertMessage{
		Title:       "DriftWatch Test Alert",
		Summary:     "This is a test message to verify Slack integration is working correctly.",
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

	return sc.Send(ctx, testMessage)
}

// GetType returns the channel type
func (sc *SlackChannel) GetType() string {
	return "slack"
}

// GetName returns the channel name
func (sc *SlackChannel) GetName() string {
	return sc.name
}

// IsEnabled returns whether the channel is enabled
func (sc *SlackChannel) IsEnabled() bool {
	return sc.enabled
}

// formatMessage formats an AlertMessage for Slack
func (sc *SlackChannel) formatMessage(message *AlertMessage) *SlackMessage {
	// Choose emoji based on severity
	emoji := sc.getSeverityEmoji(message.Severity)

	// Create the main text
	text := fmt.Sprintf("%s *%s*", emoji, message.Title)

	// Create blocks for rich formatting
	blocks := []SlackBlock{
		{
			Type: "section",
			Text: &SlackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("%s *%s*\n%s", emoji, message.Title, message.Summary),
			},
		},
		{
			Type: "section",
			Fields: []SlackField{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Endpoint:*\n%s", message.EndpointURL),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Severity:*\n%s", sc.formatSeverity(message.Severity)),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Detected:*\n%s", message.DetectedAt.Format("2006-01-02 15:04:05 UTC")),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Endpoint ID:*\n%s", message.EndpointID),
				},
			},
		},
	}

	// Add changes details if present
	if len(message.Changes) > 0 {
		changesText := "*Changes Detected:*\n"
		for i, change := range message.Changes {
			if i >= 5 { // Limit to first 5 changes to avoid message being too long
				changesText += fmt.Sprintf("... and %d more changes\n", len(message.Changes)-i)
				break
			}

			breakingIndicator := ""
			if change.Breaking {
				breakingIndicator = " :exclamation:"
			}

			changesText += fmt.Sprintf("• *%s* at `%s`%s\n", change.Type, change.Path, breakingIndicator)
			if change.Description != "" {
				changesText += fmt.Sprintf("  %s\n", change.Description)
			}
		}

		blocks = append(blocks, SlackBlock{
			Type: "section",
			Text: &SlackText{
				Type: "mrkdwn",
				Text: changesText,
			},
		})
	}

	slackMessage := &SlackMessage{
		Username:  sc.username,
		IconEmoji: sc.iconEmoji,
		Text:      text, // Fallback text for notifications
		Blocks:    blocks,
	}

	// Set channel if specified
	if sc.channel != "" {
		slackMessage.Channel = sc.channel
	}

	return slackMessage
}

// getSeverityEmoji returns an appropriate emoji for the severity level
func (sc *SlackChannel) getSeverityEmoji(severity string) string {
	switch severity {
	case "critical":
		return ":rotating_light:"
	case "high":
		return ":warning:"
	case "medium":
		return ":information_source:"
	case "low":
		return ":white_circle:"
	default:
		return ":question:"
	}
}

// formatSeverity formats the severity for display
func (sc *SlackChannel) formatSeverity(severity string) string {
	switch severity {
	case "critical":
		return ":rotating_light: Critical"
	case "high":
		return ":warning: High"
	case "medium":
		return ":information_source: Medium"
	case "low":
		return ":white_circle: Low"
	default:
		return fmt.Sprintf(":question: %s", severity)
	}
}
