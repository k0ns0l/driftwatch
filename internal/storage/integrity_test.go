package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteIntegrityCheck(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "driftwatch_integrity_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Test with healthy database
	result, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Empty(t, result.Issues)
	assert.Equal(t, 5, result.TablesChecked) // endpoints, monitoring_runs, drifts, alerts, schema_version

	// Add test data
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: "{}",
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Add monitoring run
	run := &MonitoringRun{
		EndpointID:     "test-endpoint",
		Timestamp:      time.Now(),
		ResponseStatus: 200,
	}
	err = db.SaveMonitoringRun(run)
	require.NoError(t, err)

	// Check integrity again - should still be healthy
	result, err = db.CheckIntegrity()
	require.NoError(t, err)
	assert.True(t, result.Healthy)

	// Create orphaned monitoring run by directly inserting into database
	// Temporarily disable foreign keys to create orphaned record
	_, err = db.db.Exec("PRAGMA foreign_keys = OFF")
	require.NoError(t, err)

	_, err = db.db.Exec(`
		INSERT INTO monitoring_runs (endpoint_id, timestamp, response_status)
		VALUES ('non-existent-endpoint', ?, 200)
	`, time.Now())
	require.NoError(t, err)

	// Re-enable foreign keys
	_, err = db.db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	// Check integrity - should find orphaned record
	result, err = db.CheckIntegrity()
	require.NoError(t, err)
	assert.False(t, result.Healthy)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, "orphaned_record", result.Issues[0].Type)
	assert.Equal(t, "monitoring_runs", result.Issues[0].Table)
	assert.True(t, result.Issues[0].Repairable)
	assert.Equal(t, int64(1), result.OrphanedRecords["monitoring_runs"])
}

func TestSQLiteRepairDatabase(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "driftwatch_repair_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Add test endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: "{}",
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Create orphaned monitoring run
	// Temporarily disable foreign keys to create orphaned record
	_, err = db.db.Exec("PRAGMA foreign_keys = OFF")
	require.NoError(t, err)

	_, err = db.db.Exec(`
		INSERT INTO monitoring_runs (endpoint_id, timestamp, response_status)
		VALUES ('non-existent-endpoint', ?, 200)
	`, time.Now())
	require.NoError(t, err)

	// Re-enable foreign keys
	_, err = db.db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	// Verify integrity issue exists
	integrityResult, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.False(t, integrityResult.Healthy)
	assert.Len(t, integrityResult.Issues, 1)

	// Repair database
	repairResult, err := db.RepairDatabase()
	require.NoError(t, err)
	assert.True(t, repairResult.Success)
	assert.Equal(t, 1, repairResult.IssuesRepaired)
	assert.Len(t, repairResult.Actions, 2) // delete orphaned + rebuild indexes
	assert.NotEmpty(t, repairResult.BackupCreated)

	// Verify backup was created
	_, err = os.Stat(repairResult.BackupCreated)
	assert.NoError(t, err)

	// Verify integrity is now healthy
	finalIntegrityResult, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.True(t, finalIntegrityResult.Healthy)
	assert.Empty(t, finalIntegrityResult.Issues)
}

func TestSQLiteHealthStatus(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "driftwatch_health_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Get health status
	health, err := db.GetHealthStatus()
	require.NoError(t, err)
	assert.True(t, health.Healthy)
	assert.Equal(t, "excellent", health.Status)
	assert.Equal(t, 0, health.IntegrityIssues)
	assert.Equal(t, 1, health.SchemaVersion)
	assert.True(t, health.FragmentationLevel >= 0)

	// Check recommendations (may vary based on database size and state)
	// For a fresh database, we might not have backup recommendations
	// Let's just verify we get some recommendations structure
	assert.NotNil(t, health.Recommendations)

	// Print recommendations for debugging
	t.Logf("Health recommendations: %+v", health.Recommendations)
}

func TestInMemoryIntegrityCheck(t *testing.T) {
	db, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Test with healthy in-memory database
	result, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Empty(t, result.Issues)
	assert.Equal(t, 4, result.TablesChecked)

	// Add endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: "{}",
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Add drift for non-existent endpoint (simulate orphaned record)
	memDB := db.(*InMemoryStorage)
	memDB.mu.Lock()
	orphanedDrift := &Drift{
		ID:         1,
		EndpointID: "non-existent-endpoint",
		DetectedAt: time.Now(),
		DriftType:  "test",
		Severity:   "low",
	}
	memDB.drifts = append(memDB.drifts, orphanedDrift)
	memDB.mu.Unlock()

	// Check integrity - should find orphaned drift
	result, err = db.CheckIntegrity()
	require.NoError(t, err)
	assert.False(t, result.Healthy)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, "orphaned_record", result.Issues[0].Type)
	assert.Equal(t, "drifts", result.Issues[0].Table)
	assert.Equal(t, int64(1), result.OrphanedRecords["drifts"])
}

func TestInMemoryRepairDatabase(t *testing.T) {
	db, err := NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Add endpoint
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: "{}",
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	// Add orphaned drift
	memDB := db.(*InMemoryStorage)
	memDB.mu.Lock()
	orphanedDrift := &Drift{
		ID:         1,
		EndpointID: "non-existent-endpoint",
		DetectedAt: time.Now(),
		DriftType:  "test",
		Severity:   "low",
	}
	memDB.drifts = append(memDB.drifts, orphanedDrift)
	memDB.mu.Unlock()

	// Verify integrity issue exists
	integrityResult, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.False(t, integrityResult.Healthy)

	// Repair database
	repairResult, err := db.RepairDatabase()
	require.NoError(t, err)
	assert.True(t, repairResult.Success)
	assert.Equal(t, 1, repairResult.IssuesRepaired)
	assert.Len(t, repairResult.Actions, 1)

	// Verify integrity is now healthy
	finalIntegrityResult, err := db.CheckIntegrity()
	require.NoError(t, err)
	assert.True(t, finalIntegrityResult.Healthy)
}
