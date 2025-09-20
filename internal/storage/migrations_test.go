package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupTestMigrationDB(t *testing.T) (*sql.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_migration_test_*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestMigrationManager_GetCurrentVersion(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Initially should be version 0
	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, 0, version)

	// Verify schema_version table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_version'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMigrationManager_ApplyMigration(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Create a test migration
	migration := Migration{
		Version:     1,
		Description: "Test migration",
		SQL:         "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT);",
	}

	// Apply the migration
	err := mgr.applyMigration(migration)
	require.NoError(t, err)

	// Verify the table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the migration was recorded
	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrationManager_ApplyMigrationFailure(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Create a migration with invalid SQL
	migration := Migration{
		Version:     1,
		Description: "Invalid migration",
		SQL:         "INVALID SQL STATEMENT;",
	}

	// Apply the migration (should fail)
	err := mgr.applyMigration(migration)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute migration")

	// Verify no migration was recorded
	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, 0, version)
}

func TestMigrationManager_RunMigrations(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Run all migrations
	err := mgr.runMigrations()
	require.NoError(t, err)

	// Verify current version matches the latest migration
	migrations := getMigrations()
	expectedVersion := 0
	for _, migration := range migrations {
		if migration.Version > expectedVersion {
			expectedVersion = migration.Version
		}
	}

	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, version)

	// Verify all tables were created
	tables := []string{"endpoints", "monitoring_runs", "drifts", "alerts"}
	for _, table := range tables {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Table %s should exist", table)
	}
}

func TestMigrationManager_RunMigrationsIdempotent(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Run migrations twice
	err := mgr.runMigrations()
	require.NoError(t, err)

	err = mgr.runMigrations()
	require.NoError(t, err)

	// Verify version is still correct
	migrations := getMigrations()
	expectedVersion := 0
	for _, migration := range migrations {
		if migration.Version > expectedVersion {
			expectedVersion = migration.Version
		}
	}

	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, version)

	// Verify we don't have duplicate migration records
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, count)
}

func TestMigrationManager_PartialMigrations(t *testing.T) {
	db, cleanup := setupTestMigrationDB(t)
	defer cleanup()

	mgr := newMigrationManager(db)

	// Manually insert a migration record to simulate partial state
	// First ensure schema_version table exists
	_, err := mgr.getCurrentVersion()
	require.NoError(t, err)

	// Insert a fake migration record
	_, err = db.Exec("INSERT INTO schema_version (version) VALUES (?)", 0)
	require.NoError(t, err)

	// Now run all migrations - should apply all since we only have version 0
	err = mgr.runMigrations()
	require.NoError(t, err)

	// Verify the standard tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='endpoints'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify we have the correct version
	version, err := mgr.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, 1, version) // Should be 1 from our migration system
}

func TestGetMigrations(t *testing.T) {
	migrations := getMigrations()

	// Should have at least one migration
	assert.NotEmpty(t, migrations)

	// Verify migrations are properly structured
	for _, migration := range migrations {
		assert.Greater(t, migration.Version, 0, "Migration version should be positive")
		assert.NotEmpty(t, migration.Description, "Migration should have description")
		assert.NotEmpty(t, migration.SQL, "Migration should have SQL")
	}

	// Verify migrations are in order
	for i := 1; i < len(migrations); i++ {
		assert.Greater(t, migrations[i].Version, migrations[i-1].Version,
			"Migrations should be in ascending version order")
	}
}

func TestSQLiteStorageWithMigrations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "driftwatch_storage_migration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create storage (should run migrations)
	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Verify we can use the storage normally
	endpoint := &Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
		Config: `{"timeout": "30s"}`,
	}

	err = storage.SaveEndpoint(endpoint)
	require.NoError(t, err)

	retrieved, err := storage.GetEndpoint("test-endpoint")
	require.NoError(t, err)
	assert.Equal(t, endpoint.ID, retrieved.ID)

	// Verify migration was applied by checking schema_version table
	var version int
	err = storage.db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Greater(t, version, 0)
}
