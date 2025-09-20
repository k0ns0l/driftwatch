package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*SQLiteStorage, func()) {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "driftwatch_test_*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		storage.Close()
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNewSQLiteStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	assert.NotNil(t, storage)
	assert.NotNil(t, storage.db)
}

func TestSaveAndGetEndpoint(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create test endpoint
	endpoint := &Endpoint{
		ID:       "test-endpoint-1",
		URL:      "https://api.example.com/users",
		Method:   "GET",
		SpecFile: "/path/to/spec.yaml",
		Config:   `{"timeout": "30s", "retry_count": 3}`,
	}

	// Save endpoint
	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)
	assert.False(t, endpoint.CreatedAt.IsZero())
	assert.False(t, endpoint.UpdatedAt.IsZero())

	// Get endpoint
	retrieved, err := storage.GetEndpoint("test-endpoint-1")
	require.NoError(t, err)
	assert.Equal(t, endpoint.ID, retrieved.ID)
	assert.Equal(t, endpoint.URL, retrieved.URL)
	assert.Equal(t, endpoint.Method, retrieved.Method)
	assert.Equal(t, endpoint.SpecFile, retrieved.SpecFile)
	assert.Equal(t, endpoint.Config, retrieved.Config)
	assert.WithinDuration(t, endpoint.CreatedAt, retrieved.CreatedAt, time.Second)
	assert.WithinDuration(t, endpoint.UpdatedAt, retrieved.UpdatedAt, time.Second)
}

func TestGetEndpointNotFound(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := storage.GetEndpoint("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint not found")
}

func TestSaveEndpointUpdate(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create and save endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint-1",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}

	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)
	originalUpdatedAt := endpoint.UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update endpoint
	endpoint.URL = "https://api.example.com/v2/users"
	endpoint.Config = `{"timeout": "60s"}`

	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)
	assert.True(t, endpoint.UpdatedAt.After(originalUpdatedAt))

	// Verify update
	retrieved, err := storage.GetEndpoint("test-endpoint-1")
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v2/users", retrieved.URL)
	assert.Equal(t, `{"timeout": "60s"}`, retrieved.Config)
}

func TestListEndpoints(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create test endpoints
	endpoints := []*Endpoint{
		{
			ID:     "endpoint-1",
			URL:    "https://api.example.com/users",
			Method: "GET",
			Config: `{"timeout": "30s"}`,
		},
		{
			ID:     "endpoint-2",
			URL:    "https://api.example.com/posts",
			Method: "POST",
			Config: `{"timeout": "45s"}`,
		},
		{
			ID:       "endpoint-3",
			URL:      "https://api.example.com/comments",
			Method:   "GET",
			SpecFile: "/path/to/spec.yaml",
			Config:   `{"timeout": "60s"}`,
		},
	}

	// Save endpoints
	for _, endpoint := range endpoints {
		err := storage.SaveEndpoint(endpoint)
		require.NoError(t, err)
	}

	// List endpoints
	retrieved, err := storage.ListEndpoints()
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)

	// Verify endpoints are returned in creation order (newest first)
	assert.Equal(t, "endpoint-3", retrieved[0].ID)
	assert.Equal(t, "endpoint-2", retrieved[1].ID)
	assert.Equal(t, "endpoint-1", retrieved[2].ID)
}

func TestListEndpointsEmpty(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	endpoints, err := storage.ListEndpoints()
	require.NoError(t, err)
	assert.Empty(t, endpoints)
}

func TestSaveAndGetMonitoringRun(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// First create an endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Create test monitoring run
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
	}

	run := &MonitoringRun{
		EndpointID:       "test-endpoint",
		ResponseStatus:   200,
		ResponseTimeMs:   150,
		ResponseBody:     `{"users": [{"id": 1, "name": "John"}]}`,
		ResponseHeaders:  headers,
		ValidationResult: `{"valid": true, "errors": []}`,
	}

	// Save monitoring run
	err = storage.SaveMonitoringRun(run)
	require.NoError(t, err)
	assert.NotZero(t, run.ID)
	assert.False(t, run.Timestamp.IsZero())

	// Get monitoring history
	history, err := storage.GetMonitoringHistory("test-endpoint", 24*time.Hour)
	require.NoError(t, err)
	assert.Len(t, history, 1)

	retrieved := history[0]
	assert.Equal(t, run.ID, retrieved.ID)
	assert.Equal(t, run.EndpointID, retrieved.EndpointID)
	assert.Equal(t, run.ResponseStatus, retrieved.ResponseStatus)
	assert.Equal(t, run.ResponseTimeMs, retrieved.ResponseTimeMs)
	assert.Equal(t, run.ResponseBody, retrieved.ResponseBody)
	assert.Equal(t, run.ValidationResult, retrieved.ValidationResult)
	assert.Equal(t, headers, retrieved.ResponseHeaders)
	assert.WithinDuration(t, run.Timestamp, retrieved.Timestamp, time.Second)
}

func TestGetMonitoringHistoryWithPeriod(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Create monitoring runs with different timestamps
	now := time.Now()
	runs := []*MonitoringRun{
		{
			EndpointID:      "test-endpoint",
			Timestamp:       now.Add(-2 * time.Hour),
			ResponseStatus:  200,
			ResponseTimeMs:  100,
			ResponseBody:    `{"count": 1}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			EndpointID:      "test-endpoint",
			Timestamp:       now.Add(-25 * time.Hour), // Outside 24h window
			ResponseStatus:  200,
			ResponseTimeMs:  120,
			ResponseBody:    `{"count": 2}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			EndpointID:      "test-endpoint",
			Timestamp:       now.Add(-1 * time.Hour),
			ResponseStatus:  500,
			ResponseTimeMs:  200,
			ResponseBody:    `{"error": "Internal server error"}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		},
	}

	// Save all runs
	for _, run := range runs {
		err := storage.SaveMonitoringRun(run)
		require.NoError(t, err)
	}

	// Get history for last 24 hours
	history, err := storage.GetMonitoringHistory("test-endpoint", 24*time.Hour)
	require.NoError(t, err)
	assert.Len(t, history, 2) // Should exclude the 25-hour old run

	// Verify order (newest first)
	assert.Equal(t, 500, history[0].ResponseStatus) // 1 hour ago
	assert.Equal(t, 200, history[1].ResponseStatus) // 2 hours ago
}

func TestSaveAndGetDrift(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Create test drift
	drift := &Drift{
		EndpointID:   "test-endpoint",
		DriftType:    "field_removed",
		Severity:     "high",
		Description:  "Required field 'email' was removed",
		BeforeValue:  `{"id": 1, "name": "John", "email": "john@example.com"}`,
		AfterValue:   `{"id": 1, "name": "John"}`,
		FieldPath:    "email",
		Acknowledged: false,
	}

	// Save drift
	err = storage.SaveDrift(drift)
	require.NoError(t, err)
	assert.NotZero(t, drift.ID)
	assert.False(t, drift.DetectedAt.IsZero())

	// Get drifts
	drifts, err := storage.GetDrifts(DriftFilters{})
	require.NoError(t, err)
	assert.Len(t, drifts, 1)

	retrieved := drifts[0]
	assert.Equal(t, drift.ID, retrieved.ID)
	assert.Equal(t, drift.EndpointID, retrieved.EndpointID)
	assert.Equal(t, drift.DriftType, retrieved.DriftType)
	assert.Equal(t, drift.Severity, retrieved.Severity)
	assert.Equal(t, drift.Description, retrieved.Description)
	assert.Equal(t, drift.BeforeValue, retrieved.BeforeValue)
	assert.Equal(t, drift.AfterValue, retrieved.AfterValue)
	assert.Equal(t, drift.FieldPath, retrieved.FieldPath)
	assert.Equal(t, drift.Acknowledged, retrieved.Acknowledged)
	assert.WithinDuration(t, drift.DetectedAt, retrieved.DetectedAt, time.Second)
}

func TestGetDriftsWithFilters(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create endpoints
	endpoint1 := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err := storage.SaveEndpoint(endpoint1)
	require.NoError(t, err)

	endpoint2 := &Endpoint{
		ID:     "other-endpoint",
		URL:    "https://api.example.com/posts",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err = storage.SaveEndpoint(endpoint2)
	require.NoError(t, err)

	// Create test drifts
	now := time.Now()
	drifts := []*Drift{
		{
			EndpointID:   "test-endpoint",
			DetectedAt:   now.Add(-2 * time.Hour),
			DriftType:    "field_added",
			Severity:     "low",
			Description:  "New field 'created_at' added",
			FieldPath:    "created_at",
			Acknowledged: false,
		},
		{
			EndpointID:   "test-endpoint",
			DetectedAt:   now.Add(-1 * time.Hour),
			DriftType:    "field_removed",
			Severity:     "high",
			Description:  "Required field 'email' removed",
			FieldPath:    "email",
			Acknowledged: true,
		},
		{
			EndpointID:   "other-endpoint",
			DetectedAt:   now.Add(-30 * time.Minute),
			DriftType:    "type_changed",
			Severity:     "medium",
			Description:  "Field 'age' changed from string to number",
			FieldPath:    "age",
			Acknowledged: false,
		},
	}

	// Save all drifts
	for _, drift := range drifts {
		err := storage.SaveDrift(drift)
		require.NoError(t, err)
	}

	// Test filter by endpoint ID
	filtered, err := storage.GetDrifts(DriftFilters{EndpointID: "test-endpoint"})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)

	// Test filter by severity
	filtered, err = storage.GetDrifts(DriftFilters{Severity: "high"})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "field_removed", filtered[0].DriftType)

	// Test filter by acknowledged status
	acknowledged := false
	filtered, err = storage.GetDrifts(DriftFilters{Acknowledged: &acknowledged})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)

	acknowledged = true
	filtered, err = storage.GetDrifts(DriftFilters{Acknowledged: &acknowledged})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "field_removed", filtered[0].DriftType)

	// Test filter by time range
	filtered, err = storage.GetDrifts(DriftFilters{
		StartTime: now.Add(-90 * time.Minute),
		EndTime:   now.Add(-45 * time.Minute),
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "field_removed", filtered[0].DriftType)

	// Test combined filters
	filtered, err = storage.GetDrifts(DriftFilters{
		EndpointID: "test-endpoint",
		Severity:   "low",
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "field_added", filtered[0].DriftType)
}

func TestDatabaseMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create storage instance (should run migration)
	storage1, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)

	// Add some data
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err = storage1.SaveEndpoint(endpoint)
	require.NoError(t, err)
	storage1.Close()

	// Create another storage instance with same database (should not fail)
	storage2, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage2.Close()

	// Verify data is still there
	retrieved, err := storage2.GetEndpoint("test-endpoint")
	require.NoError(t, err)
	assert.Equal(t, endpoint.ID, retrieved.ID)
}

func TestForeignKeyConstraints(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Try to save monitoring run without endpoint (should fail due to foreign key)
	run := &MonitoringRun{
		EndpointID:      "non-existent-endpoint",
		ResponseStatus:  200,
		ResponseTimeMs:  100,
		ResponseBody:    `{"test": true}`,
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
	}

	err := storage.SaveMonitoringRun(run)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed")
}

func TestJSONSerialization(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	// Create endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}
	err := storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Test with complex headers
	complexHeaders := map[string]string{
		"Content-Type":    "application/json; charset=utf-8",
		"Authorization":   "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9",
		"X-Custom-Header": "value with spaces and special chars: !@#$%",
		"Accept-Language": "en-US,en;q=0.9",
	}

	run := &MonitoringRun{
		EndpointID:      "test-endpoint",
		ResponseStatus:  200,
		ResponseTimeMs:  150,
		ResponseBody:    `{"users": [{"id": 1, "name": "John Doe", "email": "john@example.com"}]}`,
		ResponseHeaders: complexHeaders,
	}

	err = storage.SaveMonitoringRun(run)
	require.NoError(t, err)

	// Retrieve and verify JSON serialization worked correctly
	history, err := storage.GetMonitoringHistory("test-endpoint", time.Hour)
	require.NoError(t, err)
	assert.Len(t, history, 1)

	retrieved := history[0]
	assert.Equal(t, complexHeaders, retrieved.ResponseHeaders)
}

func TestClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)

	// Close should not error
	err = storage.Close()
	assert.NoError(t, err)

	// Second close should not error
	err = storage.Close()
	assert.NoError(t, err)
}
