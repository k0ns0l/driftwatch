package storage

import (
	"database/sql"
	"fmt"
)

// Migration represents a database migration
type Migration struct {
	SQL         string
	Description string
	Version     int
}

// migrationManager handles database schema migrations
type migrationManager struct {
	db *sql.DB
}

// newMigrationManager creates a new migration manager
func newMigrationManager(db *sql.DB) *migrationManager {
	return &migrationManager{db: db}
}

// getCurrentVersion returns the current schema version
func (m *migrationManager) getCurrentVersion() (int, error) {
	// Create schema_version table if it doesn't exist
	createVersionTable := `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`

	if _, err := m.db.Exec(createVersionTable); err != nil {
		return 0, fmt.Errorf("failed to create schema_version table: %w", err)
	}

	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current schema version: %w", err)
	}

	return version, nil
}

// applyMigration applies a single migration
func (m *migrationManager) applyMigration(migration Migration) error {
	// Ensure schema_version table exists
	if _, err := m.getCurrentVersion(); err != nil {
		return fmt.Errorf("failed to initialize schema version: %w", err)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	// Execute the migration SQL
	if _, err := tx.Exec(migration.SQL); err != nil {
		return fmt.Errorf("failed to execute migration %d (%s): %w",
			migration.Version, migration.Description, err)
	}

	// Record the migration as applied
	if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", migration.Version); err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
	}

	return nil
}

// runMigrations applies all pending migrations
func (m *migrationManager) runMigrations() error {
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	migrations := getMigrations()

	for _, migration := range migrations {
		if migration.Version > currentVersion {
			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration: %w", err)
			}
		}
	}

	return nil
}

// getMigrations returns all available migrations in order
func getMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Initial schema creation",
			SQL: `
				-- Endpoints configuration
				CREATE TABLE IF NOT EXISTS endpoints (
					id TEXT PRIMARY KEY,
					url TEXT NOT NULL,
					method TEXT NOT NULL,
					spec_file TEXT,
					config TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- Index for faster endpoint lookups
				CREATE INDEX IF NOT EXISTS idx_endpoints_url ON endpoints(url);
				CREATE INDEX IF NOT EXISTS idx_endpoints_method ON endpoints(method);

				-- Monitoring execution history
				CREATE TABLE IF NOT EXISTS monitoring_runs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					endpoint_id TEXT NOT NULL,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
					response_status INTEGER,
					response_time_ms INTEGER,
					response_body TEXT,
					response_headers TEXT,
					validation_result TEXT,
					FOREIGN KEY (endpoint_id) REFERENCES endpoints (id) ON DELETE CASCADE
				);

				-- Indexes for monitoring runs
				CREATE INDEX IF NOT EXISTS idx_monitoring_runs_endpoint_id ON monitoring_runs(endpoint_id);
				CREATE INDEX IF NOT EXISTS idx_monitoring_runs_timestamp ON monitoring_runs(timestamp);
				CREATE INDEX IF NOT EXISTS idx_monitoring_runs_endpoint_timestamp ON monitoring_runs(endpoint_id, timestamp);

				-- Detected API drifts
				CREATE TABLE IF NOT EXISTS drifts (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					endpoint_id TEXT NOT NULL,
					detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					drift_type TEXT NOT NULL,
					severity TEXT NOT NULL,
					description TEXT,
					before_value TEXT,
					after_value TEXT,
					field_path TEXT,
					acknowledged BOOLEAN DEFAULT FALSE,
					FOREIGN KEY (endpoint_id) REFERENCES endpoints (id) ON DELETE CASCADE
				);

				-- Indexes for drifts
				CREATE INDEX IF NOT EXISTS idx_drifts_endpoint_id ON drifts(endpoint_id);
				CREATE INDEX IF NOT EXISTS idx_drifts_detected_at ON drifts(detected_at);
				CREATE INDEX IF NOT EXISTS idx_drifts_severity ON drifts(severity);
				CREATE INDEX IF NOT EXISTS idx_drifts_acknowledged ON drifts(acknowledged);
				CREATE INDEX IF NOT EXISTS idx_drifts_endpoint_detected ON drifts(endpoint_id, detected_at);

				-- Alert delivery tracking
				CREATE TABLE IF NOT EXISTS alerts (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					drift_id INTEGER NOT NULL,
					alert_type TEXT NOT NULL,
					channel_name TEXT NOT NULL,
					sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					status TEXT NOT NULL,
					error_message TEXT,
					retry_count INTEGER DEFAULT 0,
					FOREIGN KEY (drift_id) REFERENCES drifts (id) ON DELETE CASCADE
				);

				-- Indexes for alerts
				CREATE INDEX IF NOT EXISTS idx_alerts_drift_id ON alerts(drift_id);
				CREATE INDEX IF NOT EXISTS idx_alerts_sent_at ON alerts(sent_at);
				CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
				CREATE INDEX IF NOT EXISTS idx_alerts_channel_name ON alerts(channel_name);
			`,
		},
		// Future migrations can be added here
		// {
		//     Version:     2,
		//     Description: "Add new column to endpoints table",
		//     SQL: `ALTER TABLE endpoints ADD COLUMN new_field TEXT;`,
		// },
	}
}
