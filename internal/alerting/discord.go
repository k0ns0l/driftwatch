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

// DiscordChannel implements AlertChannel for Discord webhook integration
type DiscordChannel struct {
	name       string
	webhookURL string
	username   string
	avatarURL  string
	enabled    bool
	client     *http.Client
}

// DiscordMessage represents a Discord webhook message
type DiscordMessage struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents a footer in a Discord embed
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// NewDiscordChannel creates a new Discord alert channel
func NewDiscordChannel(channelConfig config.AlertChannelConfig) (AlertChannel, error) {
	settings := channelConfig.Settings

	webhookURL, ok := settings["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required for Discord channel")
	}

	channel := &DiscordChannel{
		name:       channelConfig.Name,
		webhookURL: webhookURL,
		enabled:    channelConfig.Enabled,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Optional settings
	if username, ok := settings["username"].(string); ok {
		channel.username = username
	} else {
		channel.username = "DriftWatch"
	}

	if avatarURL, ok := settings["avatar_url"].(string); ok {
		channel.avatarURL = avatarURL
	}

	return channel, nil
}

// Send sends an alert message to Discord
func (dc *DiscordChannel) Send(ctx context.Context, message *AlertMessage) error {
	discordMessage := dc.formatMessage(message)

	payload, err := json.Marshal(discordMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dc.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := dc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test message to verify the Discord configuration
func (dc *DiscordChannel) Test(ctx context.Context) error {
	testMessage := &AlertMessage{
		Title:       "DriftWatch Test Alert",
		Summary:     "This is a test message to verify Discord integration is working correctly.",
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

	return dc.Send(ctx, testMessage)
}

// GetType returns the channel type
func (dc *DiscordChannel) GetType() string {
	return "discord"
}

// GetName returns the channel name
func (dc *DiscordChannel) GetName() string {
	return dc.name
}

// IsEnabled returns whether the channel is enabled
func (dc *DiscordChannel) IsEnabled() bool {
	return dc.enabled
}

// formatMessage formats an AlertMessage for Discord
func (dc *DiscordChannel) formatMessage(message *AlertMessage) *DiscordMessage {
	// Create the main embed
	embed := DiscordEmbed{
		Title:       message.Title,
		Description: message.Summary,
		Color:       dc.getSeverityColor(message.Severity),
		Timestamp:   message.DetectedAt.Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "DriftWatch API Monitoring",
		},
	}

	// Add basic information fields
	embed.Fields = []DiscordEmbedField{
		{
			Name:   "Endpoint",
			Value:  message.EndpointURL,
			Inline: false,
		},
		{
			Name:   "Severity",
			Value:  dc.formatSeverity(message.Severity),
			Inline: true,
		},
		{
			Name:   "Endpoint ID",
			Value:  message.EndpointID,
			Inline: true,
		},
	}

	// Add changes information if present
	if len(message.Changes) > 0 {
		changesText := ""
		for i, change := range message.Changes {
			if i >= 5 { // Limit to first 5 changes to avoid message being too long
				changesText += fmt.Sprintf("... and %d more changes", len(message.Changes)-i)
				break
			}

			breakingIndicator := ""
			if change.Breaking {
				breakingIndicator = " ⚠️"
			}

			changesText += fmt.Sprintf("**%s** at `%s`%s\n", change.Type, change.Path, breakingIndicator)
			if change.Description != "" {
				changesText += fmt.Sprintf("%s\n", change.Description)
			}
			if i < len(message.Changes)-1 && i < 4 {
				changesText += "\n"
			}
		}

		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Changes Detected",
			Value:  changesText,
			Inline: false,
		})
	}

	discordMessage := &DiscordMessage{
		Username:  dc.username,
		AvatarURL: dc.avatarURL,
		Embeds:    []DiscordEmbed{embed},
	}

	return discordMessage
}

// getSeverityColor returns an appropriate color for the severity level
func (dc *DiscordChannel) getSeverityColor(severity string) int {
	switch severity {
	case "critical":
		return 0xFF0000 // Red
	case "high":
		return 0xFF8C00 // Dark Orange
	case "medium":
		return 0xFFD700 // Gold
	case "low":
		return 0x32CD32 // Lime Green
	default:
		return 0x808080 // Gray
	}
}

// formatSeverity formats the severity for display
func (dc *DiscordChannel) formatSeverity(severity string) string {
	switch severity {
	case "critical":
		return "🚨 Critical"
	case "high":
		return "⚠️ High"
	case "medium":
		return "ℹ️ Medium"
	case "low":
		return "✅ Low"
	default:
		return fmt.Sprintf("❓ %s", severity)
	}
}
