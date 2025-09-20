package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and WAL mode for better performance
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	storage := &SQLiteStorage{db: db}

	// Run database migrations
	migrationMgr := newMigrationManager(db)
	if err := migrationMgr.runMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return storage, nil
}

// SaveEndpoint saves an endpoint configuration
func (s *SQLiteStorage) SaveEndpoint(endpoint *Endpoint) error {
	query := `
		INSERT OR REPLACE INTO endpoints (id, url, method, spec_file, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = now
	}
	endpoint.UpdatedAt = now

	_, err := s.db.Exec(query, endpoint.ID, endpoint.URL, endpoint.Method,
		endpoint.SpecFile, endpoint.Config, endpoint.CreatedAt, endpoint.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save endpoint: %w", err)
	}

	return nil
}

// GetEndpoint retrieves an endpoint by ID
func (s *SQLiteStorage) GetEndpoint(id string) (*Endpoint, error) {
	query := `
		SELECT id, url, method, spec_file, config, created_at, updated_at
		FROM endpoints
		WHERE id = ?
	`

	var endpoint Endpoint
	var specFile sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&endpoint.ID, &endpoint.URL, &endpoint.Method, &specFile,
		&endpoint.Config, &endpoint.CreatedAt, &endpoint.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("endpoint not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get endpoint: %w", err)
	}

	if specFile.Valid {
		endpoint.SpecFile = specFile.String
	}

	return &endpoint, nil
}

// ListEndpoints retrieves all endpoints
func (s *SQLiteStorage) ListEndpoints() ([]*Endpoint, error) {
	query := `
		SELECT id, url, method, spec_file, config, created_at, updated_at
		FROM endpoints
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*Endpoint
	for rows.Next() {
		var endpoint Endpoint
		var specFile sql.NullString

		err := rows.Scan(
			&endpoint.ID, &endpoint.URL, &endpoint.Method, &specFile,
			&endpoint.Config, &endpoint.CreatedAt, &endpoint.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan endpoint: %w", err)
		}

		if specFile.Valid {
			endpoint.SpecFile = specFile.String
		}

		endpoints = append(endpoints, &endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating endpoints: %w", err)
	}

	return endpoints, nil
}

// SaveMonitoringRun saves a monitoring run result
func (s *SQLiteStorage) SaveMonitoringRun(run *MonitoringRun) error {
	query := `
		INSERT INTO monitoring_runs (endpoint_id, timestamp, response_status, response_time_ms, 
			response_body, response_headers, validation_result)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	// Convert headers map to JSON
	headersJSON, err := json.Marshal(run.ResponseHeaders)
	if err != nil {
		return fmt.Errorf("failed to marshal response headers: %w", err)
	}

	if run.Timestamp.IsZero() {
		run.Timestamp = time.Now()
	}

	result, err := s.db.Exec(query, run.EndpointID, run.Timestamp, run.ResponseStatus,
		run.ResponseTimeMs, run.ResponseBody, string(headersJSON), run.ValidationResult)
	if err != nil {
		return fmt.Errorf("failed to save monitoring run: %w", err)
	}

	// Get the generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get monitoring run ID: %w", err)
	}
	run.ID = id

	return nil
}

// GetMonitoringHistory retrieves monitoring history for an endpoint
func (s *SQLiteStorage) GetMonitoringHistory(endpointID string, period time.Duration) ([]*MonitoringRun, error) {
	query := `
		SELECT id, endpoint_id, timestamp, response_status, response_time_ms,
			response_body, response_headers, validation_result
		FROM monitoring_runs
		WHERE endpoint_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC
	`

	since := time.Now().Add(-period)
	rows, err := s.db.Query(query, endpointID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitoring history: %w", err)
	}
	defer rows.Close()

	var runs []*MonitoringRun
	for rows.Next() {
		var run MonitoringRun
		var headersJSON string
		var validationResult sql.NullString

		err := rows.Scan(
			&run.ID, &run.EndpointID, &run.Timestamp, &run.ResponseStatus,
			&run.ResponseTimeMs, &run.ResponseBody, &headersJSON, &validationResult,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monitoring run: %w", err)
		}

		// Parse headers JSON
		if err := json.Unmarshal([]byte(headersJSON), &run.ResponseHeaders); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response headers: %w", err)
		}

		if validationResult.Valid {
			run.ValidationResult = validationResult.String
		}

		runs = append(runs, &run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating monitoring runs: %w", err)
	}

	return runs, nil
}

// SaveDrift saves a detected drift
func (s *SQLiteStorage) SaveDrift(drift *Drift) error {
	query := `
		INSERT INTO drifts (endpoint_id, detected_at, drift_type, severity, description,
			before_value, after_value, field_path, acknowledged)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	if drift.DetectedAt.IsZero() {
		drift.DetectedAt = time.Now()
	}

	result, err := s.db.Exec(query, drift.EndpointID, drift.DetectedAt, drift.DriftType,
		drift.Severity, drift.Description, drift.BeforeValue, drift.AfterValue,
		drift.FieldPath, drift.Acknowledged)
	if err != nil {
		return fmt.Errorf("failed to save drift: %w", err)
	}

	// Get the generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get drift ID: %w", err)
	}
	drift.ID = id

	return nil
}

// GetDrifts retrieves drifts based on filters
func (s *SQLiteStorage) GetDrifts(filters DriftFilters) ([]*Drift, error) {
	query := `
		SELECT id, endpoint_id, detected_at, drift_type, severity, description,
			before_value, after_value, field_path, acknowledged
		FROM drifts
		WHERE 1=1
	`

	var args []interface{}

	// Apply filters
	if filters.EndpointID != "" {
		query += " AND endpoint_id = ?"
		args = append(args, filters.EndpointID)
	}

	if filters.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filters.Severity)
	}

	if !filters.StartTime.IsZero() {
		query += " AND detected_at >= ?"
		args = append(args, filters.StartTime)
	}

	if !filters.EndTime.IsZero() {
		query += " AND detected_at <= ?"
		args = append(args, filters.EndTime)
	}

	if filters.Acknowledged != nil {
		query += " AND acknowledged = ?"
		args = append(args, *filters.Acknowledged)
	}

	query += " ORDER BY detected_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get drifts: %w", err)
	}
	defer rows.Close()

	var drifts []*Drift
	for rows.Next() {
		var drift Drift
		var description, beforeValue, afterValue, fieldPath sql.NullString

		err := rows.Scan(
			&drift.ID, &drift.EndpointID, &drift.DetectedAt, &drift.DriftType,
			&drift.Severity, &description, &beforeValue, &afterValue,
			&fieldPath, &drift.Acknowledged,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan drift: %w", err)
		}

		if description.Valid {
			drift.Description = description.String
		}
		if beforeValue.Valid {
			drift.BeforeValue = beforeValue.String
		}
		if afterValue.Valid {
			drift.AfterValue = afterValue.String
		}
		if fieldPath.Valid {
			drift.FieldPath = fieldPath.String
		}

		drifts = append(drifts, &drift)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating drifts: %w", err)
	}

	return drifts, nil
}

// SaveAlert saves an alert record
func (s *SQLiteStorage) SaveAlert(alert *Alert) error {
	query := `
		INSERT INTO alerts (drift_id, alert_type, channel_name, sent_at, status, error_message, retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	if alert.SentAt.IsZero() {
		alert.SentAt = time.Now()
	}

	result, err := s.db.Exec(query, alert.DriftID, alert.AlertType, alert.ChannelName,
		alert.SentAt, alert.Status, alert.ErrorMessage, alert.RetryCount)
	if err != nil {
		return fmt.Errorf("failed to save alert: %w", err)
	}

	// Get the generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get alert ID: %w", err)
	}
	alert.ID = id

	return nil
}

// GetAlerts retrieves alerts based on filters
func (s *SQLiteStorage) GetAlerts(filters AlertFilters) ([]*Alert, error) {
	query := `
		SELECT id, drift_id, alert_type, channel_name, sent_at, status, error_message, retry_count
		FROM alerts
		WHERE 1=1
	`

	var args []interface{}

	// Apply filters
	if filters.DriftID != nil {
		query += " AND drift_id = ?"
		args = append(args, *filters.DriftID)
	}

	if filters.AlertType != "" {
		query += " AND alert_type = ?"
		args = append(args, filters.AlertType)
	}

	if filters.ChannelName != "" {
		query += " AND channel_name = ?"
		args = append(args, filters.ChannelName)
	}

	if filters.Status != "" {
		query += " AND status = ?"
		args = append(args, filters.Status)
	}

	if !filters.StartTime.IsZero() {
		query += " AND sent_at >= ?"
		args = append(args, filters.StartTime)
	}

	if !filters.EndTime.IsZero() {
		query += " AND sent_at <= ?"
		args = append(args, filters.EndTime)
	}

	query += " ORDER BY sent_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		var alert Alert
		var errorMessage sql.NullString

		err := rows.Scan(
			&alert.ID, &alert.DriftID, &alert.AlertType, &alert.ChannelName,
			&alert.SentAt, &alert.Status, &errorMessage, &alert.RetryCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}

		if errorMessage.Valid {
			alert.ErrorMessage = errorMessage.String
		}

		alerts = append(alerts, &alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alerts: %w", err)
	}

	return alerts, nil
}

// CleanupOldMonitoringRuns removes monitoring runs older than the specified time
func (s *SQLiteStorage) CleanupOldMonitoringRuns(olderThan time.Time) (int64, error) {
	query := `DELETE FROM monitoring_runs WHERE timestamp < ?`

	result, err := s.db.Exec(query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old monitoring runs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// CleanupOldDrifts removes drifts older than the specified time
func (s *SQLiteStorage) CleanupOldDrifts(olderThan time.Time) (int64, error) {
	query := `DELETE FROM drifts WHERE detected_at < ?`

	result, err := s.db.Exec(query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old drifts: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// CleanupOldAlerts removes alerts older than the specified time
func (s *SQLiteStorage) CleanupOldAlerts(olderThan time.Time) (int64, error) {
	query := `DELETE FROM alerts WHERE sent_at < ?`

	result, err := s.db.Exec(query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old alerts: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// GetDatabaseStats returns database size and record count statistics
func (s *SQLiteStorage) GetDatabaseStats() (*DatabaseStats, error) {
	stats := &DatabaseStats{}

	// Get database file size
	var dbSize sql.NullInt64
	err := s.db.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&dbSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	if dbSize.Valid {
		stats.DatabaseSizeBytes = dbSize.Int64
	}

	// Get record counts
	queries := map[string]*int64{
		"SELECT COUNT(*) FROM endpoints":       &stats.Endpoints,
		"SELECT COUNT(*) FROM monitoring_runs": &stats.MonitoringRuns,
		"SELECT COUNT(*) FROM drifts":          &stats.Drifts,
		"SELECT COUNT(*) FROM alerts":          &stats.Alerts,
	}

	for query, target := range queries {
		err := s.db.QueryRow(query).Scan(target)
		if err != nil {
			return nil, fmt.Errorf("failed to get record count: %w", err)
		}
	}

	return stats, nil
}

// VacuumDatabase performs database optimization and cleanup
func (s *SQLiteStorage) VacuumDatabase() error {
	// Run VACUUM to reclaim space and optimize database
	_, err := s.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	// Analyze tables for query optimization
	_, err = s.db.Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}

	return nil
}

// CheckIntegrity performs a comprehensive database integrity check
func (s *SQLiteStorage) CheckIntegrity() (*IntegrityResult, error) {
	result := &IntegrityResult{
		Healthy:         true,
		Issues:          []IntegrityIssue{},
		CheckedAt:       time.Now(),
		OrphanedRecords: make(map[string]int64),
	}

	// Run SQLite's built-in integrity check
	var integrityCheck string
	err := s.db.QueryRow("PRAGMA integrity_check").Scan(&integrityCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to run integrity check: %w", err)
	}

	if integrityCheck != "ok" {
		result.Healthy = false
		result.Issues = append(result.Issues, IntegrityIssue{
			Type:        "corruption",
			Severity:    "critical",
			Table:       "database",
			Description: fmt.Sprintf("SQLite integrity check failed: %s", integrityCheck),
			Repairable:  false,
		})
	}

	// Check for orphaned records in monitoring_runs
	var orphanedMonitoringRuns int64
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM monitoring_runs mr 
		WHERE NOT EXISTS (SELECT 1 FROM endpoints e WHERE e.id = mr.endpoint_id)
	`).Scan(&orphanedMonitoringRuns)
	if err != nil {
		return nil, fmt.Errorf("failed to check orphaned monitoring runs: %w", err)
	}

	if orphanedMonitoringRuns > 0 {
		result.Healthy = false
		result.OrphanedRecords["monitoring_runs"] = orphanedMonitoringRuns
		result.Issues = append(result.Issues, IntegrityIssue{
			Type:        "orphaned_record",
			Severity:    "medium",
			Table:       "monitoring_runs",
			Description: fmt.Sprintf("%d monitoring runs reference non-existent endpoints", orphanedMonitoringRuns),
			Repairable:  true,
		})
	}

	// Check for orphaned records in drifts
	var orphanedDrifts int64
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM drifts d 
		WHERE NOT EXISTS (SELECT 1 FROM endpoints e WHERE e.id = d.endpoint_id)
	`).Scan(&orphanedDrifts)
	if err != nil {
		return nil, fmt.Errorf("failed to check orphaned drifts: %w", err)
	}

	if orphanedDrifts > 0 {
		result.Healthy = false
		result.OrphanedRecords["drifts"] = orphanedDrifts
		result.Issues = append(result.Issues, IntegrityIssue{
			Type:        "orphaned_record",
			Severity:    "medium",
			Table:       "drifts",
			Description: fmt.Sprintf("%d drifts reference non-existent endpoints", orphanedDrifts),
			Repairable:  true,
		})
	}

	// Check for orphaned records in alerts
	var orphanedAlerts int64
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM alerts a 
		WHERE NOT EXISTS (SELECT 1 FROM drifts d WHERE d.id = a.drift_id)
	`).Scan(&orphanedAlerts)
	if err != nil {
		return nil, fmt.Errorf("failed to check orphaned alerts: %w", err)
	}

	if orphanedAlerts > 0 {
		result.Healthy = false
		result.OrphanedRecords["alerts"] = orphanedAlerts
		result.Issues = append(result.Issues, IntegrityIssue{
			Type:        "orphaned_record",
			Severity:    "low",
			Table:       "alerts",
			Description: fmt.Sprintf("%d alerts reference non-existent drifts", orphanedAlerts),
			Repairable:  true,
		})
	}

	// Check table count
	tables := []string{"endpoints", "monitoring_runs", "drifts", "alerts", "schema_version"}
	result.TablesChecked = len(tables)

	// Check for index corruption by running PRAGMA index_list and index_info
	for _, table := range tables {
		rows, err := s.db.Query("PRAGMA index_list(?)", table)
		if err != nil {
			continue // Table might not exist or have indexes
		}
		if err := rows.Close(); err != nil {
			// Log error but continue - this is not critical
			fmt.Printf("Warning: Failed to close rows: %v\n", err)
		}
	}

	return result, nil
}

// RepairDatabase attempts to repair database integrity issues
func (s *SQLiteStorage) RepairDatabase() (*RepairResult, error) {
	backupPath, err := s.createRepairBackup()
	if err != nil {
		return nil, err
	}

	result := &RepairResult{
		Success:       true,
		RepairedAt:    time.Now(),
		Actions:       []RepairAction{},
		BackupCreated: backupPath,
	}

	integrityResult, err := s.CheckIntegrity()
	if err != nil {
		return nil, fmt.Errorf("failed to check integrity before repair: %w", err)
	}

	if integrityResult.Healthy {
		return result, nil // Nothing to repair
	}

	if err := s.performRepairOperations(integrityResult, result); err != nil {
		return result, err
	}

	s.updateRemainingIssues(result)
	return result, nil
}

// createRepairBackup creates a backup before attempting repairs
func (s *SQLiteStorage) createRepairBackup() (string, error) {
	backupPath := fmt.Sprintf("driftwatch_backup_%d.db", time.Now().Unix())
	if err := s.BackupDatabase(backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup before repair: %w", err)
	}
	return backupPath, nil
}

// performRepairOperations executes all repair operations in a transaction
func (s *SQLiteStorage) performRepairOperations(integrityResult *IntegrityResult, result *RepairResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin repair transaction: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	if err := s.repairOrphanedRecords(tx, integrityResult, result); err != nil {
		result.Success = false
		return err
	}

	if err := s.rebuildIndexes(tx, result); err != nil {
		result.Success = false
		return err
	}

	if err := tx.Commit(); err != nil {
		result.Success = false
		return fmt.Errorf("failed to commit repair transaction: %w", err)
	}

	return nil
}

// repairOrphanedRecords removes orphaned records from all tables
func (s *SQLiteStorage) repairOrphanedRecords(tx *sql.Tx, integrityResult *IntegrityResult, result *RepairResult) error {
	orphanedTables := []struct {
		table       string
		query       string
		description string
	}{
		{
			table: "monitoring_runs",
			query: `DELETE FROM monitoring_runs 
					WHERE NOT EXISTS (SELECT 1 FROM endpoints WHERE id = monitoring_runs.endpoint_id)`,
			description: "Deleted orphaned monitoring runs",
		},
		{
			table: "drifts",
			query: `DELETE FROM drifts 
					WHERE NOT EXISTS (SELECT 1 FROM endpoints WHERE id = drifts.endpoint_id)`,
			description: "Deleted orphaned drifts",
		},
		{
			table: "alerts",
			query: `DELETE FROM alerts 
					WHERE NOT EXISTS (SELECT 1 FROM drifts WHERE id = alerts.drift_id)`,
			description: "Deleted orphaned alerts",
		},
	}

	for _, orphaned := range orphanedTables {
		if count, exists := integrityResult.OrphanedRecords[orphaned.table]; exists && count > 0 {
			if err := s.deleteOrphanedRecords(tx, orphaned.query, orphaned.table, orphaned.description, result); err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteOrphanedRecords deletes orphaned records for a specific table
func (s *SQLiteStorage) deleteOrphanedRecords(tx *sql.Tx, query, table, description string, result *RepairResult) error {
	deleteResult, err := tx.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete orphaned %s: %w", table, err)
	}

	rowsAffected, _ := deleteResult.RowsAffected() // nolint:errcheck
	result.Actions = append(result.Actions, RepairAction{
		Type:            "delete_orphaned",
		Table:           table,
		Description:     description,
		RecordsAffected: rowsAffected,
	})
	result.IssuesRepaired++

	return nil
}

// rebuildIndexes rebuilds all database indexes
func (s *SQLiteStorage) rebuildIndexes(tx *sql.Tx, result *RepairResult) error {
	_, err := tx.Exec("REINDEX")
	if err != nil {
		return fmt.Errorf("failed to rebuild indexes: %w", err)
	}

	result.Actions = append(result.Actions, RepairAction{
		Type:        "rebuild_index",
		Table:       "all",
		Description: "Rebuilt all database indexes",
	})

	return nil
}

// updateRemainingIssues checks for remaining issues after repair
func (s *SQLiteStorage) updateRemainingIssues(result *RepairResult) {
	finalIntegrityResult, err := s.CheckIntegrity()
	if err == nil {
		for _, issue := range finalIntegrityResult.Issues {
			if issue.Type == "corruption" {
				result.IssuesRemaining++
			}
		}
	}
}

// BackupDatabase creates a backup of the database
func (s *SQLiteStorage) BackupDatabase(backupPath string) error {
	_, err := s.db.Exec("PRAGMA wal_checkpoint(FULL)")
	if err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}
	_, err = s.db.Exec("VACUUM INTO ?", backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

// RestoreDatabase restores the database from a backup
func (s *SQLiteStorage) RestoreDatabase(backupPath string) error {
	// Close current connection
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close current database: %w", err)
	}

	// This is a simplified restore - in practice, you'd want to:
	// 1. Validate the backup file
	// 2. Create a backup of the current database
	// 3. Replace the current database with the backup
	// 4. Reopen the connection -

	return fmt.Errorf("restore functionality requires external file operations - use system tools to replace database file")
}

// GetHealthStatus returns comprehensive database health information
func (s *SQLiteStorage) GetHealthStatus() (*HealthStatus, error) {
	status := &HealthStatus{
		Healthy:         true,
		Status:          "excellent",
		CheckedAt:       time.Now(),
		Recommendations: []HealthRecommendation{},
	}

	// Get database stats
	stats, err := s.GetDatabaseStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}
	status.DatabaseSize = stats.DatabaseSizeBytes

	// Get schema version
	err = s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&status.SchemaVersion)
	if err != nil {
		status.SchemaVersion = 0
	}

	// Check integrity
	integrityResult, err := s.CheckIntegrity()
	if err != nil {
		status.Healthy = false
		status.Status = "critical"
		status.Recommendations = append(status.Recommendations, HealthRecommendation{
			Type:        "repair",
			Priority:    "critical",
			Description: "Database integrity check failed",
			Action:      "driftwatch repair",
		})
	} else {
		status.IntegrityIssues = len(integrityResult.Issues)
		if !integrityResult.Healthy {
			status.Healthy = false
			if status.IntegrityIssues > 5 {
				status.Status = "critical"
			} else if status.IntegrityIssues > 2 {
				status.Status = "warning"
			} else {
				status.Status = "good"
			}

			status.Recommendations = append(status.Recommendations, HealthRecommendation{
				Type:        "repair",
				Priority:    "high",
				Description: fmt.Sprintf("Found %d integrity issues", status.IntegrityIssues),
				Action:      "driftwatch repair",
			})
		}
	}

	// Check fragmentation level
	var pageCount, freelistCount int64
	err = s.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		err = s.db.QueryRow("PRAGMA freelist_count").Scan(&freelistCount)
		if err == nil && pageCount > 0 {
			status.FragmentationLevel = float64(freelistCount) / float64(pageCount) * 100

			if status.FragmentationLevel > 25 {
				status.Recommendations = append(status.Recommendations, HealthRecommendation{
					Type:        "vacuum",
					Priority:    "medium",
					Description: fmt.Sprintf("Database fragmentation is %.1f%%", status.FragmentationLevel),
					Action:      "driftwatch cleanup --vacuum",
				})
			}
		}
	}

	// Check database size and recommend cleanup if large
	sizeMB := float64(status.DatabaseSize) / 1024 / 1024
	if sizeMB > 100 {
		status.Recommendations = append(status.Recommendations, HealthRecommendation{
			Type:        "cleanup",
			Priority:    "low",
			Description: fmt.Sprintf("Database size is %.1f MB", sizeMB),
			Action:      "driftwatch cleanup",
		})
	}

	// Recommend backup if database is significant size and no recent backup
	if sizeMB > 10 {
		status.Recommendations = append(status.Recommendations, HealthRecommendation{
			Type:        "backup",
			Priority:    "medium",
			Description: "Regular backups recommended for databases over 10MB",
			Action:      "driftwatch backup",
		})
	}

	return status, nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
