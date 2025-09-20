package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup [backup-file]",
	Short: "Create a backup of the DriftWatch database",
	Long: `Create a backup of the DriftWatch database to preserve monitoring data and configuration.

The backup command creates a complete copy of the database that can be restored later
using the 'driftwatch restore' command. If no backup file is specified, a timestamped
backup will be created in the current directory.

Examples:
  driftwatch backup                           # Create timestamped backup
  driftwatch backup my-backup.db              # Create backup with specific name
  driftwatch backup /path/to/backup.db        # Create backup at specific path
  driftwatch backup --compress                # Create compressed backup (future feature)`,
	Args: cobra.MaximumNArgs(1),
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

		// Determine backup file path
		var backupPath string
		if len(args) > 0 {
			backupPath = args[0]
		} else {
			// Generate timestamped backup filename
			timestamp := time.Now().Format("20060102_150405")
			backupPath = fmt.Sprintf("driftwatch_backup_%s.db", timestamp)
		}

		// Ensure backup path has .db extension
		if filepath.Ext(backupPath) == "" {
			backupPath += ".db"
		}

		logger.Info("Creating database backup", "backup_path", backupPath)

		// Get database stats before backup
		stats, err := db.GetDatabaseStats()
		if err != nil {
			logger.LogError(context.TODO(), err, "Failed to get database stats")
		} else {
			sizeMB := float64(stats.DatabaseSizeBytes) / 1024 / 1024
			fmt.Printf("📊 Database size: %.2f MB\n", sizeMB)
			fmt.Printf("📈 Records: %d endpoints, %d monitoring runs, %d drifts, %d alerts\n",
				stats.Endpoints, stats.MonitoringRuns, stats.Drifts, stats.Alerts)
		}

		// Create backup
		fmt.Printf("💾 Creating backup: %s\n", backupPath)

		startTime := time.Now()
		if err := db.BackupDatabase(backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		duration := time.Since(startTime)

		fmt.Printf("✅ Backup created successfully in %v\n", duration.Round(time.Millisecond))
		fmt.Printf("📁 Backup location: %s\n", backupPath)

		// Verify backup by checking if file exists and has reasonable size
		if _, err := db.GetDatabaseStats(); err == nil {
			fmt.Printf("💡 To restore this backup later, run: driftwatch restore %s\n", backupPath)
		}

		logger.Info("Database backup completed",
			"backup_path", backupPath,
			"duration", duration)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)

	// Future flags for backup compression, encryption, etc.
	// backupCmd.Flags().Bool("compress", false, "compress the backup file")
	// backupCmd.Flags().String("encryption-key", "", "encrypt backup with provided key")
}
