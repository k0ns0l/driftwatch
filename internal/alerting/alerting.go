// Package alerting provides alert management and delivery functionality
package alerting

import (
	"context"
	"fmt"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	"github.com/k0ns0l/driftwatch/internal/storage"
)

// AlertManager defines the interface for managing and sending alerts
type AlertManager interface {
	SendAlert(ctx context.Context, drift *storage.Drift, endpoint *storage.Endpoint) error
	TestConfiguration(ctx context.Context) error
	GetAlertHistory(filters AlertFilters) ([]*Alert, error)
	ProcessDrift(ctx context.Context, driftResult *drift.DiffResult, endpoint *storage.Endpoint) error
}

// AlertChannel defines the interface for different alert delivery channels
type AlertChannel interface {
	Send(ctx context.Context, message *AlertMessage) error
	Test(ctx context.Context) error
	GetType() string
	GetName() string
	IsEnabled() bool
}

// AlertMessage represents a formatted alert message
type AlertMessage struct {
	Changes     []ChangeDetail         `json:"changes"`
	Metadata    map[string]interface{} `json:"metadata"`
	DetectedAt  time.Time              `json:"detected_at"`
	Title       string                 `json:"title"`
	Summary     string                 `json:"summary"`
	Severity    string                 `json:"severity"`
	EndpointID  string                 `json:"endpoint_id"`
	EndpointURL string                 `json:"endpoint_url"`
}

// ChangeDetail represents details about a specific change
type ChangeDetail struct {
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	Type        string      `json:"type"`
	Path        string      `json:"path"`
	Description string      `json:"description"`
	Severity    string      `json:"severity"`
	Breaking    bool        `json:"breaking"`
}

// Alert represents a sent alert record
type Alert struct {
	SentAt       time.Time `json:"sent_at"`
	ID           int64     `json:"id"`
	DriftID      int64     `json:"drift_id"`
	AlertType    string    `json:"alert_type"`
	ChannelName  string    `json:"channel_name"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	RetryCount   int       `json:"retry_count"`
}

// AlertFilters represents filters for querying alerts
type AlertFilters struct {
	DriftID     *int64
	AlertType   string
	ChannelName string
	Status      string
	StartTime   time.Time
	EndTime     time.Time
}

// AlertStatus represents the status of an alert delivery
type AlertStatus string

const (
	AlertStatusPending AlertStatus = "pending"
	AlertStatusSent    AlertStatus = "sent"
	AlertStatusFailed  AlertStatus = "failed"
	AlertStatusRetry   AlertStatus = "retry"
)

// DefaultAlertManager implements the AlertManager interface
type DefaultAlertManager struct {
	config   *config.Config
	storage  storage.Storage
	channels map[string]AlertChannel
}

// NewAlertManager creates a new alert manager instance
func NewAlertManager(cfg *config.Config, storage storage.Storage) (AlertManager, error) {
	manager := &DefaultAlertManager{
		config:   cfg,
		storage:  storage,
		channels: make(map[string]AlertChannel),
	}

	// Initialize alert channels based on configuration
	if err := manager.initializeChannels(); err != nil {
		return nil, fmt.Errorf("failed to initialize alert channels: %w", err)
	}

	return manager, nil
}

// initializeChannels initializes alert channels based on configuration
func (am *DefaultAlertManager) initializeChannels() error {
	for _, channelConfig := range am.config.Alerting.Channels {
		if !channelConfig.Enabled {
			continue
		}

		var channel AlertChannel
		var err error

		switch channelConfig.Type {
		case "slack":
			channel, err = NewSlackChannel(channelConfig)
		case "discord":
			channel, err = NewDiscordChannel(channelConfig)
		case "email":
			channel, err = NewEmailChannel(channelConfig)
		case "webhook":
			channel, err = NewWebhookChannel(channelConfig)
		default:
			return fmt.Errorf("unsupported alert channel type: %s", channelConfig.Type)
		}

		if err != nil {
			return fmt.Errorf("failed to create %s channel '%s': %w",
				channelConfig.Type, channelConfig.Name, err)
		}

		am.channels[channelConfig.Name] = channel
	}

	return nil
}

// ProcessDrift processes a drift result and sends alerts based on configured rules
func (am *DefaultAlertManager) ProcessDrift(ctx context.Context, driftResult *drift.DiffResult, endpoint *storage.Endpoint) error {
	if !driftResult.HasChanges {
		return nil
	}

	// Convert drift result to storage drift records
	drifts := am.convertDriftResult(driftResult, endpoint)

	// Process each drift
	for _, drift := range drifts {
		// Save drift to storage
		if err := am.storage.SaveDrift(drift); err != nil {
			return fmt.Errorf("failed to save drift: %w", err)
		}

		// Send alerts based on rules (only if alerting is enabled)
		if am.config.Alerting.Enabled {
			if err := am.SendAlert(ctx, drift, endpoint); err != nil {
				return fmt.Errorf("failed to send alert for drift %d: %w", drift.ID, err)
			}
		}
	}

	return nil
}

// SendAlert sends an alert for a specific drift
func (am *DefaultAlertManager) SendAlert(ctx context.Context, drift *storage.Drift, endpoint *storage.Endpoint) error {
	// Find applicable alert rules
	applicableRules := am.findApplicableRules(drift, endpoint)
	if len(applicableRules) == 0 {
		return nil // No rules match, no alerts to send
	}

	// Create alert message
	message := am.createAlertMessage(drift, endpoint)

	// Send alerts through configured channels
	for _, rule := range applicableRules {
		for _, channelName := range rule.Channels {
			channel, exists := am.channels[channelName]
			if !exists || !channel.IsEnabled() {
				continue
			}

			alert := &storage.Alert{
				DriftID:     drift.ID,
				AlertType:   channel.GetType(),
				ChannelName: channelName,
				SentAt:      time.Now(),
				Status:      string(AlertStatusPending),
				RetryCount:  0,
			}

			// Send the alert
			if err := channel.Send(ctx, message); err != nil {
				alert.Status = string(AlertStatusFailed)
				alert.ErrorMessage = err.Error()

				// Save failed alert record
				if saveErr := am.storage.SaveAlert(alert); saveErr != nil {
					return fmt.Errorf("failed to save alert record: %w", saveErr)
				}

				return fmt.Errorf("failed to send alert via %s channel '%s': %w",
					channel.GetType(), channelName, err)
			}

			alert.Status = string(AlertStatusSent)

			// Save successful alert record
			if err := am.storage.SaveAlert(alert); err != nil {
				return fmt.Errorf("failed to save alert record: %w", err)
			}
		}
	}

	return nil
}

// TestConfiguration tests all configured alert channels
func (am *DefaultAlertManager) TestConfiguration(ctx context.Context) error {
	if !am.config.Alerting.Enabled {
		return fmt.Errorf("alerting is disabled in configuration")
	}

	if len(am.channels) == 0 {
		return fmt.Errorf("no alert channels configured")
	}

	var errors []string
	for name, channel := range am.channels {
		if !channel.IsEnabled() {
			continue
		}

		if err := channel.Test(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s (%s): %v", name, channel.GetType(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("alert channel test failures: %v", errors)
	}

	return nil
}

// GetAlertHistory retrieves alert history based on filters
func (am *DefaultAlertManager) GetAlertHistory(filters AlertFilters) ([]*Alert, error) {
	storageFilters := storage.AlertFilters{
		DriftID:     filters.DriftID,
		AlertType:   filters.AlertType,
		ChannelName: filters.ChannelName,
		Status:      filters.Status,
		StartTime:   filters.StartTime,
		EndTime:     filters.EndTime,
	}

	storageAlerts, err := am.storage.GetAlerts(storageFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts from storage: %w", err)
	}

	// Convert storage alerts to alerting alerts
	alerts := make([]*Alert, len(storageAlerts))
	for i, sa := range storageAlerts {
		alerts[i] = &Alert{
			ID:           sa.ID,
			DriftID:      sa.DriftID,
			AlertType:    sa.AlertType,
			ChannelName:  sa.ChannelName,
			SentAt:       sa.SentAt,
			Status:       sa.Status,
			ErrorMessage: sa.ErrorMessage,
			RetryCount:   sa.RetryCount,
		}
	}

	return alerts, nil
}

// Helper methods

func (am *DefaultAlertManager) findApplicableRules(drift *storage.Drift, endpoint *storage.Endpoint) []config.AlertRuleConfig {
	var applicableRules []config.AlertRuleConfig

	for _, rule := range am.config.Alerting.Rules {
		// Check severity match
		severityMatch := false
		for _, severity := range rule.Severity {
			if severity == drift.Severity {
				severityMatch = true
				break
			}
		}
		if !severityMatch {
			continue
		}

		// Check endpoint match (empty means all endpoints)
		if len(rule.Endpoints) > 0 {
			endpointMatch := false
			for _, endpointID := range rule.Endpoints {
				if endpointID == endpoint.ID {
					endpointMatch = true
					break
				}
			}
			if !endpointMatch {
				continue
			}
		}

		applicableRules = append(applicableRules, rule)
	}

	return applicableRules
}

func (am *DefaultAlertManager) createAlertMessage(drift *storage.Drift, endpoint *storage.Endpoint) *AlertMessage {
	severity := drift.Severity
	if severity == "" {
		severity = "medium"
	}

	return &AlertMessage{
		Title:       fmt.Sprintf("API Drift Detected: %s", endpoint.URL),
		Summary:     drift.Description,
		Severity:    severity,
		EndpointID:  endpoint.ID,
		EndpointURL: endpoint.URL,
		DetectedAt:  drift.DetectedAt,
		Changes: []ChangeDetail{
			{
				Type:        drift.DriftType,
				Path:        drift.FieldPath,
				Description: drift.Description,
				Severity:    severity,
				Breaking:    am.isBreakingChange(severity),
				OldValue:    drift.BeforeValue,
				NewValue:    drift.AfterValue,
			},
		},
		Metadata: map[string]interface{}{
			"endpoint_method": endpoint.Method,
			"drift_id":        drift.ID,
		},
	}
}

func (am *DefaultAlertManager) isBreakingChange(severity string) bool {
	return severity == "high" || severity == "critical"
}

func (am *DefaultAlertManager) convertDriftResult(driftResult *drift.DiffResult, endpoint *storage.Endpoint) []*storage.Drift {
	var drifts []*storage.Drift
	now := time.Now()

	// Convert structural changes
	for _, change := range driftResult.StructuralChanges {
		drift := &storage.Drift{
			EndpointID:   endpoint.ID,
			DetectedAt:   now,
			DriftType:    string(change.Type),
			Severity:     string(change.Severity),
			Description:  change.Description,
			FieldPath:    change.Path,
			Acknowledged: false,
		}

		if change.OldValue != nil {
			drift.BeforeValue = fmt.Sprintf("%v", change.OldValue)
		}
		if change.NewValue != nil {
			drift.AfterValue = fmt.Sprintf("%v", change.NewValue)
		}

		drifts = append(drifts, drift)
	}

	// Convert data changes
	for _, change := range driftResult.DataChanges {
		drift := &storage.Drift{
			EndpointID:   endpoint.ID,
			DetectedAt:   now,
			DriftType:    string(change.ChangeType),
			Severity:     string(change.Severity),
			Description:  change.Description,
			BeforeValue:  fmt.Sprintf("%v", change.OldValue),
			AfterValue:   fmt.Sprintf("%v", change.NewValue),
			FieldPath:    change.Path,
			Acknowledged: false,
		}

		drifts = append(drifts, drift)
	}

	// Convert performance changes
	if driftResult.PerformanceChanges != nil {
		drift := &storage.Drift{
			EndpointID:   endpoint.ID,
			DetectedAt:   now,
			DriftType:    "performance_change",
			Severity:     string(driftResult.PerformanceChanges.Severity),
			Description:  driftResult.PerformanceChanges.Description,
			FieldPath:    "$.response_time",
			Acknowledged: false,
		}

		drifts = append(drifts, drift)
	}

	return drifts
}
