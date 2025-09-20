package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// repairCmd represents the repair command
var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair database integrity issues",
	Long: `Repair database integrity issues such as orphaned records, corrupted indexes,
and constraint violations. This command automatically creates a backup before
attempting any repairs.

The repair process includes:
- Removing orphaned records that reference non-existent parent records
- Rebuilding corrupted database indexes
- Fixing constraint violations where possible
- Optimizing database structure

A backup is automatically created before repairs unless --no-backup is specified.

Examples:
  driftwatch repair                    # Repair all detected issues
  driftwatch repair --dry-run          # Show what would be repaired without doing it
  driftwatch repair --no-backup        # Skip creating backup before repair
  driftwatch repair --force            # Skip confirmation prompts`,
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
		noBackup, err := cmd.Flags().GetBool("no-backup")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "no-backup", err)
		}
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "force", err)
		}
		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "json", err)
		}

		// First, check what issues exist
		fmt.Println("🔍 Checking database integrity...")
		integrityResult, err := db.CheckIntegrity()
		if err != nil {
			return fmt.Errorf("failed to check database integrity: %w", err)
		}

		if integrityResult.Healthy {
			fmt.Println("✅ Database is healthy - no repairs needed")
			return nil
		}

		// Show what issues were found
		fmt.Printf("⚠️  Found %d integrity issues:\n", len(integrityResult.Issues))
		repairableCount := 0
		for i, issue := range integrityResult.Issues {
			repairableEmoji := "❌"
			if issue.Repairable {
				repairableEmoji = "🔧"
				repairableCount++
			}
			fmt.Printf("  %d. %s [%s] %s - %s\n", i+1, repairableEmoji,
				strings.ToUpper(issue.Severity), issue.Table, issue.Description)
		}

		if repairableCount == 0 {
			fmt.Println("❌ No repairable issues found")
			return nil
		}

		fmt.Printf("\n🔧 %d issues can be repaired automatically\n", repairableCount)

		// Show orphaned records summary
		if len(integrityResult.OrphanedRecords) > 0 {
			fmt.Println("\n🔗 Orphaned records to be cleaned:")
			for table, count := range integrityResult.OrphanedRecords {
				fmt.Printf("  • %s: %d records\n", table, count)
			}
		}

		if dryRun {
			fmt.Println("\n🔍 Dry run mode - no changes will be made")
			if jsonOutput {
				jsonData, err := json.MarshalIndent(integrityResult, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal integrity result to JSON: %w", err)
				}
				fmt.Println(string(jsonData))
			}
			return nil
		}

		// Confirmation prompt unless --force is used
		if !force {
			fmt.Printf("\n⚠️  WARNING: This will modify your database to fix integrity issues.\n")
			if !noBackup {
				fmt.Printf("💾 A backup will be created automatically before repairs.\n")
			}
			fmt.Print("\nDo you want to continue with the repair? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
			}

			if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
				fmt.Println("Repair cancelled.")
				return nil
			}
		}

		// Perform the repair
		fmt.Println("\n🔧 Starting database repair...")

		repairResult, err := db.RepairDatabase()
		if err != nil {
			return fmt.Errorf("failed to repair database: %w", err)
		}

		if jsonOutput {
			jsonData, err := json.MarshalIndent(repairResult, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal repair result to JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// Display repair results
		displayRepairResults(repairResult)

		// Run final integrity check
		fmt.Println("\n🔍 Running final integrity check...")
		finalIntegrityResult, err := db.CheckIntegrity()
		if err != nil {
			logger.LogError(context.TODO(), err, "Failed to run final integrity check")
			fmt.Printf("⚠️  Warning: Could not verify repair results: %v\n", err)
		} else if finalIntegrityResult.Healthy {
			fmt.Println("✅ Database is now healthy")
		} else {
			fmt.Printf("⚠️  %d issues remain after repair\n", len(finalIntegrityResult.Issues))
			for _, issue := range finalIntegrityResult.Issues {
				if !issue.Repairable {
					fmt.Printf("  • [%s] %s - %s (not automatically repairable)\n",
						strings.ToUpper(issue.Severity), issue.Table, issue.Description)
				}
			}
		}

		logger.Info("Database repair completed",
			"success", repairResult.Success,
			"issues_repaired", repairResult.IssuesRepaired,
			"issues_remaining", repairResult.IssuesRemaining,
			"backup_created", repairResult.BackupCreated)

		return nil
	},
}

func displayRepairResults(result *storage.RepairResult) {
	fmt.Printf("\n📊 Repair Results\n")
	fmt.Printf("================\n")

	if result.Success {
		fmt.Printf("Status: ✅ Success\n")
	} else {
		fmt.Printf("Status: ❌ Failed\n")
	}

	fmt.Printf("Issues Repaired: %d\n", result.IssuesRepaired)
	fmt.Printf("Issues Remaining: %d\n", result.IssuesRemaining)
	fmt.Printf("Completed At: %s\n", result.RepairedAt.Format("2006-01-02 15:04:05"))

	if result.BackupCreated != "" {
		fmt.Printf("Backup Created: %s\n", result.BackupCreated)
	}

	if len(result.Actions) > 0 {
		fmt.Printf("\n🔧 Actions Performed:\n")
		for i, action := range result.Actions {
			fmt.Printf("  %d. %s on %s\n", i+1, action.Description, action.Table)
			if action.RecordsAffected > 0 {
				fmt.Printf("     Records affected: %d\n", action.RecordsAffected)
			}
		}
	}

	if result.Success && result.IssuesRepaired > 0 {
		fmt.Printf("\n✅ Successfully repaired %d issues\n", result.IssuesRepaired)
		if result.BackupCreated != "" {
			fmt.Printf("💾 Original database backed up to: %s\n", result.BackupCreated)
		}
	}
}

func init() {
	rootCmd.AddCommand(repairCmd)

	repairCmd.Flags().Bool("dry-run", false, "show what would be repaired without actually doing it")
	repairCmd.Flags().Bool("no-backup", false, "skip creating backup before repair")
	repairCmd.Flags().Bool("force", false, "skip confirmation prompts")
	repairCmd.Flags().Bool("json", false, "output repair results as JSON")
}
