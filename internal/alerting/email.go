package alerting

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
)

// EmailChannel implements AlertChannel for SMTP email integration
type EmailChannel struct {
	name     string
	host     string
	port     int
	username string
	password string
	from     string
	to       []string
	enabled  bool
	useTLS   bool
}

// NewEmailChannel creates a new email alert channel
func NewEmailChannel(channelConfig config.AlertChannelConfig) (AlertChannel, error) {
	settings := channelConfig.Settings

	host, err := validateHost(settings)
	if err != nil {
		return nil, err
	}

	port, err := validatePort(settings)
	if err != nil {
		return nil, err
	}

	from, err := validateFrom(settings)
	if err != nil {
		return nil, err
	}

	to, err := validateTo(settings)
	if err != nil {
		return nil, err
	}

	channel := &EmailChannel{
		name:    channelConfig.Name,
		host:    host,
		port:    port,
		from:    from,
		to:      to,
		enabled: channelConfig.Enabled,
		useTLS:  true, // Default to TLS
	}

	setOptionalSettings(channel, settings)

	return channel, nil
}

// validateHost validates the SMTP host setting
func validateHost(settings map[string]interface{}) (string, error) {
	host, ok := settings["smtp_host"].(string)
	if !ok || host == "" {
		return "", fmt.Errorf("smtp_host is required for email channel")
	}
	return host, nil
}

// validatePort validates the SMTP port setting
func validatePort(settings map[string]interface{}) (int, error) {
	portInterface, ok := settings["smtp_port"]
	if !ok {
		return 0, fmt.Errorf("smtp_port is required for email channel")
	}

	switch v := portInterface.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		port, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid smtp_port: %w", err)
		}
		return port, nil
	default:
		return 0, fmt.Errorf("smtp_port must be a number")
	}
}

// validateFrom validates the from address setting
func validateFrom(settings map[string]interface{}) (string, error) {
	from, ok := settings["from"].(string)
	if !ok || from == "" {
		return "", fmt.Errorf("from address is required for email channel")
	}
	return from, nil
}

// validateTo validates the to addresses setting
func validateTo(settings map[string]interface{}) ([]string, error) {
	toInterface, ok := settings["to"]
	if !ok {
		return nil, fmt.Errorf("to addresses are required for email channel")
	}

	var to []string
	switch v := toInterface.(type) {
	case string:
		to = []string{v}
	case []interface{}:
		for _, addr := range v {
			if addrStr, ok := addr.(string); ok {
				to = append(to, addrStr)
			}
		}
	case []string:
		to = v
	default:
		return nil, fmt.Errorf("to addresses must be a string or array of strings")
	}

	if len(to) == 0 {
		return nil, fmt.Errorf("at least one to address is required")
	}

	return to, nil
}

// setOptionalSettings sets optional email channel settings
func setOptionalSettings(channel *EmailChannel, settings map[string]interface{}) {
	if username, ok := settings["username"].(string); ok {
		channel.username = username
	}
	if password, ok := settings["password"].(string); ok {
		channel.password = password
	}
	if useTLS, ok := settings["use_tls"].(bool); ok {
		channel.useTLS = useTLS
	}
}

// Send sends an alert message via email
func (ec *EmailChannel) Send(ctx context.Context, message *AlertMessage) error {
	subject := fmt.Sprintf("[DriftWatch] %s", message.Title)
	body := ec.formatMessage(message)

	return ec.sendEmail(ctx, subject, body)
}

// Test sends a test email to verify the configuration
func (ec *EmailChannel) Test(ctx context.Context) error {
	testMessage := &AlertMessage{
		Title:       "DriftWatch Test Alert",
		Summary:     "This is a test message to verify email integration is working correctly.",
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

	return ec.Send(ctx, testMessage)
}

// GetType returns the channel type
func (ec *EmailChannel) GetType() string {
	return "email"
}

// GetName returns the channel name
func (ec *EmailChannel) GetName() string {
	return ec.name
}

// IsEnabled returns whether the channel is enabled
func (ec *EmailChannel) IsEnabled() bool {
	return ec.enabled
}

// sendEmail sends an email using SMTP
func (ec *EmailChannel) sendEmail(ctx context.Context, subject, body string) error {
	// Create the email message
	msg := ec.buildEmailMessage(subject, body)

	// Set up authentication
	var auth smtp.Auth
	if ec.username != "" && ec.password != "" {
		auth = smtp.PlainAuth("", ec.username, ec.password, ec.host)
	}

	// Send the email
	addr := fmt.Sprintf("%s:%d", ec.host, ec.port)

	// Use a channel to handle timeout
	errChan := make(chan error, 1)

	go func() {
		err := smtp.SendMail(addr, auth, ec.from, ec.to, []byte(msg))
		errChan <- err
	}()

	// Wait for completion or timeout
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("email send timeout")
	}
}

// buildEmailMessage builds the complete email message with headers
func (ec *EmailChannel) buildEmailMessage(subject, body string) string {
	var msg strings.Builder

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", ec.from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(ec.to, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")

	// Body
	msg.WriteString(body)

	return msg.String()
}

// formatMessage formats an AlertMessage for email
func (ec *EmailChannel) formatMessage(message *AlertMessage) string {
	var html strings.Builder

	// HTML email template
	html.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>DriftWatch Alert</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .severity-critical { border-left: 5px solid #dc3545; }
        .severity-high { border-left: 5px solid #fd7e14; }
        .severity-medium { border-left: 5px solid #ffc107; }
        .severity-low { border-left: 5px solid #28a745; }
        .details { background-color: #f8f9fa; padding: 15px; border-radius: 5px; margin: 15px 0; }
        .changes { margin: 20px 0; }
        .change-item { margin: 10px 0; padding: 10px; background-color: #fff; border-radius: 3px; border-left: 3px solid #007bff; }
        .breaking { border-left-color: #dc3545; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #dee2e6; font-size: 12px; color: #6c757d; }
        table { width: 100%; border-collapse: collapse; }
        td { padding: 8px; border-bottom: 1px solid #dee2e6; }
        .label { font-weight: bold; width: 120px; }
    </style>
</head>
<body>
    <div class="container">`)

	// Header with severity styling
	severityClass := fmt.Sprintf("severity-%s", message.Severity)
	html.WriteString(fmt.Sprintf(`
        <div class="header %s">
            <h1>%s</h1>
            <p>%s</p>
        </div>`, severityClass, message.Title, message.Summary))

	// Details table
	html.WriteString(`
        <div class="details">
            <h3>Alert Details</h3>
            <table>`)

	html.WriteString(fmt.Sprintf(`
                <tr><td class="label">Endpoint:</td><td>%s</td></tr>
                <tr><td class="label">Endpoint ID:</td><td>%s</td></tr>
                <tr><td class="label">Severity:</td><td>%s</td></tr>
                <tr><td class="label">Detected At:</td><td>%s</td></tr>`,
		message.EndpointURL,
		message.EndpointID,
		ec.formatSeverity(message.Severity),
		message.DetectedAt.Format("2006-01-02 15:04:05 UTC")))

	html.WriteString(`
            </table>
        </div>`)

	// Changes section
	if len(message.Changes) > 0 {
		html.WriteString(`
        <div class="changes">
            <h3>Changes Detected</h3>`)

		for _, change := range message.Changes {
			breakingClass := ""
			if change.Breaking {
				breakingClass = " breaking"
			}

			html.WriteString(fmt.Sprintf(`
            <div class="change-item%s">
                <strong>%s</strong> at <code>%s</code>`,
				breakingClass, change.Type, change.Path))

			if change.Breaking {
				html.WriteString(` <span style="color: #dc3545; font-weight: bold;">[BREAKING]</span>`)
			}

			html.WriteString(fmt.Sprintf(`
                <br><em>Severity: %s</em>`, change.Severity))

			if change.Description != "" {
				html.WriteString(fmt.Sprintf(`
                <p>%s</p>`, change.Description))
			}

			if change.OldValue != nil || change.NewValue != nil {
				html.WriteString(`<table style="margin-top: 10px; font-size: 12px;">`)
				if change.OldValue != nil {
					html.WriteString(fmt.Sprintf(`
                    <tr><td class="label">Old Value:</td><td><code>%v</code></td></tr>`, change.OldValue))
				}
				if change.NewValue != nil {
					html.WriteString(fmt.Sprintf(`
                    <tr><td class="label">New Value:</td><td><code>%v</code></td></tr>`, change.NewValue))
				}
				html.WriteString(`</table>`)
			}

			html.WriteString(`
            </div>`)
		}

		html.WriteString(`
        </div>`)
	}

	// Footer
	html.WriteString(`
        <div class="footer">
            <p>This alert was generated by DriftWatch API monitoring system.</p>
            <p>If you believe this alert was sent in error, please check your DriftWatch configuration.</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// formatSeverity formats the severity for display
func (ec *EmailChannel) formatSeverity(severity string) string {
	switch severity {
	case "critical":
		return "🚨 Critical"
	case "high":
		return "⚠️ High"
	case "medium":
		return "ℹ️ Medium"
	case "low":
		return "⚪ Low"
	default:
		return fmt.Sprintf("❓ %s", severity)
	}
}
