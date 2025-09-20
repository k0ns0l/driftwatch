package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore <backup-file>",
	Short: "Restore the DriftWatch database from a backup",
	Long: `Restore the DriftWatch database from a previously created backup file.

This command replaces the current database with the contents of the backup file.
A backup of the current database is automatically created before restoration
unless the --no-backup flag is used.

WARNING: This operation will replace all current data. Make sure you have
a backup of your current database before proceeding.

Examples:
  driftwatch restore backup.db                # Restore from backup.db
  driftwatch restore --no-backup backup.db    # Restore without creating current backup
  driftwatch restore --force backup.db        # Skip confirmation prompts`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		logger := GetLogger()
		backupFile := args[0]

		// Check if backup file exists
		if _, err := os.Stat(backupFile); os.IsNotExist(err) {
			return fmt.Errorf("backup file does not exist: %s", backupFile)
		}

		// Get flags
		noBackup, err := cmd.Flags().GetBool("no-backup")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "no-backup", err)
		}
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "force", err)
		}

		// Confirmation prompt unless --force is used
		if !force {
			fmt.Printf("⚠️  WARNING: This will replace your current database with the backup.\n")
			fmt.Printf("📁 Backup file: %s\n", backupFile)
			fmt.Printf("🗄️  Current database: %s\n", cfg.Global.DatabaseURL)

			if !noBackup {
				fmt.Printf("💾 A backup of your current database will be created first.\n")
			}

			fmt.Print("\nDo you want to continue? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
			}

			if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
				fmt.Println("Restore cancelled.")
				return nil
			}
		}

		// Connect to current database to create backup if needed
		if !noBackup {
			db, err := storage.NewStorage(cfg.Global.DatabaseURL)
			if err != nil {
				return fmt.Errorf("failed to connect to current database: %w", err)
			}

			// Create backup of current database
			timestamp := time.Now().Format("20060102_150405")
			currentBackupPath := fmt.Sprintf("driftwatch_pre_restore_backup_%s.db", timestamp)

			fmt.Printf("💾 Creating backup of current database: %s\n", currentBackupPath)
			if err := db.BackupDatabase(currentBackupPath); err != nil {
				if closeErr := db.Close(); closeErr != nil {
					logger.LogError(context.TODO(), closeErr, "Failed to close database connection")
				}
				return fmt.Errorf("failed to backup current database: %w", err)
			}

			fmt.Printf("✅ Current database backed up to: %s\n", currentBackupPath)
			if closeErr := db.Close(); closeErr != nil {
				logger.LogError(context.TODO(), closeErr, "Failed to close database connection")
			}
		}

		// Perform the restore
		fmt.Printf("🔄 Restoring database from: %s\n", backupFile)

		// Get the absolute paths
		currentDBPath, err := filepath.Abs(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for current database: %w", err)
		}

		backupPath, err := filepath.Abs(backupFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for backup file: %w", err)
		}

		// Copy backup file to current database location
		// First, remove the current database file
		if err := os.Remove(currentDBPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove current database: %w", err)
		}

		// Copy backup to current database location
		backupData, err := os.ReadFile(filepath.Clean(backupPath))
		if err != nil {
			return fmt.Errorf("failed to read backup file: %w", err)
		}

		if err := os.WriteFile(currentDBPath, backupData, 0o600); err != nil {
			return fmt.Errorf("failed to write restored database: %w", err)
		}

		// Verify the restored database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to restored database: %w", err)
		}
		defer db.Close()

		// Get stats of restored database
		stats, err := db.GetDatabaseStats()
		if err != nil {
			logger.LogError(context.TODO(), err, "Failed to get restored database stats")
		} else {
			sizeMB := float64(stats.DatabaseSizeBytes) / 1024 / 1024
			fmt.Printf("📊 Restored database size: %.2f MB\n", sizeMB)
			fmt.Printf("📈 Restored records: %d endpoints, %d monitoring runs, %d drifts, %d alerts\n",
				stats.Endpoints, stats.MonitoringRuns, stats.Drifts, stats.Alerts)
		}

		// Run integrity check on restored database
		fmt.Println("🔍 Running integrity check on restored database...")
		integrityResult, err := db.CheckIntegrity()
		if err != nil {
			logger.LogError(context.TODO(), err, "Failed to check integrity of restored database")
			fmt.Printf("⚠️  Warning: Could not verify integrity of restored database: %v\n", err)
		} else if integrityResult.Healthy {
			fmt.Println("✅ Restored database passed integrity check")
		} else {
			fmt.Printf("⚠️  Warning: Restored database has %d integrity issues\n", len(integrityResult.Issues))
			fmt.Println("💡 Consider running 'driftwatch repair' to fix issues")
		}

		fmt.Println("✅ Database restore completed successfully")

		logger.Info("Database restore completed",
			"backup_file", backupFile,
			"database_path", currentDBPath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().Bool("no-backup", false, "skip creating backup of current database")
	restoreCmd.Flags().Bool("force", false, "skip confirmation prompts")
}
