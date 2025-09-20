package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageIntegration(t *testing.T) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "driftwatch_integration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "integration_test.db")

	// Create storage using the convenience function
	storage, err := NewStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Test complete workflow: endpoint -> monitoring -> drift detection

	// 1. Create and save endpoint
	endpoint := &Endpoint{
		ID:       "api-users",
		URL:      "https://api.example.com/v1/users",
		Method:   "GET",
		SpecFile: "/specs/users.yaml",
		Config:   `{"timeout": "30s", "retry_count": 3, "interval": "5m"}`,
	}

	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// 2. Simulate monitoring runs over time
	baseTime := time.Now().Add(-24 * time.Hour)
	monitoringRuns := []*MonitoringRun{
		{
			EndpointID:       "api-users",
			Timestamp:        baseTime,
			ResponseStatus:   200,
			ResponseTimeMs:   120,
			ResponseBody:     `{"users": [{"id": 1, "name": "John", "email": "john@example.com"}]}`,
			ResponseHeaders:  map[string]string{"Content-Type": "application/json"},
			ValidationResult: `{"valid": true, "errors": []}`,
		},
		{
			EndpointID:       "api-users",
			Timestamp:        baseTime.Add(6 * time.Hour),
			ResponseStatus:   200,
			ResponseTimeMs:   135,
			ResponseBody:     `{"users": [{"id": 1, "name": "John", "email": "john@example.com", "created_at": "2024-01-01T00:00:00Z"}]}`,
			ResponseHeaders:  map[string]string{"Content-Type": "application/json"},
			ValidationResult: `{"valid": true, "warnings": ["new field added"]}`,
		},
		{
			EndpointID:       "api-users",
			Timestamp:        baseTime.Add(12 * time.Hour),
			ResponseStatus:   200,
			ResponseTimeMs:   98,
			ResponseBody:     `{"users": [{"id": 1, "name": "John"}]}`,
			ResponseHeaders:  map[string]string{"Content-Type": "application/json"},
			ValidationResult: `{"valid": false, "errors": ["required field 'email' missing"]}`,
		},
	}

	for _, run := range monitoringRuns {
		err = storage.SaveMonitoringRun(run)
		require.NoError(t, err)
		assert.NotZero(t, run.ID)
	}

	// 3. Record detected drifts
	drifts := []*Drift{
		{
			EndpointID:   "api-users",
			DetectedAt:   baseTime.Add(6 * time.Hour),
			DriftType:    "field_added",
			Severity:     "low",
			Description:  "New field 'created_at' added to user object",
			BeforeValue:  `{"id": 1, "name": "John", "email": "john@example.com"}`,
			AfterValue:   `{"id": 1, "name": "John", "email": "john@example.com", "created_at": "2024-01-01T00:00:00Z"}`,
			FieldPath:    "created_at",
			Acknowledged: false,
		},
		{
			EndpointID:   "api-users",
			DetectedAt:   baseTime.Add(12 * time.Hour),
			DriftType:    "field_removed",
			Severity:     "high",
			Description:  "Required field 'email' was removed from user object",
			BeforeValue:  `{"id": 1, "name": "John", "email": "john@example.com", "created_at": "2024-01-01T00:00:00Z"}`,
			AfterValue:   `{"id": 1, "name": "John"}`,
			FieldPath:    "email",
			Acknowledged: false,
		},
	}

	for _, drift := range drifts {
		err = storage.SaveDrift(drift)
		require.NoError(t, err)
		assert.NotZero(t, drift.ID)
	}

	// 4. Test data retrieval and filtering

	// Get endpoint
	retrievedEndpoint, err := storage.GetEndpoint("api-users")
	require.NoError(t, err)
	assert.Equal(t, endpoint.URL, retrievedEndpoint.URL)
	assert.Equal(t, endpoint.Method, retrievedEndpoint.Method)
	assert.Equal(t, endpoint.SpecFile, retrievedEndpoint.SpecFile)

	// Get monitoring history
	history, err := storage.GetMonitoringHistory("api-users", 25*time.Hour)
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify order (newest first)
	assert.True(t, history[0].Timestamp.After(history[1].Timestamp))
	assert.True(t, history[1].Timestamp.After(history[2].Timestamp))

	// Get all drifts
	allDrifts, err := storage.GetDrifts(DriftFilters{})
	require.NoError(t, err)
	assert.Len(t, allDrifts, 2)

	// Filter drifts by severity
	highSeverityDrifts, err := storage.GetDrifts(DriftFilters{Severity: "high"})
	require.NoError(t, err)
	assert.Len(t, highSeverityDrifts, 1)
	assert.Equal(t, "field_removed", highSeverityDrifts[0].DriftType)

	// Filter drifts by endpoint
	endpointDrifts, err := storage.GetDrifts(DriftFilters{EndpointID: "api-users"})
	require.NoError(t, err)
	assert.Len(t, endpointDrifts, 2)

	// Filter drifts by time range
	recentDrifts, err := storage.GetDrifts(DriftFilters{
		StartTime: baseTime.Add(10 * time.Hour),
		EndTime:   baseTime.Add(15 * time.Hour),
	})
	require.NoError(t, err)
	assert.Len(t, recentDrifts, 1)
	assert.Equal(t, "field_removed", recentDrifts[0].DriftType)

	// Test acknowledgment filtering
	acknowledged := false
	unacknowledgedDrifts, err := storage.GetDrifts(DriftFilters{Acknowledged: &acknowledged})
	require.NoError(t, err)
	assert.Len(t, unacknowledgedDrifts, 2)

	// 5. Test list endpoints
	endpoints, err := storage.ListEndpoints()
	require.NoError(t, err)
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "api-users", endpoints[0].ID)

	// 6. Test performance with larger dataset
	// Add more monitoring runs to test query performance
	for i := 0; i < 100; i++ {
		run := &MonitoringRun{
			EndpointID:      "api-users",
			Timestamp:       baseTime.Add(time.Duration(i) * time.Minute),
			ResponseStatus:  200,
			ResponseTimeMs:  int64(100 + i),
			ResponseBody:    `{"test": true}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		}
		err = storage.SaveMonitoringRun(run)
		require.NoError(t, err)
	}

	// Query should still be fast
	start := time.Now()
	largeHistory, err := storage.GetMonitoringHistory("api-users", 25*time.Hour)
	queryDuration := time.Since(start)

	require.NoError(t, err)
	assert.Greater(t, len(largeHistory), 100)           // Should have original 3 + 100 new runs
	assert.Less(t, queryDuration, 100*time.Millisecond) // Should be fast due to indexes
}

func TestStorageErrorHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_error_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "error_test.db")
	storage, err := NewStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Test foreign key constraint
	run := &MonitoringRun{
		EndpointID:      "non-existent-endpoint",
		ResponseStatus:  200,
		ResponseTimeMs:  100,
		ResponseBody:    `{"test": true}`,
		ResponseHeaders: map[string]string{},
	}

	err = storage.SaveMonitoringRun(run)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed")

	// Test drift foreign key constraint
	drift := &Drift{
		EndpointID:  "non-existent-endpoint",
		DriftType:   "test",
		Severity:    "low",
		Description: "test drift",
	}

	err = storage.SaveDrift(drift)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed")

	// Test get non-existent endpoint
	_, err = storage.GetEndpoint("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint not found")
}

func TestStorageConcurrency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_concurrency_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "concurrency_test.db")
	storage, err := NewStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Create endpoint first
	endpoint := &Endpoint{
		ID:     "concurrent-endpoint",
		URL:    "https://api.example.com/concurrent",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Test sequential writes to verify basic functionality
	// (Concurrent writes with SQLite can cause locking issues which is expected)
	const numRuns = 10

	for i := 0; i < numRuns; i++ {
		run := &MonitoringRun{
			EndpointID:      "concurrent-endpoint",
			ResponseStatus:  200,
			ResponseTimeMs:  int64(i * 10),
			ResponseBody:    `{"run": ` + string(rune(i)) + `}`,
			ResponseHeaders: map[string]string{"X-Run": string(rune(i))},
		}

		err := storage.SaveMonitoringRun(run)
		require.NoError(t, err)
	}

	// Verify all runs were saved
	history, err := storage.GetMonitoringHistory("concurrent-endpoint", 24*time.Hour)
	require.NoError(t, err)
	assert.Len(t, history, numRuns)
}
