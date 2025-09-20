// Package storage provides data persistence interfaces and implementations
package storage

import (
	"time"
)

// Storage defines the interface for data persistence operations
type Storage interface {
	SaveEndpoint(endpoint *Endpoint) error
	GetEndpoint(id string) (*Endpoint, error)
	ListEndpoints() ([]*Endpoint, error)
	SaveMonitoringRun(run *MonitoringRun) error
	GetMonitoringHistory(endpointID string, period time.Duration) ([]*MonitoringRun, error)
	SaveDrift(drift *Drift) error
	GetDrifts(filters DriftFilters) ([]*Drift, error)
	SaveAlert(alert *Alert) error
	GetAlerts(filters AlertFilters) ([]*Alert, error)

	// Data retention and cleanup methods
	CleanupOldMonitoringRuns(olderThan time.Time) (int64, error)
	CleanupOldDrifts(olderThan time.Time) (int64, error)
	CleanupOldAlerts(olderThan time.Time) (int64, error)
	GetDatabaseStats() (*DatabaseStats, error)
	VacuumDatabase() error

	// Database integrity and recovery methods
	CheckIntegrity() (*IntegrityResult, error)
	RepairDatabase() (*RepairResult, error)
	BackupDatabase(backupPath string) error
	RestoreDatabase(backupPath string) error
	GetHealthStatus() (*HealthStatus, error)

	Close() error
}

// DatabaseStats contains database size and record count information
type DatabaseStats struct {
	DatabaseSizeBytes int64 `json:"database_size_bytes"`
	MonitoringRuns    int64 `json:"monitoring_runs"`
	Drifts            int64 `json:"drifts"`
	Alerts            int64 `json:"alerts"`
	Endpoints         int64 `json:"endpoints"`
}

// IntegrityResult contains the results of a database integrity check
type IntegrityResult struct {
	Issues          []IntegrityIssue `json:"issues"`
	OrphanedRecords map[string]int64 `json:"orphaned_records"`
	CheckedAt       time.Time        `json:"checked_at"`
	TablesChecked   int              `json:"tables_checked"`
	CorruptedPages  int              `json:"corrupted_pages"`
	Healthy         bool             `json:"healthy"`
}

// IntegrityIssue represents a specific database integrity issue
type IntegrityIssue struct {
	Type        string `json:"type"`     // "corruption", "orphaned_record", "constraint_violation", "index_corruption"
	Severity    string `json:"severity"` // "low", "medium", "high", "critical"
	Table       string `json:"table"`
	Description string `json:"description"`
	Repairable  bool   `json:"repairable"`
}

// RepairResult contains the results of a database repair operation
type RepairResult struct {
	Actions         []RepairAction `json:"actions"`
	BackupCreated   string         `json:"backup_created,omitempty"`
	RepairedAt      time.Time      `json:"repaired_at"`
	IssuesRepaired  int            `json:"issues_repaired"`
	IssuesRemaining int            `json:"issues_remaining"`
	Success         bool           `json:"success"`
}

// RepairAction represents an action taken during database repair
type RepairAction struct {
	Type            string `json:"type"` // "delete_orphaned", "rebuild_index", "fix_constraint"
	Table           string `json:"table"`
	Description     string `json:"description"`
	RecordsAffected int64  `json:"records_affected"`
}

// HealthStatus contains overall database health information
type HealthStatus struct {
	Status             string                 `json:"status"` // "excellent", "good", "warning", "critical"
	Recommendations    []HealthRecommendation `json:"recommendations"`
	CheckedAt          time.Time              `json:"checked_at"`
	LastBackup         *time.Time             `json:"last_backup,omitempty"`
	DatabaseSize       int64                  `json:"database_size"`
	FragmentationLevel float64                `json:"fragmentation_level"`
	IntegrityIssues    int                    `json:"integrity_issues"`
	SchemaVersion      int                    `json:"schema_version"`
	Healthy            bool                   `json:"healthy"`
}

// HealthRecommendation represents a recommendation for database health improvement
type HealthRecommendation struct {
	Type        string `json:"type"`     // "backup", "vacuum", "repair", "cleanup"
	Priority    string `json:"priority"` // "low", "medium", "high", "critical"
	Description string `json:"description"`
	Action      string `json:"action"` // Suggested command or action
}

// Endpoint represents an API endpoint configuration
type Endpoint struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	SpecFile  string    `json:"spec_file,omitempty"`
	Config    string    `json:"config"` // JSON-encoded EndpointConfig
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MonitoringRun represents a single monitoring execution
type MonitoringRun struct {
	EndpointID       string            `json:"endpoint_id"`
	ResponseBody     string            `json:"response_body"`
	ValidationResult string            `json:"validation_result"` // JSON-encoded ValidationResult
	ResponseHeaders  map[string]string `json:"response_headers"`
	Timestamp        time.Time         `json:"timestamp"`
	ID               int64             `json:"id"`
	ResponseTimeMs   int64             `json:"response_time_ms"`
	ResponseStatus   int               `json:"response_status"`
}

// Drift represents a detected API drift
type Drift struct {
	EndpointID   string    `json:"endpoint_id"`
	DriftType    string    `json:"drift_type"`
	Severity     string    `json:"severity"`
	Description  string    `json:"description"`
	BeforeValue  string    `json:"before_value"`
	AfterValue   string    `json:"after_value"`
	FieldPath    string    `json:"field_path"`
	DetectedAt   time.Time `json:"detected_at"`
	ID           int64     `json:"id"`
	Acknowledged bool      `json:"acknowledged"`
}

// DriftFilters represents filters for querying drifts
type DriftFilters struct {
	EndpointID   string
	Severity     string
	StartTime    time.Time
	EndTime      time.Time
	Acknowledged *bool
}

// Alert represents a sent alert record
type Alert struct {
	AlertType    string    `json:"alert_type"`
	ChannelName  string    `json:"channel_name"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	SentAt       time.Time `json:"sent_at"`
	ID           int64     `json:"id"`
	DriftID      int64     `json:"drift_id"`
	RetryCount   int       `json:"retry_count"`
}

// AlertFilters represents filters for querying alerts
type AlertFilters struct {
	DriftID     *int64
	AlertType   string
	ChannelName string
	Status      string
	StartTime   time.Time
	EndTime     time.Time
}

// NewStorage creates a new SQLite storage instance
// This is a convenience function that wraps NewSQLiteStorage
func NewStorage(dbPath string) (Storage, error) {
	return NewSQLiteStorage(dbPath)
}
