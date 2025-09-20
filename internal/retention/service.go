// Package retention provides data retention and cleanup services
package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/robfig/cron/v3"
)

// Service manages data retention and automatic cleanup
type Service struct {
	storage storage.Storage
	config  *config.RetentionConfig
	logger  *logging.Logger
	cron    *cron.Cron
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewService creates a new retention service
func NewService(storage storage.Storage, config *config.RetentionConfig, logger *logging.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		storage: storage,
		config:  config,
		logger:  logger,
		cron:    cron.New(),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins the automatic cleanup process if enabled
func (s *Service) Start() error {
	if !s.config.AutoCleanup {
		s.logger.Info("Automatic cleanup is disabled")
		return nil
	}

	// Convert cleanup interval to cron expression
	cronExpr := s.intervalToCron(s.config.CleanupInterval)

	// Schedule cleanup job
	_, err := s.cron.AddFunc(cronExpr, func() {
		if err := s.performCleanup(); err != nil {
			s.logger.LogError(context.TODO(), err, "Automatic cleanup failed")
		}
	})
	if err != nil {
		return fmt.Errorf("failed to schedule cleanup job: %w", err)
	}

	s.cron.Start()
	s.logger.Info("Automatic cleanup service started",
		"interval", s.config.CleanupInterval,
		"cron_expression", cronExpr)

	return nil
}

// Stop stops the automatic cleanup process
func (s *Service) Stop() {
	s.cancel()
	if s.cron != nil {
		s.cron.Stop()
	}
	s.logger.Info("Retention service stopped")
}

// PerformCleanup manually triggers a cleanup operation
func (s *Service) PerformCleanup() error {
	return s.performCleanup()
}

// GetStats returns current database statistics
func (s *Service) GetStats() (*storage.DatabaseStats, error) {
	return s.storage.GetDatabaseStats()
}

// performCleanup executes the cleanup process
func (s *Service) performCleanup() error {
	s.logger.Info("Starting automatic cleanup")

	now := time.Now()
	var totalCleaned int64

	// Calculate cutoff times
	monitoringCutoff := now.AddDate(0, 0, -s.config.MonitoringRunsDays)
	driftsCutoff := now.AddDate(0, 0, -s.config.DriftsDays)
	alertsCutoff := now.AddDate(0, 0, -s.config.AlertsDays)

	// Clean monitoring runs
	if s.config.MonitoringRunsDays > 0 {
		cleaned, err := s.storage.CleanupOldMonitoringRuns(monitoringCutoff)
		if err != nil {
			return fmt.Errorf("failed to cleanup monitoring runs: %w", err)
		}
		totalCleaned += cleaned
		if cleaned > 0 {
			s.logger.Info("Cleaned old monitoring runs", "count", cleaned, "cutoff", monitoringCutoff)
		}
	}

	// Clean drifts
	if s.config.DriftsDays > 0 {
		cleaned, err := s.storage.CleanupOldDrifts(driftsCutoff)
		if err != nil {
			return fmt.Errorf("failed to cleanup drifts: %w", err)
		}
		totalCleaned += cleaned
		if cleaned > 0 {
			s.logger.Info("Cleaned old drifts", "count", cleaned, "cutoff", driftsCutoff)
		}
	}

	// Clean alerts
	if s.config.AlertsDays > 0 {
		cleaned, err := s.storage.CleanupOldAlerts(alertsCutoff)
		if err != nil {
			return fmt.Errorf("failed to cleanup alerts: %w", err)
		}
		totalCleaned += cleaned
		if cleaned > 0 {
			s.logger.Info("Cleaned old alerts", "count", cleaned, "cutoff", alertsCutoff)
		}
	}

	// Vacuum database if records were cleaned
	if totalCleaned > 0 {
		if err := s.storage.VacuumDatabase(); err != nil {
			s.logger.LogError(context.TODO(), err, "Failed to vacuum database after cleanup")
			// Don't return error as cleanup was successful
		} else {
			s.logger.Info("Database vacuum completed")
		}
	}

	s.logger.Info("Automatic cleanup completed", "total_cleaned", totalCleaned)
	return nil
}

// intervalToCron converts a time.Duration to a cron expression
func (s *Service) intervalToCron(interval time.Duration) string {
	// For simplicity, we'll use fixed schedules based on common intervals
	switch {
	case interval <= time.Hour:
		// Every hour
		return "0 * * * *"
	case interval <= 6*time.Hour:
		// Every 6 hours
		return "0 */6 * * *"
	case interval <= 12*time.Hour:
		// Every 12 hours
		return "0 */12 * * *"
	case interval <= 24*time.Hour:
		// Daily at 2 AM
		return "0 2 * * *"
	case interval <= 7*24*time.Hour:
		// Weekly on Sunday at 2 AM
		return "0 2 * * 0"
	default:
		// Monthly on the 1st at 2 AM
		return "0 2 1 * *"
	}
}

// CleanupOptions provides options for manual cleanup operations
type CleanupOptions struct {
	MonitoringRunsOlderThan *time.Time
	DriftsOlderThan         *time.Time
	AlertsOlderThan         *time.Time
	VacuumAfter             bool
	DryRun                  bool
}

// CleanupWithOptions performs cleanup with custom options
func (s *Service) CleanupWithOptions(opts CleanupOptions) (*CleanupResult, error) {
	result := &CleanupResult{}

	if opts.DryRun {
		s.logger.Info("Performing dry run cleanup")
	}

	// Clean monitoring runs
	if opts.MonitoringRunsOlderThan != nil {
		if opts.DryRun {
			result.MonitoringRunsWouldClean = true
		} else {
			cleaned, err := s.storage.CleanupOldMonitoringRuns(*opts.MonitoringRunsOlderThan)
			if err != nil {
				return nil, fmt.Errorf("failed to cleanup monitoring runs: %w", err)
			}
			result.MonitoringRunsCleaned = cleaned
		}
	}

	// Clean drifts
	if opts.DriftsOlderThan != nil {
		if opts.DryRun {
			result.DriftsWouldClean = true
		} else {
			cleaned, err := s.storage.CleanupOldDrifts(*opts.DriftsOlderThan)
			if err != nil {
				return nil, fmt.Errorf("failed to cleanup drifts: %w", err)
			}
			result.DriftsCleaned = cleaned
		}
	}

	// Clean alerts
	if opts.AlertsOlderThan != nil {
		if opts.DryRun {
			result.AlertsWouldClean = true
		} else {
			cleaned, err := s.storage.CleanupOldAlerts(*opts.AlertsOlderThan)
			if err != nil {
				return nil, fmt.Errorf("failed to cleanup alerts: %w", err)
			}
			result.AlertsCleaned = cleaned
		}
	}

	// Vacuum database if requested and not dry run
	if opts.VacuumAfter && !opts.DryRun && result.TotalCleaned() > 0 {
		if err := s.storage.VacuumDatabase(); err != nil {
			return nil, fmt.Errorf("failed to vacuum database: %w", err)
		}
		result.DatabaseVacuumed = true
	}

	return result, nil
}

// CleanupResult contains the results of a cleanup operation
type CleanupResult struct {
	MonitoringRunsCleaned    int64 `json:"monitoring_runs_cleaned"`
	DriftsCleaned            int64 `json:"drifts_cleaned"`
	AlertsCleaned            int64 `json:"alerts_cleaned"`
	DatabaseVacuumed         bool  `json:"database_vacuumed"`
	MonitoringRunsWouldClean bool  `json:"monitoring_runs_would_clean,omitempty"`
	DriftsWouldClean         bool  `json:"drifts_would_clean,omitempty"`
	AlertsWouldClean         bool  `json:"alerts_would_clean,omitempty"`
}

// TotalCleaned returns the total number of records cleaned
func (r *CleanupResult) TotalCleaned() int64 {
	return r.MonitoringRunsCleaned + r.DriftsCleaned + r.AlertsCleaned
}
