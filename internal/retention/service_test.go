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

func TestRetentionService(t *testing.T) {
	// Create in-memory storage for testing
	db, err := storage.NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Create test configuration
	retentionConfig := &config.RetentionConfig{
		MonitoringRunsDays: 7,
		DriftsDays:         30,
		AlertsDays:         14,
		AutoCleanup:        true,
		CleanupInterval:    24 * time.Hour,
	}

	// Create logger
	logger, err := logging.NewLogger(logging.DefaultLoggerConfig())
	require.NoError(t, err)

	// Create retention service
	service := NewService(db, retentionConfig, logger)

	t.Run("NewService", func(t *testing.T) {
		assert.NotNil(t, service)
		assert.Equal(t, db, service.storage)
		assert.Equal(t, retentionConfig, service.config)
		assert.Equal(t, logger, service.logger)
	})

	t.Run("IntervalToCron", func(t *testing.T) {
		testCases := []struct {
			interval time.Duration
			expected string
		}{
			{30 * time.Minute, "0 * * * *"},    // Every hour
			{3 * time.Hour, "0 */6 * * *"},     // Every 6 hours
			{8 * time.Hour, "0 */12 * * *"},    // Every 12 hours
			{24 * time.Hour, "0 2 * * *"},      // Daily at 2 AM
			{3 * 24 * time.Hour, "0 2 * * 0"},  // Weekly on Sunday at 2 AM
			{30 * 24 * time.Hour, "0 2 1 * *"}, // Monthly on the 1st at 2 AM
		}

		for _, tc := range testCases {
			result := service.intervalToCron(tc.interval)
			assert.Equal(t, tc.expected, result, "Interval: %v", tc.interval)
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		stats, err := service.GetStats()
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(0), stats.MonitoringRuns)
		assert.Equal(t, int64(0), stats.Drifts)
		assert.Equal(t, int64(0), stats.Alerts)
	})

	t.Run("PerformCleanup", func(t *testing.T) {
		// Add test data
		now := time.Now()

		// Add old monitoring runs
		oldRun := &storage.MonitoringRun{
			EndpointID:      "test-endpoint",
			Timestamp:       now.AddDate(0, 0, -10), // 10 days old
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    "test response",
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err := db.SaveMonitoringRun(oldRun)
		require.NoError(t, err)

		// Add recent monitoring runs
		recentRun := &storage.MonitoringRun{
			EndpointID:      "test-endpoint",
			Timestamp:       now.AddDate(0, 0, -3), // 3 days old
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    "test response",
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err = db.SaveMonitoringRun(recentRun)
		require.NoError(t, err)

		// Add old drifts
		oldDrift := &storage.Drift{
			EndpointID:  "test-endpoint",
			DetectedAt:  now.AddDate(0, 0, -35), // 35 days old
			DriftType:   "schema_change",
			Severity:    "high",
			Description: "Field removed",
		}
		err = db.SaveDrift(oldDrift)
		require.NoError(t, err)

		// Add recent drifts
		recentDrift := &storage.Drift{
			EndpointID:  "test-endpoint",
			DetectedAt:  now.AddDate(0, 0, -15), // 15 days old
			DriftType:   "schema_change",
			Severity:    "medium",
			Description: "Field added",
		}
		err = db.SaveDrift(recentDrift)
		require.NoError(t, err)

		// Add old alerts
		oldAlert := &storage.Alert{
			DriftID:     1,
			AlertType:   "slack",
			ChannelName: "test-channel",
			SentAt:      now.AddDate(0, 0, -20), // 20 days old
			Status:      "sent",
		}
		err = db.SaveAlert(oldAlert)
		require.NoError(t, err)

		// Add recent alerts
		recentAlert := &storage.Alert{
			DriftID:     2,
			AlertType:   "slack",
			ChannelName: "test-channel",
			SentAt:      now.AddDate(0, 0, -5), // 5 days old
			Status:      "sent",
		}
		err = db.SaveAlert(recentAlert)
		require.NoError(t, err)

		// Perform cleanup
		err = service.PerformCleanup()
		assert.NoError(t, err)

		// Verify cleanup results
		stats, err := service.GetStats()
		assert.NoError(t, err)

		// Should have 1 monitoring run (recent one)
		assert.Equal(t, int64(1), stats.MonitoringRuns)

		// Should have 1 drift (recent one)
		assert.Equal(t, int64(1), stats.Drifts)

		// Should have 1 alert (recent one)
		assert.Equal(t, int64(1), stats.Alerts)
	})

	t.Run("CleanupWithOptions", func(t *testing.T) {
		// Clear previous data
		db.Close()
		db, err = storage.NewInMemoryStorage()
		require.NoError(t, err)
		service.storage = db

		now := time.Now()
		cutoff := now.AddDate(0, 0, -7)

		// Add test data
		oldRun := &storage.MonitoringRun{
			EndpointID:      "test-endpoint",
			Timestamp:       now.AddDate(0, 0, -10),
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    "test response",
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err = db.SaveMonitoringRun(oldRun)
		require.NoError(t, err)

		// Test dry run
		opts := CleanupOptions{
			MonitoringRunsOlderThan: &cutoff,
			DryRun:                  true,
		}

		result, err := service.CleanupWithOptions(opts)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.MonitoringRunsWouldClean)
		assert.Equal(t, int64(0), result.MonitoringRunsCleaned)

		// Test actual cleanup
		opts.DryRun = false
		opts.VacuumAfter = true

		result, err = service.CleanupWithOptions(opts)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1), result.MonitoringRunsCleaned)
		assert.Equal(t, int64(1), result.TotalCleaned())
	})

	t.Run("StartStop", func(t *testing.T) {
		// Test starting with auto cleanup disabled
		disabledConfig := &config.RetentionConfig{
			AutoCleanup: false,
		}
		disabledService := NewService(db, disabledConfig, logger)

		err := disabledService.Start()
		assert.NoError(t, err)

		disabledService.Stop()

		// Test starting with auto cleanup enabled
		err = service.Start()
		assert.NoError(t, err)

		service.Stop()
	})
}

func TestCleanupResult(t *testing.T) {
	result := &CleanupResult{
		MonitoringRunsCleaned: 10,
		DriftsCleaned:         5,
		AlertsCleaned:         3,
		DatabaseVacuumed:      true,
	}

	assert.Equal(t, int64(18), result.TotalCleaned())

	emptyResult := &CleanupResult{}
	assert.Equal(t, int64(0), emptyResult.TotalCleaned())
}
