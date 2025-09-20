package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old monitoring data and optimize database",
	Long: `Clean up old monitoring data based on retention policies and optimize the database.

This command removes old monitoring runs, drifts, and alerts according to the configured
retention policies. It also performs database optimization to reclaim disk space.

Examples:
  driftwatch cleanup                    # Clean up using configured retention policies
  driftwatch cleanup --dry-run         # Show what would be cleaned up without doing it
  driftwatch cleanup --monitoring 7d   # Clean monitoring runs older than 7 days
  driftwatch cleanup --drifts 30d      # Clean drifts older than 30 days
  driftwatch cleanup --alerts 14d      # Clean alerts older than 14 days
  driftwatch cleanup --vacuum          # Only perform database optimization
  driftwatch cleanup --stats           # Show database statistics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		logger := GetLogger()

		// Connect to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Get flags
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "dry-run", err)
		}
		showStats, err := cmd.Flags().GetBool("stats")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "stats", err)
		}
		vacuumOnly, err := cmd.Flags().GetBool("vacuum")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "vacuum", err)
		}
		monitoringAge, err := cmd.Flags().GetDuration("monitoring")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "monitoring", err)
		}
		driftsAge, err := cmd.Flags().GetDuration("drifts")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "drifts", err)
		}
		alertsAge, err := cmd.Flags().GetDuration("alerts")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "alerts", err)
		}

		// Show database statistics if requested
		if showStats {
			return showDatabaseStats(db)
		}

		// If vacuum only, just run vacuum and exit
		if vacuumOnly {
			return performVacuum(db, dryRun)
		}

		// Determine retention periods
		now := time.Now()

		// Use flag values or fall back to config
		monitoringCutoff := now.AddDate(0, 0, -cfg.Retention.MonitoringRunsDays)
		if monitoringAge > 0 {
			monitoringCutoff = now.Add(-monitoringAge)
		}

		driftsCutoff := now.AddDate(0, 0, -cfg.Retention.DriftsDays)
		if driftsAge > 0 {
			driftsCutoff = now.Add(-driftsAge)
		}

		alertsCutoff := now.AddDate(0, 0, -cfg.Retention.AlertsDays)
		if alertsAge > 0 {
			alertsCutoff = now.Add(-alertsAge)
		}

		logger.Info("Starting cleanup process",
			"dry_run", dryRun,
			"monitoring_cutoff", monitoringCutoff.Format(time.RFC3339),
			"drifts_cutoff", driftsCutoff.Format(time.RFC3339),
			"alerts_cutoff", alertsCutoff.Format(time.RFC3339))

		// Show what will be cleaned up
		if dryRun {
			fmt.Println("🔍 Dry run mode - showing what would be cleaned up:")
			fmt.Printf("  • Monitoring runs older than: %s\n", monitoringCutoff.Format("2006-01-02 15:04:05"))
			fmt.Printf("  • Drifts older than: %s\n", driftsCutoff.Format("2006-01-02 15:04:05"))
			fmt.Printf("  • Alerts older than: %s\n", alertsCutoff.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}

		var totalCleaned int64

		// Clean up monitoring runs
		if monitoringAge > 0 || !cmd.Flags().Changed("monitoring") {
			cleaned, err := cleanupMonitoringRuns(db, monitoringCutoff, dryRun)
			if err != nil {
				return fmt.Errorf("failed to cleanup monitoring runs: %w", err)
			}
			totalCleaned += cleaned
		}

		// Clean up drifts
		if driftsAge > 0 || !cmd.Flags().Changed("drifts") {
			cleaned, err := cleanupDrifts(db, driftsCutoff, dryRun)
			if err != nil {
				return fmt.Errorf("failed to cleanup drifts: %w", err)
			}
			totalCleaned += cleaned
		}

		// Clean up alerts
		if alertsAge > 0 || !cmd.Flags().Changed("alerts") {
			cleaned, err := cleanupAlerts(db, alertsCutoff, dryRun)
			if err != nil {
				return fmt.Errorf("failed to cleanup alerts: %w", err)
			}
			totalCleaned += cleaned
		}

		// Perform database optimization if not dry run and records were cleaned
		if !dryRun && totalCleaned > 0 {
			fmt.Println("\n🔧 Optimizing database...")
			if err := performVacuum(db, false); err != nil {
				logger.LogError(context.TODO(), err, "Failed to vacuum database")
				fmt.Printf("⚠️  Database optimization failed: %v\n", err)
			} else {
				fmt.Println("✅ Database optimization completed")
			}
		}

		if dryRun {
			fmt.Printf("\n📊 Total records that would be cleaned: %d\n", totalCleaned)
			fmt.Println("Run without --dry-run to perform actual cleanup")
		} else {
			fmt.Printf("\n✅ Cleanup completed successfully - %d records removed\n", totalCleaned)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	// Add flags
	cleanupCmd.Flags().Bool("dry-run", false, "show what would be cleaned up without actually doing it")
	cleanupCmd.Flags().Bool("stats", false, "show database statistics and exit")
	cleanupCmd.Flags().Bool("vacuum", false, "only perform database optimization (vacuum)")
	cleanupCmd.Flags().Duration("monitoring", 0, "clean monitoring runs older than this duration (e.g., 7d, 24h)")
	cleanupCmd.Flags().Duration("drifts", 0, "clean drifts older than this duration (e.g., 30d, 720h)")
	cleanupCmd.Flags().Duration("alerts", 0, "clean alerts older than this duration (e.g., 14d, 336h)")
}

// Helper functions

func showDatabaseStats(db storage.Storage) error {
	stats, err := db.GetDatabaseStats()
	if err != nil {
		return fmt.Errorf("failed to get database statistics: %w", err)
	}

	fmt.Println("📊 Database Statistics")
	fmt.Println("=====================")
	fmt.Printf("Database size: %.2f MB (%d bytes)\n",
		float64(stats.DatabaseSizeBytes)/1024/1024, stats.DatabaseSizeBytes)
	fmt.Printf("Endpoints: %d\n", stats.Endpoints)
	fmt.Printf("Monitoring runs: %d\n", stats.MonitoringRuns)
	fmt.Printf("Drifts: %d\n", stats.Drifts)
	fmt.Printf("Alerts: %d\n", stats.Alerts)

	return nil
}

func cleanupMonitoringRuns(db storage.Storage, cutoff time.Time, dryRun bool) (int64, error) {
	if dryRun {
		// For dry run, we need to count records that would be deleted
		// Since we don't have a count method, we'll use the cleanup method with a transaction rollback
		// For now, we'll just show the cutoff date
		fmt.Printf("📈 Would clean monitoring runs older than %s\n", cutoff.Format("2006-01-02 15:04:05"))
		return 0, nil
	}

	cleaned, err := db.CleanupOldMonitoringRuns(cutoff)
	if err != nil {
		return 0, err
	}

	if cleaned > 0 {
		fmt.Printf("📈 Cleaned %d monitoring runs\n", cleaned)
	} else {
		fmt.Println("📈 No old monitoring runs to clean")
	}

	return cleaned, nil
}

func cleanupDrifts(db storage.Storage, cutoff time.Time, dryRun bool) (int64, error) {
	if dryRun {
		fmt.Printf("🔄 Would clean drifts older than %s\n", cutoff.Format("2006-01-02 15:04:05"))
		return 0, nil
	}

	cleaned, err := db.CleanupOldDrifts(cutoff)
	if err != nil {
		return 0, err
	}

	if cleaned > 0 {
		fmt.Printf("🔄 Cleaned %d drifts\n", cleaned)
	} else {
		fmt.Println("🔄 No old drifts to clean")
	}

	return cleaned, nil
}

func cleanupAlerts(db storage.Storage, cutoff time.Time, dryRun bool) (int64, error) {
	if dryRun {
		fmt.Printf("🚨 Would clean alerts older than %s\n", cutoff.Format("2006-01-02 15:04:05"))
		return 0, nil
	}

	cleaned, err := db.CleanupOldAlerts(cutoff)
	if err != nil {
		return 0, err
	}

	if cleaned > 0 {
		fmt.Printf("🚨 Cleaned %d alerts\n", cleaned)
	} else {
		fmt.Println("🚨 No old alerts to clean")
	}

	return cleaned, nil
}

func performVacuum(db storage.Storage, dryRun bool) error {
	if dryRun {
		fmt.Println("🔧 Would optimize database (vacuum and analyze)")
		return nil
	}

	fmt.Println("🔧 Optimizing database...")
	if err := db.VacuumDatabase(); err != nil {
		return err
	}

	fmt.Println("✅ Database optimization completed")
	return nil
}
