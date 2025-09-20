// Package storage provides data persistence interfaces and implementations
package storage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// InMemoryStorage implements the Storage interface using in-memory data structures
// This is useful for CI/CD scenarios where persistent storage is not needed
type InMemoryStorage struct {
	endpoints      map[string]*Endpoint
	monitoringRuns map[string][]*MonitoringRun // keyed by endpoint ID
	drifts         []*Drift
	alerts         []*Alert
	nextDriftID    int64
	nextAlertID    int64
	nextRunID      int64
	mu             sync.RWMutex
}

// NewInMemoryStorage creates a new in-memory storage instance
func NewInMemoryStorage() (Storage, error) {
	return &InMemoryStorage{
		endpoints:      make(map[string]*Endpoint),
		monitoringRuns: make(map[string][]*MonitoringRun),
		drifts:         make([]*Drift, 0),
		alerts:         make([]*Alert, 0),
		nextDriftID:    1,
		nextAlertID:    1,
		nextRunID:      1,
	}, nil
}

// SaveEndpoint saves an endpoint to memory
func (m *InMemoryStorage) SaveEndpoint(endpoint *Endpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}

	if endpoint.ID == "" {
		return fmt.Errorf("endpoint ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy to avoid external modifications
	endpointCopy := *endpoint
	now := time.Now()

	if existing, exists := m.endpoints[endpoint.ID]; exists {
		endpointCopy.CreatedAt = existing.CreatedAt
		endpointCopy.UpdatedAt = now
	} else {
		endpointCopy.CreatedAt = now
		endpointCopy.UpdatedAt = now
	}

	m.endpoints[endpoint.ID] = &endpointCopy
	return nil
}

// GetEndpoint retrieves an endpoint by ID
func (m *InMemoryStorage) GetEndpoint(id string) (*Endpoint, error) {
	if id == "" {
		return nil, fmt.Errorf("endpoint ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoint, exists := m.endpoints[id]
	if !exists {
		return nil, fmt.Errorf("endpoint not found: %s", id)
	}

	// Return a copy to prevent external modifications
	endpointCopy := *endpoint
	return &endpointCopy, nil
}

// ListEndpoints returns all endpoints
func (m *InMemoryStorage) ListEndpoints() ([]*Endpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoints := make([]*Endpoint, 0, len(m.endpoints))
	for _, endpoint := range m.endpoints {
		// Create copies to prevent external modifications
		endpointCopy := *endpoint
		endpoints = append(endpoints, &endpointCopy)
	}

	// Sort by ID for consistent ordering
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ID < endpoints[j].ID
	})

	return endpoints, nil
}

// SaveMonitoringRun saves a monitoring run to memory
func (m *InMemoryStorage) SaveMonitoringRun(run *MonitoringRun) error {
	if run == nil {
		return fmt.Errorf("monitoring run cannot be nil")
	}

	if run.EndpointID == "" {
		return fmt.Errorf("endpoint ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy and assign ID
	runCopy := *run
	runCopy.ID = m.nextRunID
	m.nextRunID++

	// Add to the endpoint's runs
	m.monitoringRuns[run.EndpointID] = append(m.monitoringRuns[run.EndpointID], &runCopy)

	// Sort runs by timestamp (most recent first)
	runs := m.monitoringRuns[run.EndpointID]
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Timestamp.After(runs[j].Timestamp)
	})

	return nil
}

// GetMonitoringHistory retrieves monitoring history for an endpoint
func (m *InMemoryStorage) GetMonitoringHistory(endpointID string, period time.Duration) ([]*MonitoringRun, error) {
	if endpointID == "" {
		return nil, fmt.Errorf("endpoint ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	runs, exists := m.monitoringRuns[endpointID]
	if !exists {
		return []*MonitoringRun{}, nil
	}

	// Filter by time period
	cutoff := time.Now().Add(-period)
	var filteredRuns []*MonitoringRun

	for _, run := range runs {
		if run.Timestamp.After(cutoff) {
			// Create a copy to prevent external modifications
			runCopy := *run
			filteredRuns = append(filteredRuns, &runCopy)
		}
	}

	return filteredRuns, nil
}

// SaveDrift saves a drift to memory
func (m *InMemoryStorage) SaveDrift(drift *Drift) error {
	if drift == nil {
		return fmt.Errorf("drift cannot be nil")
	}

	if drift.EndpointID == "" {
		return fmt.Errorf("endpoint ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy and assign ID
	driftCopy := *drift
	driftCopy.ID = m.nextDriftID
	m.nextDriftID++

	if driftCopy.DetectedAt.IsZero() {
		driftCopy.DetectedAt = time.Now()
	}

	m.drifts = append(m.drifts, &driftCopy)

	// Sort drifts by detection time (most recent first)
	sort.Slice(m.drifts, func(i, j int) bool {
		return m.drifts[i].DetectedAt.After(m.drifts[j].DetectedAt)
	})

	return nil
}

// GetDrifts retrieves drifts based on filters
func (m *InMemoryStorage) GetDrifts(filters DriftFilters) ([]*Drift, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filteredDrifts []*Drift

	for _, drift := range m.drifts {
		// Apply filters
		if filters.EndpointID != "" && drift.EndpointID != filters.EndpointID {
			continue
		}

		if filters.Severity != "" && drift.Severity != filters.Severity {
			continue
		}

		if !filters.StartTime.IsZero() && drift.DetectedAt.Before(filters.StartTime) {
			continue
		}

		if !filters.EndTime.IsZero() && drift.DetectedAt.After(filters.EndTime) {
			continue
		}

		if filters.Acknowledged != nil && drift.Acknowledged != *filters.Acknowledged {
			continue
		}

		// Create a copy to prevent external modifications
		driftCopy := *drift
		filteredDrifts = append(filteredDrifts, &driftCopy)
	}

	return filteredDrifts, nil
}

// SaveAlert saves an alert to memory
func (m *InMemoryStorage) SaveAlert(alert *Alert) error {
	if alert == nil {
		return fmt.Errorf("alert cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy and assign ID
	alertCopy := *alert
	alertCopy.ID = m.nextAlertID
	m.nextAlertID++

	if alertCopy.SentAt.IsZero() {
		alertCopy.SentAt = time.Now()
	}

	m.alerts = append(m.alerts, &alertCopy)

	// Sort alerts by sent time (most recent first)
	sort.Slice(m.alerts, func(i, j int) bool {
		return m.alerts[i].SentAt.After(m.alerts[j].SentAt)
	})

	return nil
}

// GetAlerts retrieves alerts based on filters
func (m *InMemoryStorage) GetAlerts(filters AlertFilters) ([]*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filteredAlerts []*Alert

	for _, alert := range m.alerts {
		// Apply filters
		if filters.DriftID != nil && alert.DriftID != *filters.DriftID {
			continue
		}

		if filters.AlertType != "" && alert.AlertType != filters.AlertType {
			continue
		}

		if filters.ChannelName != "" && alert.ChannelName != filters.ChannelName {
			continue
		}

		if filters.Status != "" && alert.Status != filters.Status {
			continue
		}

		if !filters.StartTime.IsZero() && alert.SentAt.Before(filters.StartTime) {
			continue
		}

		if !filters.EndTime.IsZero() && alert.SentAt.After(filters.EndTime) {
			continue
		}

		// Create a copy to prevent external modifications
		alertCopy := *alert
		filteredAlerts = append(filteredAlerts, &alertCopy)
	}

	return filteredAlerts, nil
}

// CleanupOldMonitoringRuns removes monitoring runs older than the specified time
func (m *InMemoryStorage) CleanupOldMonitoringRuns(olderThan time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var totalCleaned int64

	for endpointID, runs := range m.monitoringRuns {
		var filteredRuns []*MonitoringRun
		for _, run := range runs {
			if run.Timestamp.After(olderThan) {
				filteredRuns = append(filteredRuns, run)
			} else {
				totalCleaned++
			}
		}
		m.monitoringRuns[endpointID] = filteredRuns
	}

	return totalCleaned, nil
}

// CleanupOldDrifts removes drifts older than the specified time
func (m *InMemoryStorage) CleanupOldDrifts(olderThan time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var filteredDrifts []*Drift
	var cleaned int64

	for _, drift := range m.drifts {
		if drift.DetectedAt.After(olderThan) {
			filteredDrifts = append(filteredDrifts, drift)
		} else {
			cleaned++
		}
	}

	m.drifts = filteredDrifts
	return cleaned, nil
}

// CleanupOldAlerts removes alerts older than the specified time
func (m *InMemoryStorage) CleanupOldAlerts(olderThan time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var filteredAlerts []*Alert
	var cleaned int64

	for _, alert := range m.alerts {
		if alert.SentAt.After(olderThan) {
			filteredAlerts = append(filteredAlerts, alert)
		} else {
			cleaned++
		}
	}

	m.alerts = filteredAlerts
	return cleaned, nil
}

// GetDatabaseStats returns database statistics for in-memory storage
func (m *InMemoryStorage) GetDatabaseStats() (*DatabaseStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRuns := 0
	for _, runs := range m.monitoringRuns {
		totalRuns += len(runs)
	}

	return &DatabaseStats{
		DatabaseSizeBytes: 0, // Not applicable for in-memory storage
		Endpoints:         int64(len(m.endpoints)),
		MonitoringRuns:    int64(totalRuns),
		Drifts:            int64(len(m.drifts)),
		Alerts:            int64(len(m.alerts)),
	}, nil
}

// VacuumDatabase is a no-op for in-memory storage
func (m *InMemoryStorage) VacuumDatabase() error {
	// No-op for in-memory storage
	return nil
}

// CheckIntegrity performs integrity checks on in-memory data
func (m *InMemoryStorage) CheckIntegrity() (*IntegrityResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &IntegrityResult{
		Healthy:         true,
		Issues:          []IntegrityIssue{},
		CheckedAt:       time.Now(),
		TablesChecked:   4, // endpoints, monitoring_runs, drifts, alerts
		OrphanedRecords: make(map[string]int64),
	}

	// Check for orphaned monitoring runs
	var orphanedRuns int64
	for endpointID, runs := range m.monitoringRuns {
		if _, exists := m.endpoints[endpointID]; !exists {
			orphanedRuns += int64(len(runs))
		}
	}

	if orphanedRuns > 0 {
		result.Healthy = false
		result.OrphanedRecords["monitoring_runs"] = orphanedRuns
		result.Issues = append(result.Issues, IntegrityIssue{
			Type:        "orphaned_record",
			Severity:    "medium",
			Table:       "monitoring_runs",
			Description: fmt.Sprintf("%d monitoring runs reference non-existent endpoints", orphanedRuns),
			Repairable:  true,
		})
	}

	// Check for orphaned drifts
	var orphanedDrifts int64
	for _, drift := range m.drifts {
		if _, exists := m.endpoints[drift.EndpointID]; !exists {
			orphanedDrifts++
		}
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

	// Check for orphaned alerts
	driftIDs := make(map[int64]bool)
	for _, drift := range m.drifts {
		driftIDs[drift.ID] = true
	}

	var orphanedAlerts int64
	for _, alert := range m.alerts {
		if !driftIDs[alert.DriftID] {
			orphanedAlerts++
		}
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

	return result, nil
}

// RepairDatabase repairs integrity issues in in-memory data
func (m *InMemoryStorage) RepairDatabase() (*RepairResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &RepairResult{
		Success:    true,
		RepairedAt: time.Now(),
		Actions:    []RepairAction{},
	}

	// Remove orphaned monitoring runs
	var cleanedRuns int64
	for endpointID, runs := range m.monitoringRuns {
		if _, exists := m.endpoints[endpointID]; !exists {
			cleanedRuns += int64(len(runs))
			delete(m.monitoringRuns, endpointID)
		}
	}

	if cleanedRuns > 0 {
		result.Actions = append(result.Actions, RepairAction{
			Type:            "delete_orphaned",
			Table:           "monitoring_runs",
			Description:     "Deleted orphaned monitoring runs",
			RecordsAffected: cleanedRuns,
		})
		result.IssuesRepaired++
	}

	// Remove orphaned drifts
	var filteredDrifts []*Drift
	var cleanedDrifts int64
	for _, drift := range m.drifts {
		if _, exists := m.endpoints[drift.EndpointID]; exists {
			filteredDrifts = append(filteredDrifts, drift)
		} else {
			cleanedDrifts++
		}
	}
	m.drifts = filteredDrifts

	if cleanedDrifts > 0 {
		result.Actions = append(result.Actions, RepairAction{
			Type:            "delete_orphaned",
			Table:           "drifts",
			Description:     "Deleted orphaned drifts",
			RecordsAffected: cleanedDrifts,
		})
		result.IssuesRepaired++
	}

	// Remove orphaned alerts
	driftIDs := make(map[int64]bool)
	for _, drift := range m.drifts {
		driftIDs[drift.ID] = true
	}

	var filteredAlerts []*Alert
	var cleanedAlerts int64
	for _, alert := range m.alerts {
		if driftIDs[alert.DriftID] {
			filteredAlerts = append(filteredAlerts, alert)
		} else {
			cleanedAlerts++
		}
	}
	m.alerts = filteredAlerts

	if cleanedAlerts > 0 {
		result.Actions = append(result.Actions, RepairAction{
			Type:            "delete_orphaned",
			Table:           "alerts",
			Description:     "Deleted orphaned alerts",
			RecordsAffected: cleanedAlerts,
		})
		result.IssuesRepaired++
	}

	return result, nil
}

// BackupDatabase creates a backup (not applicable for in-memory storage)
func (m *InMemoryStorage) BackupDatabase(backupPath string) error {
	return fmt.Errorf("backup not supported for in-memory storage")
}

// RestoreDatabase restores from backup (not applicable for in-memory storage)
func (m *InMemoryStorage) RestoreDatabase(backupPath string) error {
	return fmt.Errorf("restore not supported for in-memory storage")
}

// GetHealthStatus returns health status for in-memory storage
func (m *InMemoryStorage) GetHealthStatus() (*HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &HealthStatus{
		Healthy:            true,
		Status:             "excellent",
		CheckedAt:          time.Now(),
		DatabaseSize:       0, // Not applicable for in-memory
		FragmentationLevel: 0, // Not applicable for in-memory
		SchemaVersion:      1, // Fixed for in-memory
		Recommendations:    []HealthRecommendation{},
	}

	// Check integrity
	integrityResult, err := m.CheckIntegrity()
	if err != nil {
		status.Healthy = false
		status.Status = "critical"
		status.Recommendations = append(status.Recommendations, HealthRecommendation{
			Type:        "repair",
			Priority:    "critical",
			Description: "Integrity check failed",
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

	// Add recommendation about persistence
	status.Recommendations = append(status.Recommendations, HealthRecommendation{
		Type:        "backup",
		Priority:    "low",
		Description: "In-memory storage does not persist data between restarts",
		Action:      "Consider using SQLite storage for persistence",
	})

	return status, nil
}

// Close closes the in-memory storage (no-op for memory storage)
func (m *InMemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all data
	m.endpoints = make(map[string]*Endpoint)
	m.monitoringRuns = make(map[string][]*MonitoringRun)
	m.drifts = make([]*Drift, 0)
	m.alerts = make([]*Alert, 0)
	m.nextDriftID = 1
	m.nextAlertID = 1
	m.nextRunID = 1

	return nil
}
