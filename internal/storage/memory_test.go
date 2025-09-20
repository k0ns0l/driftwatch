package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryStorage(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify it implements the Storage interface
	var _ Storage = storage

	// Clean up
	err = storage.Close()
	assert.NoError(t, err)
}

func TestInMemoryStorage_Endpoints(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("save and get endpoint", func(t *testing.T) {
		endpoint := &Endpoint{
			ID:       "test-api",
			URL:      "https://api.example.com/test",
			Method:   "GET",
			SpecFile: "test-spec.yaml",
			Config:   `{"timeout": "30s"}`,
		}

		// Save endpoint
		err := storage.SaveEndpoint(endpoint)
		require.NoError(t, err)

		// Get endpoint
		retrieved, err := storage.GetEndpoint("test-api")
		require.NoError(t, err)

		assert.Equal(t, endpoint.ID, retrieved.ID)
		assert.Equal(t, endpoint.URL, retrieved.URL)
		assert.Equal(t, endpoint.Method, retrieved.Method)
		assert.Equal(t, endpoint.SpecFile, retrieved.SpecFile)
		assert.Equal(t, endpoint.Config, retrieved.Config)
		assert.False(t, retrieved.CreatedAt.IsZero())
		assert.False(t, retrieved.UpdatedAt.IsZero())
	})

	t.Run("update existing endpoint", func(t *testing.T) {
		endpoint := &Endpoint{
			ID:     "update-api",
			URL:    "https://api.example.com/v1",
			Method: "GET",
			Config: `{"timeout": "30s"}`,
		}

		// Save initial endpoint
		err := storage.SaveEndpoint(endpoint)
		require.NoError(t, err)

		// Get initial timestamps
		initial, err := storage.GetEndpoint("update-api")
		require.NoError(t, err)

		// Wait a bit to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Update endpoint
		endpoint.URL = "https://api.example.com/v2"
		err = storage.SaveEndpoint(endpoint)
		require.NoError(t, err)

		// Get updated endpoint
		updated, err := storage.GetEndpoint("update-api")
		require.NoError(t, err)

		assert.Equal(t, "https://api.example.com/v2", updated.URL)
		assert.Equal(t, initial.CreatedAt, updated.CreatedAt)      // CreatedAt should not change
		assert.True(t, updated.UpdatedAt.After(initial.UpdatedAt)) // UpdatedAt should be newer
	})

	t.Run("get nonexistent endpoint", func(t *testing.T) {
		_, err := storage.GetEndpoint("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("save nil endpoint", func(t *testing.T) {
		err := storage.SaveEndpoint(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("save endpoint with empty ID", func(t *testing.T) {
		endpoint := &Endpoint{
			URL:    "https://api.example.com/test",
			Method: "GET",
		}

		err := storage.SaveEndpoint(endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("list endpoints", func(t *testing.T) {
		// Clear any existing endpoints
		storage.Close()
		storage, _ = NewInMemoryStorage()
		defer storage.Close()

		// Add multiple endpoints
		endpoints := []*Endpoint{
			{ID: "api-1", URL: "https://api1.example.com", Method: "GET", Config: "{}"},
			{ID: "api-2", URL: "https://api2.example.com", Method: "POST", Config: "{}"},
			{ID: "api-3", URL: "https://api3.example.com", Method: "PUT", Config: "{}"},
		}

		for _, ep := range endpoints {
			err := storage.SaveEndpoint(ep)
			require.NoError(t, err)
		}

		// List endpoints
		listed, err := storage.ListEndpoints()
		require.NoError(t, err)

		assert.Len(t, listed, 3)

		// Verify sorting by ID
		assert.Equal(t, "api-1", listed[0].ID)
		assert.Equal(t, "api-2", listed[1].ID)
		assert.Equal(t, "api-3", listed[2].ID)
	})
}

func TestInMemoryStorage_MonitoringRuns(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("save and get monitoring runs", func(t *testing.T) {
		run := &MonitoringRun{
			EndpointID:      "test-api",
			Timestamp:       time.Now(),
			ResponseStatus:  200,
			ResponseTimeMs:  150,
			ResponseBody:    `{"test": "data"}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}

		// Save monitoring run
		err := storage.SaveMonitoringRun(run)
		require.NoError(t, err)

		// Get monitoring history
		runs, err := storage.GetMonitoringHistory("test-api", 24*time.Hour)
		require.NoError(t, err)

		assert.Len(t, runs, 1)
		retrieved := runs[0]

		assert.Equal(t, run.EndpointID, retrieved.EndpointID)
		assert.Equal(t, run.ResponseStatus, retrieved.ResponseStatus)
		assert.Equal(t, run.ResponseTimeMs, retrieved.ResponseTimeMs)
		assert.Equal(t, run.ResponseBody, retrieved.ResponseBody)
		assert.Equal(t, run.ResponseHeaders, retrieved.ResponseHeaders)
		assert.True(t, retrieved.ID > 0) // ID should be assigned
	})

	t.Run("get monitoring history with time filter", func(t *testing.T) {
		endpointID := "time-filter-api"
		now := time.Now()

		// Add runs at different times
		runs := []*MonitoringRun{
			{
				EndpointID:     endpointID,
				Timestamp:      now.Add(-2 * time.Hour), // 2 hours ago
				ResponseStatus: 200,
			},
			{
				EndpointID:     endpointID,
				Timestamp:      now.Add(-30 * time.Minute), // 30 minutes ago
				ResponseStatus: 201,
			},
			{
				EndpointID:     endpointID,
				Timestamp:      now.Add(-25 * time.Hour), // 25 hours ago (outside filter)
				ResponseStatus: 404,
			},
		}

		for _, run := range runs {
			err := storage.SaveMonitoringRun(run)
			require.NoError(t, err)
		}

		// Get runs from last hour (should get only the 30-minute-old run)
		recentRuns, err := storage.GetMonitoringHistory(endpointID, 1*time.Hour)
		require.NoError(t, err)

		assert.Len(t, recentRuns, 1)
		assert.Equal(t, 201, recentRuns[0].ResponseStatus)

		// Get runs from last 3 hours (should get 2 runs)
		longerRuns, err := storage.GetMonitoringHistory(endpointID, 3*time.Hour)
		require.NoError(t, err)

		assert.Len(t, longerRuns, 2)
		// Should be sorted by timestamp (most recent first)
		assert.Equal(t, 201, longerRuns[0].ResponseStatus) // 30 minutes ago
		assert.Equal(t, 200, longerRuns[1].ResponseStatus) // 2 hours ago
	})

	t.Run("get monitoring history for nonexistent endpoint", func(t *testing.T) {
		runs, err := storage.GetMonitoringHistory("nonexistent", 24*time.Hour)
		require.NoError(t, err)
		assert.Len(t, runs, 0)
	})

	t.Run("save nil monitoring run", func(t *testing.T) {
		err := storage.SaveMonitoringRun(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("save monitoring run with empty endpoint ID", func(t *testing.T) {
		run := &MonitoringRun{
			Timestamp:      time.Now(),
			ResponseStatus: 200,
		}

		err := storage.SaveMonitoringRun(run)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestInMemoryStorage_Drifts(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("save and get drifts", func(t *testing.T) {
		drift := &Drift{
			EndpointID:   "test-api",
			DetectedAt:   time.Now(),
			DriftType:    "field_removed",
			Severity:     "high",
			Description:  "Field 'user.id' was removed",
			BeforeValue:  "123",
			AfterValue:   "",
			FieldPath:    "$.user.id",
			Acknowledged: false,
		}

		// Save drift
		err := storage.SaveDrift(drift)
		require.NoError(t, err)

		// Get all drifts
		drifts, err := storage.GetDrifts(DriftFilters{})
		require.NoError(t, err)

		assert.Len(t, drifts, 1)
		retrieved := drifts[0]

		assert.Equal(t, drift.EndpointID, retrieved.EndpointID)
		assert.Equal(t, drift.DriftType, retrieved.DriftType)
		assert.Equal(t, drift.Severity, retrieved.Severity)
		assert.Equal(t, drift.Description, retrieved.Description)
		assert.Equal(t, drift.BeforeValue, retrieved.BeforeValue)
		assert.Equal(t, drift.AfterValue, retrieved.AfterValue)
		assert.Equal(t, drift.FieldPath, retrieved.FieldPath)
		assert.Equal(t, drift.Acknowledged, retrieved.Acknowledged)
		assert.True(t, retrieved.ID > 0) // ID should be assigned
	})

	t.Run("get drifts with filters", func(t *testing.T) {
		// Clear existing drifts
		storage.Close()
		storage, _ = NewInMemoryStorage()
		defer storage.Close()

		now := time.Now()
		drifts := []*Drift{
			{
				EndpointID:   "api-1",
				DetectedAt:   now.Add(-1 * time.Hour),
				DriftType:    "field_added",
				Severity:     "low",
				Description:  "Field added",
				Acknowledged: false,
			},
			{
				EndpointID:   "api-1",
				DetectedAt:   now.Add(-30 * time.Minute),
				DriftType:    "field_removed",
				Severity:     "high",
				Description:  "Field removed",
				Acknowledged: true,
			},
			{
				EndpointID:   "api-2",
				DetectedAt:   now.Add(-15 * time.Minute),
				DriftType:    "type_change",
				Severity:     "critical",
				Description:  "Type changed",
				Acknowledged: false,
			},
		}

		for _, drift := range drifts {
			err := storage.SaveDrift(drift)
			require.NoError(t, err)
		}

		// Filter by endpoint
		api1Drifts, err := storage.GetDrifts(DriftFilters{EndpointID: "api-1"})
		require.NoError(t, err)
		assert.Len(t, api1Drifts, 2)

		// Filter by severity
		highDrifts, err := storage.GetDrifts(DriftFilters{Severity: "high"})
		require.NoError(t, err)
		assert.Len(t, highDrifts, 1)
		assert.Equal(t, "field_removed", highDrifts[0].DriftType)

		// Filter by acknowledged status
		acknowledged := true
		ackedDrifts, err := storage.GetDrifts(DriftFilters{Acknowledged: &acknowledged})
		require.NoError(t, err)
		assert.Len(t, ackedDrifts, 1)
		assert.True(t, ackedDrifts[0].Acknowledged)

		unacknowledged := false
		unackedDrifts, err := storage.GetDrifts(DriftFilters{Acknowledged: &unacknowledged})
		require.NoError(t, err)
		assert.Len(t, unackedDrifts, 2)

		// Filter by time range
		timeFiltered, err := storage.GetDrifts(DriftFilters{
			StartTime: now.Add(-45 * time.Minute),
			EndTime:   now.Add(-10 * time.Minute),
		})
		require.NoError(t, err)
		assert.Len(t, timeFiltered, 2) // Should get the 30-minute and 15-minute old drifts
	})

	t.Run("save nil drift", func(t *testing.T) {
		err := storage.SaveDrift(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("save drift with empty endpoint ID", func(t *testing.T) {
		drift := &Drift{
			DetectedAt:  time.Now(),
			DriftType:   "field_added",
			Severity:    "low",
			Description: "Test drift",
		}

		err := storage.SaveDrift(drift)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestInMemoryStorage_Alerts(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("save and get alerts", func(t *testing.T) {
		alert := &Alert{
			DriftID:      1,
			AlertType:    "slack",
			ChannelName:  "api-alerts",
			SentAt:       time.Now(),
			Status:       "sent",
			ErrorMessage: "",
			RetryCount:   0,
		}

		// Save alert
		err := storage.SaveAlert(alert)
		require.NoError(t, err)

		// Get all alerts
		alerts, err := storage.GetAlerts(AlertFilters{})
		require.NoError(t, err)

		assert.Len(t, alerts, 1)
		retrieved := alerts[0]

		assert.Equal(t, alert.DriftID, retrieved.DriftID)
		assert.Equal(t, alert.AlertType, retrieved.AlertType)
		assert.Equal(t, alert.ChannelName, retrieved.ChannelName)
		assert.Equal(t, alert.Status, retrieved.Status)
		assert.Equal(t, alert.ErrorMessage, retrieved.ErrorMessage)
		assert.Equal(t, alert.RetryCount, retrieved.RetryCount)
		assert.True(t, retrieved.ID > 0) // ID should be assigned
	})

	t.Run("get alerts with filters", func(t *testing.T) {
		// Clear existing alerts
		storage.Close()
		storage, _ = NewInMemoryStorage()
		defer storage.Close()

		now := time.Now()
		alerts := []*Alert{
			{
				DriftID:     1,
				AlertType:   "slack",
				ChannelName: "alerts",
				SentAt:      now.Add(-1 * time.Hour),
				Status:      "sent",
			},
			{
				DriftID:     2,
				AlertType:   "email",
				ChannelName: "admin@example.com",
				SentAt:      now.Add(-30 * time.Minute),
				Status:      "failed",
			},
			{
				DriftID:     1,
				AlertType:   "slack",
				ChannelName: "alerts",
				SentAt:      now.Add(-15 * time.Minute),
				Status:      "sent",
			},
		}

		for _, alert := range alerts {
			err := storage.SaveAlert(alert)
			require.NoError(t, err)
		}

		// Filter by drift ID
		driftID := int64(1)
		drift1Alerts, err := storage.GetAlerts(AlertFilters{DriftID: &driftID})
		require.NoError(t, err)
		assert.Len(t, drift1Alerts, 2)

		// Filter by alert type
		slackAlerts, err := storage.GetAlerts(AlertFilters{AlertType: "slack"})
		require.NoError(t, err)
		assert.Len(t, slackAlerts, 2)

		// Filter by status
		failedAlerts, err := storage.GetAlerts(AlertFilters{Status: "failed"})
		require.NoError(t, err)
		assert.Len(t, failedAlerts, 1)
		assert.Equal(t, "email", failedAlerts[0].AlertType)

		// Filter by time range
		timeFiltered, err := storage.GetAlerts(AlertFilters{
			StartTime: now.Add(-45 * time.Minute),
			EndTime:   now.Add(-10 * time.Minute),
		})
		require.NoError(t, err)
		assert.Len(t, timeFiltered, 2)
	})

	t.Run("save nil alert", func(t *testing.T) {
		err := storage.SaveAlert(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestInMemoryStorage_Close(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)

	// Add some data
	endpoint := &Endpoint{ID: "test", URL: "https://test.com", Method: "GET", Config: "{}"}
	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	run := &MonitoringRun{EndpointID: "test", Timestamp: time.Now(), ResponseStatus: 200}
	err = storage.SaveMonitoringRun(run)
	require.NoError(t, err)

	// Verify data exists
	endpoints, err := storage.ListEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, 1)

	// Close storage
	err = storage.Close()
	require.NoError(t, err)

	// Verify data is cleared
	endpoints, err = storage.ListEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, 0)
}

func TestInMemoryStorage_GetDatabaseStats(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	// Add test data
	endpoint := &Endpoint{ID: "test", URL: "https://test.com", Method: "GET", Config: "{}"}
	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	run := &MonitoringRun{EndpointID: "test", Timestamp: time.Now(), ResponseStatus: 200}
	err = storage.SaveMonitoringRun(run)
	require.NoError(t, err)

	drift := &Drift{EndpointID: "test", DetectedAt: time.Now(), DriftType: "test", Severity: "low", Description: "test"}
	err = storage.SaveDrift(drift)
	require.NoError(t, err)

	alert := &Alert{DriftID: 1, AlertType: "test", ChannelName: "test", Status: "sent"}
	err = storage.SaveAlert(alert)
	require.NoError(t, err)

	// Get stats
	stats, err := storage.GetDatabaseStats()
	require.NoError(t, err)

	assert.Equal(t, int64(1), stats.Endpoints)
	assert.Equal(t, int64(1), stats.MonitoringRuns)
	assert.Equal(t, int64(1), stats.Drifts)
	assert.Equal(t, int64(1), stats.Alerts)
	assert.Equal(t, int64(0), stats.DatabaseSizeBytes) // Not applicable for in-memory
}

func TestInMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	// Test concurrent writes and reads
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines performing operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Save endpoint
				endpoint := &Endpoint{
					ID:     fmt.Sprintf("endpoint-%d-%d", id, j),
					URL:    fmt.Sprintf("https://api%d.example.com/%d", id, j),
					Method: "GET",
					Config: "{}",
				}
				storage.SaveEndpoint(endpoint)

				// Save monitoring run
				run := &MonitoringRun{
					EndpointID:     endpoint.ID,
					Timestamp:      time.Now(),
					ResponseStatus: 200 + j,
				}
				storage.SaveMonitoringRun(run)

				// Read operations
				storage.GetEndpoint(endpoint.ID)
				storage.GetMonitoringHistory(endpoint.ID, time.Hour)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state
	endpoints, err := storage.ListEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, numGoroutines*numOperations)
}

func TestInMemoryStorage_Cleanup(t *testing.T) {
	storage, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	now := time.Now()
	cutoff := now.AddDate(0, 0, -7) // 7 days ago

	// Add old and new monitoring runs
	oldRun := &MonitoringRun{
		EndpointID:     "test",
		Timestamp:      now.AddDate(0, 0, -10), // 10 days old
		ResponseStatus: 200,
	}
	err = storage.SaveMonitoringRun(oldRun)
	require.NoError(t, err)

	newRun := &MonitoringRun{
		EndpointID:     "test",
		Timestamp:      now.AddDate(0, 0, -3), // 3 days old
		ResponseStatus: 200,
	}
	err = storage.SaveMonitoringRun(newRun)
	require.NoError(t, err)

	// Add old and new drifts
	oldDrift := &Drift{
		EndpointID:  "test",
		DetectedAt:  now.AddDate(0, 0, -10), // 10 days old
		DriftType:   "test",
		Severity:    "low",
		Description: "old drift",
	}
	err = storage.SaveDrift(oldDrift)
	require.NoError(t, err)

	newDrift := &Drift{
		EndpointID:  "test",
		DetectedAt:  now.AddDate(0, 0, -3), // 3 days old
		DriftType:   "test",
		Severity:    "low",
		Description: "new drift",
	}
	err = storage.SaveDrift(newDrift)
	require.NoError(t, err)

	// Add old and new alerts
	oldAlert := &Alert{
		DriftID:     1,
		AlertType:   "test",
		ChannelName: "test",
		SentAt:      now.AddDate(0, 0, -10), // 10 days old
		Status:      "sent",
	}
	err = storage.SaveAlert(oldAlert)
	require.NoError(t, err)

	newAlert := &Alert{
		DriftID:     2,
		AlertType:   "test",
		ChannelName: "test",
		SentAt:      now.AddDate(0, 0, -3), // 3 days old
		Status:      "sent",
	}
	err = storage.SaveAlert(newAlert)
	require.NoError(t, err)

	// Test cleanup monitoring runs
	cleaned, err := storage.CleanupOldMonitoringRuns(cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleaned)

	// Verify only new run remains
	runs, err := storage.GetMonitoringHistory("test", 30*24*time.Hour)
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.True(t, runs[0].Timestamp.After(cutoff))

	// Test cleanup drifts
	cleaned, err = storage.CleanupOldDrifts(cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleaned)

	// Verify only new drift remains
	drifts, err := storage.GetDrifts(DriftFilters{})
	require.NoError(t, err)
	assert.Len(t, drifts, 1)
	assert.True(t, drifts[0].DetectedAt.After(cutoff))

	// Test cleanup alerts
	cleaned, err = storage.CleanupOldAlerts(cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleaned)

	// Verify only new alert remains
	alerts, err := storage.GetAlerts(AlertFilters{})
	require.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.True(t, alerts[0].SentAt.After(cutoff))

	// Test vacuum (should be no-op for memory storage)
	err = storage.VacuumDatabase()
	assert.NoError(t, err)
}
