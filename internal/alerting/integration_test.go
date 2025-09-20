package alerting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	dbFile := "test_alerting.db"
	defer os.Remove(dbFile)

	// Create storage
	store, err := storage.NewSQLiteStorage(dbFile)
	require.NoError(t, err)
	defer store.Close()

	// Create mock webhook server
	var receivedAlerts []WebhookPayload
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			receivedAlerts = append(receivedAlerts, payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// Create configuration with webhook channel
	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: true,
			Channels: []config.AlertChannelConfig{
				{
					Type:    "webhook",
					Name:    "test-webhook",
					Enabled: true,
					Settings: map[string]interface{}{
						"url": webhookServer.URL,
					},
				},
			},
			Rules: []config.AlertRuleConfig{
				{
					Name:     "critical-alerts",
					Severity: []string{"critical", "high"},
					Channels: []string{"test-webhook"},
				},
				{
					Name:     "medium-alerts",
					Severity: []string{"medium"},
					Channels: []string{"test-webhook"},
				},
			},
		},
	}

	// Create alert manager
	alertManager, err := NewAlertManager(cfg, store)
	require.NoError(t, err)

	// Create test endpoint
	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
	}

	// Save endpoint first to satisfy foreign key constraint
	err = store.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Test 1: Process critical drift
	criticalDrift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.user.email",
				Description: "Critical field removed",
				Severity:    drift.SeverityCritical,
				Breaking:    true,
				OldValue:    "test@example.com",
			},
		},
	}

	ctx := context.Background()
	err = alertManager.ProcessDrift(ctx, criticalDrift, endpoint)
	require.NoError(t, err)

	// Verify alert was sent
	assert.Len(t, receivedAlerts, 1)
	assert.Equal(t, "critical", receivedAlerts[0].Alert.Severity)
	assert.Contains(t, receivedAlerts[0].Alert.Title, "API Drift Detected")

	// Test 2: Process medium drift
	mediumDrift := &drift.DiffResult{
		HasChanges: true,
		DataChanges: []drift.DataChange{
			{
				Path:        "$.user.name",
				OldValue:    "John",
				NewValue:    "Jane",
				ChangeType:  drift.ChangeTypeFieldModified,
				Severity:    drift.SeverityMedium,
				Description: "Name field changed",
			},
		},
	}

	err = alertManager.ProcessDrift(ctx, mediumDrift, endpoint)
	require.NoError(t, err)

	// Verify second alert was sent
	assert.Len(t, receivedAlerts, 2)
	assert.Equal(t, "medium", receivedAlerts[1].Alert.Severity)

	// Test 3: Process low severity drift (should not trigger alerts based on rules)
	lowDrift := &drift.DiffResult{
		HasChanges: true,
		DataChanges: []drift.DataChange{
			{
				Path:        "$.metadata.timestamp",
				OldValue:    "2023-01-01",
				NewValue:    "2023-01-02",
				ChangeType:  drift.ChangeTypeFieldModified,
				Severity:    drift.SeverityLow,
				Description: "Timestamp updated",
			},
		},
	}

	err = alertManager.ProcessDrift(ctx, lowDrift, endpoint)
	require.NoError(t, err)

	// Should still be 2 alerts (low severity not configured in rules)
	assert.Len(t, receivedAlerts, 2)

	// Test 4: Verify alert history
	filters := AlertFilters{}
	alerts, err := alertManager.GetAlertHistory(filters)
	require.NoError(t, err)
	assert.Len(t, alerts, 2)

	// Test 5: Test configuration
	err = alertManager.TestConfiguration(ctx)
	assert.NoError(t, err)

	// Should have received test alert
	assert.Len(t, receivedAlerts, 3)
	assert.Contains(t, receivedAlerts[2].Alert.Title, "Test Alert")
}

func TestAlertingWithMultipleChannels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	dbFile := "test_multi_alerting.db"
	defer os.Remove(dbFile)

	store, err := storage.NewSQLiteStorage(dbFile)
	require.NoError(t, err)
	defer store.Close()

	// Create multiple mock servers
	var webhook1Alerts, webhook2Alerts []WebhookPayload
	var discordAlerts []DiscordMessage

	webhook1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			webhook1Alerts = append(webhook1Alerts, payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook1Server.Close()

	webhook2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			webhook2Alerts = append(webhook2Alerts, payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook2Server.Close()

	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message DiscordMessage
		err := json.NewDecoder(r.Body).Decode(&message)
		if err == nil {
			discordAlerts = append(discordAlerts, message)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer discordServer.Close()

	// Configuration with multiple channels and rules
	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: true,
			Channels: []config.AlertChannelConfig{
				{
					Type:    "webhook",
					Name:    "webhook1",
					Enabled: true,
					Settings: map[string]interface{}{
						"url": webhook1Server.URL,
					},
				},
				{
					Type:    "webhook",
					Name:    "webhook2",
					Enabled: true,
					Settings: map[string]interface{}{
						"url": webhook2Server.URL,
					},
				},
				{
					Type:    "discord",
					Name:    "discord1",
					Enabled: true,
					Settings: map[string]interface{}{
						"webhook_url": discordServer.URL,
					},
				},
			},
			Rules: []config.AlertRuleConfig{
				{
					Name:     "critical-to-all",
					Severity: []string{"critical"},
					Channels: []string{"webhook1", "webhook2", "discord1"},
				},
				{
					Name:     "high-to-webhook1",
					Severity: []string{"high"},
					Channels: []string{"webhook1"},
				},
			},
		},
	}

	alertManager, err := NewAlertManager(cfg, store)
	require.NoError(t, err)

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
	}

	// Save endpoint first to satisfy foreign key constraint
	err = store.SaveEndpoint(endpoint)
	require.NoError(t, err)

	ctx := context.Background()

	// Test critical alert (should go to both channels)
	criticalDrift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.critical.field",
				Description: "Critical field removed",
				Severity:    drift.SeverityCritical,
				Breaking:    true,
			},
		},
	}

	err = alertManager.ProcessDrift(ctx, criticalDrift, endpoint)
	require.NoError(t, err)

	// All channels should receive the critical alert
	assert.Len(t, webhook1Alerts, 1)
	assert.Len(t, webhook2Alerts, 1)
	assert.Len(t, discordAlerts, 1)

	// Test high alert (should go to webhook1 only)
	highDrift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldModified,
				Path:        "$.high.field",
				Description: "High severity change",
				Severity:    drift.SeverityHigh,
				Breaking:    false,
			},
		},
	}

	err = alertManager.ProcessDrift(ctx, highDrift, endpoint)
	require.NoError(t, err)

	// Only webhook1 should receive the high alert
	assert.Len(t, webhook1Alerts, 2)
	assert.Len(t, webhook2Alerts, 1) // Still just the critical alert
	assert.Len(t, discordAlerts, 1)  // Still just the critical alert
}

func TestAlertingWithEndpointFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbFile := "test_endpoint_filtering.db"
	defer os.Remove(dbFile)

	store, err := storage.NewSQLiteStorage(dbFile)
	require.NoError(t, err)
	defer store.Close()

	var receivedAlerts []WebhookPayload
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			receivedAlerts = append(receivedAlerts, payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// Configuration with endpoint-specific rules
	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: true,
			Channels: []config.AlertChannelConfig{
				{
					Type:    "webhook",
					Name:    "webhook",
					Enabled: true,
					Settings: map[string]interface{}{
						"url": webhookServer.URL,
					},
				},
			},
			Rules: []config.AlertRuleConfig{
				{
					Name:      "users-api-alerts",
					Severity:  []string{"high", "critical"},
					Endpoints: []string{"users-api"},
					Channels:  []string{"webhook"},
				},
			},
		},
	}

	alertManager, err := NewAlertManager(cfg, store)
	require.NoError(t, err)

	ctx := context.Background()

	// Create endpoints
	usersEndpoint := &storage.Endpoint{
		ID:     "users-api",
		URL:    "https://api.example.com/users",
		Method: "GET",
	}

	ordersEndpoint := &storage.Endpoint{
		ID:     "orders-api",
		URL:    "https://api.example.com/orders",
		Method: "GET",
	}

	// Save endpoints first to satisfy foreign key constraint
	err = store.SaveEndpoint(usersEndpoint)
	require.NoError(t, err)
	err = store.SaveEndpoint(ordersEndpoint)
	require.NoError(t, err)

	// Test high drift on users-api (should trigger alert)
	highDrift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.user.email",
				Description: "Email field removed",
				Severity:    drift.SeverityHigh,
				Breaking:    true,
			},
		},
	}

	err = alertManager.ProcessDrift(ctx, highDrift, usersEndpoint)
	require.NoError(t, err)

	// Should receive alert for users-api
	assert.Len(t, receivedAlerts, 1)
	assert.Equal(t, "users-api", receivedAlerts[0].Alert.EndpointID)

	// Test high drift on orders-api (should NOT trigger alert due to endpoint filter)
	err = alertManager.ProcessDrift(ctx, highDrift, ordersEndpoint)
	require.NoError(t, err)

	// Should still be just 1 alert (orders-api not in rule)
	assert.Len(t, receivedAlerts, 1)
}

func TestAlertingFailureHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbFile := "test_failure_handling.db"
	defer os.Remove(dbFile)

	store, err := storage.NewSQLiteStorage(dbFile)
	require.NoError(t, err)
	defer store.Close()

	// Create a server that returns errors
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: true,
			Channels: []config.AlertChannelConfig{
				{
					Type:    "webhook",
					Name:    "failing-webhook",
					Enabled: true,
					Settings: map[string]interface{}{
						"url": failingServer.URL,
					},
				},
			},
			Rules: []config.AlertRuleConfig{
				{
					Name:     "test-rule",
					Severity: []string{"high"},
					Channels: []string{"failing-webhook"},
				},
			},
		},
	}

	alertManager, err := NewAlertManager(cfg, store)
	require.NoError(t, err)

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
	}

	// Save endpoint first to satisfy foreign key constraint
	err = store.SaveEndpoint(endpoint)
	require.NoError(t, err)

	highDrift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.field",
				Description: "Field removed",
				Severity:    drift.SeverityHigh,
				Breaking:    true,
			},
		},
	}

	ctx := context.Background()
	err = alertManager.ProcessDrift(ctx, highDrift, endpoint)

	// Should return error due to webhook failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send alert")

	// Verify that the drift was still saved even though alert failed
	filters := storage.DriftFilters{EndpointID: "test-endpoint"}
	drifts, err := store.GetDrifts(filters)
	require.NoError(t, err)
	assert.Len(t, drifts, 1)

	// Verify that failed alert was recorded
	alertFilters := storage.AlertFilters{Status: "failed"}
	alerts, err := store.GetAlerts(alertFilters)
	require.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "failed", alerts[0].Status)
	assert.NotEmpty(t, alerts[0].ErrorMessage)
}

func TestAlertingDisabled(t *testing.T) {
	dbFile := "test_disabled.db"
	defer os.Remove(dbFile)

	store, err := storage.NewSQLiteStorage(dbFile)
	require.NoError(t, err)
	defer store.Close()

	// Configuration with alerting disabled
	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: false,
		},
	}

	alertManager, err := NewAlertManager(cfg, store)
	require.NoError(t, err)

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
	}

	// Save endpoint first to satisfy foreign key constraint
	err = store.SaveEndpoint(endpoint)
	require.NoError(t, err)

	drift := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.field",
				Description: "Field removed",
				Severity:    drift.SeverityCritical,
				Breaking:    true,
			},
		},
	}

	ctx := context.Background()
	err = alertManager.ProcessDrift(ctx, drift, endpoint)

	// Should not return error, but should not send alerts
	assert.NoError(t, err)

	// Verify no alerts were sent (no storage calls for alerts)
	alertFilters := storage.AlertFilters{}
	alerts, err := store.GetAlerts(alertFilters)
	require.NoError(t, err)
	assert.Len(t, alerts, 0)
}
