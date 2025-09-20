package cmd

import (
	"os"
	"path/filepath"
	"testing"

	// "github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupCommand(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "driftwatch_backup_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test database
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := storage.NewSQLiteStorage(dbPath)
	require.NoError(t, err)

	// Add some test data
	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
		Config: "{}",
	}
	err = db.SaveEndpoint(endpoint)
	require.NoError(t, err)

	db.Close()

	// Test backup creation
	backupPath := filepath.Join(tempDir, "backup.db")

	// Reopen database for backup
	db, err = storage.NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer db.Close()

	err = db.BackupDatabase(backupPath)
	assert.NoError(t, err)

	// Verify backup file exists
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)

	// Verify backup contains data
	backupDB, err := storage.NewSQLiteStorage(backupPath)
	require.NoError(t, err)
	defer backupDB.Close()

	restoredEndpoint, err := backupDB.GetEndpoint("test-endpoint")
	assert.NoError(t, err)
	assert.Equal(t, endpoint.ID, restoredEndpoint.ID)
	assert.Equal(t, endpoint.URL, restoredEndpoint.URL)
}

func TestBackupCommandWithConfig(t *testing.T) {
	// This test would require setting up the full CLI context
	// For now, we'll test the core backup functionality
	t.Skip("CLI integration test - requires full setup")
}
