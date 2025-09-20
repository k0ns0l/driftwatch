package retention

import (
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetentionServiceIntegration(t *testing.T) {
	// Create temporary database for testing
	tmpDB := t.TempDir() + "/test.db"

	// Initialize storage
	db, err := storage.NewStorage(tmpDB)
	require.NoError(t, err)
	defer db.Close()

	// Create retention configuration
	retentionConfig := &config.RetentionConfig{
		MonitoringRunsDays: 7,
		DriftsDays:         30,
		AlertsDays:         14,
		AutoCleanup:        false, // Disable auto cleanup for testing
		CleanupInterval:    24 * time.Hour,
	}

	// Create logger
	logger, err := logging.NewLogger(logging.DefaultLoggerConfig())
	require.NoError(t, err)

	// Create retention service
	service := NewService(db, retentionConfig, logger)

	// Add test endpoint first (required for foreign key constraint)
	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: `{"interval": "5m"}`,
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Add test data
	now := time.Now()

	// Add old monitoring runs (should be cleaned)
	for i := 0; i < 3; i++ {
		oldRun := &storage.MonitoringRun{
			EndpointID:      "test-endpoint",
			Timestamp:       now.AddDate(0, 0, -10-i), // 10+ days old
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    "test response",
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err = db.SaveMonitoringRun(oldRun)
		require.NoError(t, err)
	}

	// Add recent monitoring runs (should be kept)
	for i := 0; i < 2; i++ {
		recentRun := &storage.MonitoringRun{
			EndpointID:      "test-endpoint",
			Timestamp:       now.AddDate(0, 0, -3-i), // 3-4 days old
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    "test response",
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err = db.SaveMonitoringRun(recentRun)
		require.NoError(t, err)
	}

	// Add old drifts (should be cleaned)
	for i := 0; i < 2; i++ {
		oldDrift := &storage.Drift{
			EndpointID:  "test-endpoint",
			DetectedAt:  now.AddDate(0, 0, -35-i), // 35+ days old
			DriftType:   "schema_change",
			Severity:    "high",
			Description: "Old drift",
		}
		err = db.SaveDrift(oldDrift)
		require.NoError(t, err)
	}

	// Add recent drifts (should be kept)
	recentDrift := &storage.Drift{
		EndpointID:  "test-endpoint",
		DetectedAt:  now.AddDate(0, 0, -15), // 15 days old
		DriftType:   "schema_change",
		Severity:    "medium",
		Description: "Recent drift",
	}
	err = db.SaveDrift(recentDrift)
	require.NoError(t, err)

	// Add alerts for the recent drift (should be kept since drift is kept)
	recentAlert := &storage.Alert{
		DriftID:     3, // This corresponds to the recent drift (ID 3)
		AlertType:   "slack",
		ChannelName: "test-channel",
		SentAt:      now.AddDate(0, 0, -5), // 5 days old
		Status:      "sent",
	}
	err = db.SaveAlert(recentAlert)
	require.NoError(t, err)

	// Add alerts for old drifts (will be cascade deleted when drifts are deleted)
	oldAlert1 := &storage.Alert{
		DriftID:     1, // This corresponds to an old drift (ID 1)
		AlertType:   "slack",
		ChannelName: "test-channel",
		SentAt:      now.AddDate(0, 0, -20), // 20 days old
		Status:      "sent",
	}
	err = db.SaveAlert(oldAlert1)
	require.NoError(t, err)

	oldAlert2 := &storage.Alert{
		DriftID:     2, // This corresponds to an old drift (ID 2)
		AlertType:   "email",
		ChannelName: "test-email",
		SentAt:      now.AddDate(0, 0, -25), // 25 days old
		Status:      "sent",
	}
	err = db.SaveAlert(oldAlert2)
	require.NoError(t, err)

	// Get initial stats
	initialStats, err := service.GetStats()
	require.NoError(t, err)
	assert.Equal(t, int64(5), initialStats.MonitoringRuns) // 3 old + 2 recent
	assert.Equal(t, int64(3), initialStats.Drifts)         // 2 old + 1 recent
	assert.Equal(t, int64(3), initialStats.Alerts)         // 2 old + 1 recent

	// Perform cleanup
	err = service.PerformCleanup()
	require.NoError(t, err)

	// Get final stats
	finalStats, err := service.GetStats()
	require.NoError(t, err)

	// Verify cleanup results
	assert.Equal(t, int64(2), finalStats.MonitoringRuns) // Only recent runs should remain
	assert.Equal(t, int64(1), finalStats.Drifts)         // Only recent drift should remain

	// Debug: Check what alerts remain
	allAlerts, err := db.GetAlerts(storage.AlertFilters{})
	require.NoError(t, err)
	t.Logf("Remaining alerts: %d", len(allAlerts))
	for i, alert := range allAlerts {
		t.Logf("  Alert %d: DriftID=%d, SentAt=%s, Age=%s", i+1, alert.DriftID, alert.SentAt.Format("2006-01-02"), now.Sub(alert.SentAt))
	}

	// The recent alert should remain since its associated drift is kept
	// Note: Alerts are cascade deleted when their associated drifts are deleted
	assert.Equal(t, int64(1), finalStats.Alerts) // Only alert for recent drift should remain

	// Verify the database size is reasonable
	assert.Greater(t, finalStats.DatabaseSizeBytes, int64(0))

	t.Logf("Cleanup completed successfully:")
	t.Logf("  Monitoring runs: %d -> %d", initialStats.MonitoringRuns, finalStats.MonitoringRuns)
	t.Logf("  Drifts: %d -> %d", initialStats.Drifts, finalStats.Drifts)
	t.Logf("  Alerts: %d -> %d (cascade deleted with drifts)", initialStats.Alerts, finalStats.Alerts)
	t.Logf("  Database size: %.2f KB", float64(finalStats.DatabaseSizeBytes)/1024)
}
