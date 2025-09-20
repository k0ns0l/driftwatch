package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// dbHealthCmd represents the database health command
var dbHealthCmd = &cobra.Command{
	Use:   "db-health",
	Short: "Check the health status of the DriftWatch database",
	Long: `Check the comprehensive health status of the DriftWatch database including
integrity, performance metrics, and maintenance recommendations.

This command performs various checks including:
- Database integrity verification
- Fragmentation analysis
- Size and performance metrics
- Maintenance recommendations

Examples:
  driftwatch health                    # Show health status summary
  driftwatch health --detailed         # Show detailed health information
  driftwatch health --json             # Output health status as JSON
  driftwatch health --check-integrity  # Focus on integrity checking only`,
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
		detailed, err := cmd.Flags().GetBool("detailed")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "detailed", err)
		}
		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "json", err)
		}
		checkIntegrityOnly, err := cmd.Flags().GetBool("check-integrity")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "check-integrity", err)
		}

		if checkIntegrityOnly {
			return runIntegrityCheck(db, jsonOutput)
		}

		// Get comprehensive health status
		fmt.Println("🔍 Checking database health...")
		healthStatus, err := db.GetHealthStatus()
		if err != nil {
			return fmt.Errorf("failed to get health status: %w", err)
		}

		if jsonOutput {
			return outputHealthJSON(healthStatus)
		}

		// Display health status
		displayHealthStatus(healthStatus, detailed)

		logger.Info("Database health check completed",
			"healthy", healthStatus.Healthy,
			"status", healthStatus.Status,
			"integrity_issues", healthStatus.IntegrityIssues)

		return nil
	},
}

func runIntegrityCheck(db storage.Storage, jsonOutput bool) error {
	fmt.Println("🔍 Running database integrity check...")

	integrityResult, err := db.CheckIntegrity()
	if err != nil {
		return fmt.Errorf("failed to check database integrity: %w", err)
	}

	if jsonOutput {
		jsonData, err := json.MarshalIndent(integrityResult, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal integrity result to JSON: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	// Display integrity results
	fmt.Printf("\n📊 Integrity Check Results\n")
	fmt.Printf("========================\n")
	fmt.Printf("Overall Status: %s\n", getHealthEmoji(integrityResult.Healthy))
	fmt.Printf("Tables Checked: %d\n", integrityResult.TablesChecked)
	fmt.Printf("Issues Found: %d\n", len(integrityResult.Issues))
	fmt.Printf("Checked At: %s\n", integrityResult.CheckedAt.Format("2006-01-02 15:04:05"))

	if len(integrityResult.OrphanedRecords) > 0 {
		fmt.Printf("\n🔗 Orphaned Records:\n")
		for table, count := range integrityResult.OrphanedRecords {
			fmt.Printf("  • %s: %d records\n", table, count)
		}
	}

	if len(integrityResult.Issues) > 0 {
		fmt.Printf("\n⚠️  Issues Found:\n")
		for i, issue := range integrityResult.Issues {
			fmt.Printf("  %d. [%s] %s - %s\n", i+1, strings.ToUpper(issue.Severity), issue.Table, issue.Description)
			if issue.Repairable {
				fmt.Printf("     💡 This issue can be repaired with 'driftwatch repair'\n")
			}
		}
	} else {
		fmt.Println("\n✅ No integrity issues found")
	}

	return nil
}

func outputHealthJSON(healthStatus *storage.HealthStatus) error {
	jsonData, err := json.MarshalIndent(healthStatus, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal health status to JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

func displayHealthStatus(healthStatus *storage.HealthStatus, detailed bool) {
	// Header
	fmt.Printf("\n🏥 Database Health Status\n")
	fmt.Printf("========================\n")

	// Overall status
	statusEmoji := getStatusEmoji(healthStatus.Status)
	healthEmoji := getHealthEmoji(healthStatus.Healthy)

	fmt.Printf("Overall Health: %s %s\n", healthEmoji, strings.ToUpper(string(healthStatus.Status[0]))+healthStatus.Status[1:])
	fmt.Printf("Database Size: %.2f MB\n", float64(healthStatus.DatabaseSize)/1024/1024)
	fmt.Printf("Schema Version: %d\n", healthStatus.SchemaVersion)
	fmt.Printf("Checked At: %s\n", healthStatus.CheckedAt.Format("2006-01-02 15:04:05"))

	// Integrity status
	if healthStatus.IntegrityIssues > 0 {
		fmt.Printf("Integrity Issues: ⚠️  %d issues found\n", healthStatus.IntegrityIssues)
	} else {
		fmt.Printf("Integrity Issues: ✅ None\n")
	}

	// Fragmentation
	if healthStatus.FragmentationLevel > 0 {
		fragEmoji := "✅"
		if healthStatus.FragmentationLevel > 25 {
			fragEmoji = "⚠️ "
		} else if healthStatus.FragmentationLevel > 10 {
			fragEmoji = "💡"
		}
		fmt.Printf("Fragmentation: %s %.1f%%\n", fragEmoji, healthStatus.FragmentationLevel)
	}

	// Last backup
	if healthStatus.LastBackup != nil {
		fmt.Printf("Last Backup: %s\n", healthStatus.LastBackup.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Backup: ⚠️  No backup information available\n")
	}

	// Recommendations
	if len(healthStatus.Recommendations) > 0 {
		fmt.Printf("\n💡 Recommendations:\n")
		for i, rec := range healthStatus.Recommendations {
			priorityEmoji := getPriorityEmoji(rec.Priority)
			fmt.Printf("  %d. %s [%s] %s\n", i+1, priorityEmoji, strings.ToUpper(rec.Priority), rec.Description)
			if detailed {
				fmt.Printf("     Action: %s\n", rec.Action)
			}
		}
	} else {
		fmt.Printf("\n✅ No maintenance recommendations\n")
	}

	// Detailed information
	if detailed {
		fmt.Printf("\n📊 Detailed Information:\n")
		fmt.Printf("  • Database file fragmentation: %.1f%%\n", healthStatus.FragmentationLevel)
		fmt.Printf("  • Integrity issues detected: %d\n", healthStatus.IntegrityIssues)
		fmt.Printf("  • Current schema version: %d\n", healthStatus.SchemaVersion)
	}

	// Summary
	fmt.Printf("\n%s Summary: Database is %s", statusEmoji, healthStatus.Status)
	if !healthStatus.Healthy {
		fmt.Printf(" - %d issues need attention", len(healthStatus.Recommendations))
	}
	fmt.Println()
}

func getHealthEmoji(healthy bool) string {
	if healthy {
		return "✅"
	}
	return "❌"
}

func getStatusEmoji(status string) string {
	switch status {
	case "excellent":
		return "🟢"
	case "good":
		return "🟡"
	case "warning":
		return "🟠"
	case "critical":
		return "🔴"
	default:
		return "⚪"
	}
}

func getPriorityEmoji(priority string) string {
	switch priority {
	case "critical":
		return "🚨"
	case "high":
		return "⚠️ "
	case "medium":
		return "💡"
	case "low":
		return "ℹ️ "
	default:
		return "📝"
	}
}

func init() {
	rootCmd.AddCommand(dbHealthCmd)

	dbHealthCmd.Flags().Bool("detailed", false, "show detailed health information")
	dbHealthCmd.Flags().Bool("json", false, "output health status as JSON")
	dbHealthCmd.Flags().Bool("check-integrity", false, "focus on integrity checking only")
}
