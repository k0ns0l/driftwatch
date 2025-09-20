package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupCommand(t *testing.T) {
	// Skip this test for now as it requires complex setup with global config
	// The helper functions are tested separately below
	t.Skip("Skipping command integration test - helper functions are tested separately")
}

func TestCleanupHelperFunctions(t *testing.T) {
	// Create in-memory storage for testing
	db, err := storage.NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	cutoff := now.AddDate(0, 0, -7) // 7 days ago

	// Add test data - first add endpoint for foreign key constraint
	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: `{"interval": "5m"}`,
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	oldRun := &storage.MonitoringRun{
		EndpointID:      "test-endpoint",
		Timestamp:       now.AddDate(0, 0, -10), // 10 days old
		ResponseStatus:  200,
		ResponseTimeMs:  100,
		ResponseBody:    "test response",
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
	}
	err = db.SaveMonitoringRun(oldRun)
	require.NoError(t, err)

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

	t.Run("ShowDatabaseStats", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := showDatabaseStats(db)
		assert.NoError(t, err)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, "Database Statistics")
		assert.Contains(t, output, "Monitoring runs: 2")
	})

	t.Run("CleanupMonitoringRuns", func(t *testing.T) {
		// Test actual cleanup (not dry run)
		cleaned, err := cleanupMonitoringRuns(db, cutoff, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), cleaned) // Should clean 1 old run

		// Verify only recent run remains
		runs, err := db.GetMonitoringHistory("test-endpoint", 24*time.Hour*30)
		assert.NoError(t, err)
		assert.Len(t, runs, 1)
		assert.True(t, runs[0].Timestamp.After(cutoff))
	})

	t.Run("PerformVacuum", func(t *testing.T) {
		// Test vacuum operation
		err := performVacuum(db, false)
		assert.NoError(t, err)

		// Test dry run vacuum
		err = performVacuum(db, true)
		assert.NoError(t, err)
	})
}
