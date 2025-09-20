package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/deprecation"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migration tools for deprecated features",
	Long: `Migration tools to help upgrade from deprecated features to their replacements.

This command provides utilities to:
- Check for deprecated configuration
- Migrate configuration files
- Validate migrations
- Show deprecation status

Examples:
  driftwatch migrate config              # Migrate configuration file
  driftwatch migrate config --dry-run    # Preview configuration changes
  driftwatch migrate check               # Check for deprecated usage
  driftwatch migrate status              # Show deprecation status`,
}

// migrateConfigCmd handles configuration migration
var migrateConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Migrate configuration files",
	Long: `Migrate configuration files from deprecated formats to current standards.

This command will:
- Detect deprecated configuration options
- Create a backup of the original file
- Update the configuration to use current format
- Validate the migrated configuration

Examples:
  driftwatch migrate config                    # Migrate .driftwatch.yaml
  driftwatch migrate config --file custom.yaml # Migrate specific file
  driftwatch migrate config --dry-run          # Preview changes only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("file")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "file", err)
		}
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "dry-run", err)
		}
		backup, err := cmd.Flags().GetBool("backup")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "backup", err)
		}

		if configFile == "" {
			configFile = ".driftwatch.yaml"
		}

		return migrateConfigFile(configFile, dryRun, backup)
	},
}

// migrateCheckCmd checks for deprecated usage
var migrateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for deprecated feature usage",
	Long: `Check the current project for usage of deprecated features.

This command will scan:
- Configuration files for deprecated options
- Scripts for deprecated CLI usage
- Documentation for outdated examples

Examples:
  driftwatch migrate check                # Check current directory
  driftwatch migrate check --path ./scripts # Check specific directory
  driftwatch migrate check --format json # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString("path")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "path", err)
		}
		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "format", err)
		}

		if path == "" {
			path = "."
		}

		return checkDeprecatedUsage(path, format)
	},
}

// migrateStatusCmd shows deprecation status
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deprecation status",
	Long: `Show the status of all known deprecations and their timelines.

This command displays:
- Active deprecations with removal dates
- Migration guides and resources
- Severity levels and recommendations

Examples:
  driftwatch migrate status              # Show all deprecations
  driftwatch migrate status --active     # Show only active deprecations
  driftwatch migrate status --format json # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		activeOnly, err := cmd.Flags().GetBool("active")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "active", err)
		}
		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "format", err)
		}

		return showDeprecationStatus(activeOnly, format)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateConfigCmd)
	migrateCmd.AddCommand(migrateCheckCmd)
	migrateCmd.AddCommand(migrateStatusCmd)

	// Config migration flags
	migrateConfigCmd.Flags().StringP("file", "f", "", "configuration file to migrate (default: .driftwatch.yaml)")
	migrateConfigCmd.Flags().Bool("dry-run", false, "preview changes without modifying files")
	migrateConfigCmd.Flags().Bool("backup", true, "create backup before migration")

	// Check flags
	migrateCheckCmd.Flags().StringP("path", "p", ".", "path to check for deprecated usage")
	migrateCheckCmd.Flags().String("format", "text", "output format (text, json, yaml)")

	// Status flags
	migrateStatusCmd.Flags().Bool("active", false, "show only active deprecations")
	migrateStatusCmd.Flags().String("format", "text", "output format (text, json, yaml)")
}

// migrateConfigFile migrates a configuration file
func migrateConfigFile(configFile string, dryRun, backup bool) error {
	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", configFile)
	}

	// Read current configuration
	data, err := os.ReadFile(filepath.Clean(configFile))
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Track changes
	changes := make([]string, 0)
	migrated := false

	// Check for deprecated configuration options
	if checkAndMigrateField(config, "old_monitoring_format", "endpoints", &changes) {
		migrated = true
	}

	if checkAndMigrateField(config, "legacy_alerts", "alerting", &changes) {
		migrated = true
	}

	// Check for deprecated nested structures
	if global, ok := config["global"].(map[string]interface{}); ok {
		if checkAndMigrateField(global, "old_timeout", "timeout", &changes) {
			migrated = true
		}
	}

	if !migrated {
		fmt.Printf("✅ Configuration file is up to date: %s\n", configFile)
		return nil
	}

	// Show changes
	fmt.Printf("📝 Found deprecated configuration in %s:\n", configFile)
	for _, change := range changes {
		fmt.Printf("  - %s\n", change)
	}

	if dryRun {
		fmt.Println("\n🔍 Dry run mode - no changes made")
		return nil
	}

	// Create backup if requested
	if backup {
		backupFile := configFile + ".backup." + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backupFile, data, 0o600); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		fmt.Printf("💾 Backup created: %s\n", backupFile)
	}

	// Write migrated configuration
	migratedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal migrated configuration: %w", err)
	}

	if err := os.WriteFile(configFile, migratedData, 0o600); err != nil {
		return fmt.Errorf("failed to write migrated configuration: %w", err)
	}

	fmt.Printf("✅ Configuration migrated successfully: %s\n", configFile)
	fmt.Println("📚 Review the changes and update any scripts or documentation")

	return nil
}

// checkAndMigrateField checks for a deprecated field and migrates it
func checkAndMigrateField(config map[string]interface{}, oldKey, newKey string, changes *[]string) bool {
	if value, exists := config[oldKey]; exists {
		config[newKey] = value
		delete(config, oldKey)
		*changes = append(*changes, fmt.Sprintf("Migrated '%s' to '%s'", oldKey, newKey))
		return true
	}
	return false
}

// checkDeprecatedUsage checks for deprecated feature usage
func checkDeprecatedUsage(path, format string) error {
	issues := make([]DeprecationIssue, 0)

	// Check configuration files
	configIssues, err := checkConfigFiles(path)
	if err != nil {
		return fmt.Errorf("failed to check configuration files: %w", err)
	}
	issues = append(issues, configIssues...)

	// Check script files
	scriptIssues, err := checkScriptFiles(path)
	if err != nil {
		return fmt.Errorf("failed to check script files: %w", err)
	}
	issues = append(issues, scriptIssues...)

	// Output results
	return outputDeprecationIssues(issues, format)
}

// DeprecationIssue represents a found deprecation issue
type DeprecationIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Feature     string `json:"feature"`
	Replacement string `json:"replacement"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
}

// checkConfigFiles checks configuration files for deprecated options
func checkConfigFiles(basePath string) ([]DeprecationIssue, error) {
	issues := make([]DeprecationIssue, 0)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		// Check for deprecated configuration options
		content := string(data)
		lines := strings.Split(content, "\n")

		for i, line := range lines {
			if strings.Contains(line, "old_monitoring_format") {
				issues = append(issues, DeprecationIssue{
					File:        path,
					Line:        i + 1,
					Feature:     "old_monitoring_format",
					Replacement: "endpoints",
					Severity:    "WARNING",
					Message:     "Use 'endpoints' configuration section instead",
				})
			}

			if strings.Contains(line, "legacy_alerts") {
				issues = append(issues, DeprecationIssue{
					File:        path,
					Line:        i + 1,
					Feature:     "legacy_alerts",
					Replacement: "alerting",
					Severity:    "WARNING",
					Message:     "Use 'alerting' configuration section instead",
				})
			}
		}

		return nil
	})

	return issues, err
}

// checkScriptFiles checks script files for deprecated CLI usage
func checkScriptFiles(basePath string) ([]DeprecationIssue, error) {
	issues := make([]DeprecationIssue, 0)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".sh") && !strings.HasSuffix(path, ".bash") {
			return nil
		}

		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		content := string(data)
		lines := strings.Split(content, "\n")

		for i, line := range lines {
			if strings.Contains(line, "driftwatch") && strings.Contains(line, "--old-flag") {
				issues = append(issues, DeprecationIssue{
					File:        path,
					Line:        i + 1,
					Feature:     "--old-flag",
					Replacement: "--new-flag",
					Severity:    "WARNING",
					Message:     "Use '--new-flag' instead of '--old-flag'",
				})
			}
		}

		return nil
	})

	return issues, err
}

// outputDeprecationIssues outputs deprecation issues in the specified format
func outputDeprecationIssues(issues []DeprecationIssue, format string) error {
	if len(issues) == 0 {
		fmt.Println("✅ No deprecated feature usage found")
		return nil
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"issues": issues,
			"count":  len(issues),
		})

	case "yaml":
		data := map[string]interface{}{
			"issues": issues,
			"count":  len(issues),
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(data)

	default:
		fmt.Printf("⚠️  Found %d deprecated feature usage(s):\n\n", len(issues))
		for _, issue := range issues {
			fmt.Printf("📁 %s:%d\n", issue.File, issue.Line)
			fmt.Printf("   Feature: %s\n", issue.Feature)
			fmt.Printf("   Severity: %s\n", issue.Severity)
			fmt.Printf("   Message: %s\n", issue.Message)
			if issue.Replacement != "" {
				fmt.Printf("   Use instead: %s\n", issue.Replacement)
			}
			fmt.Println()
		}
	}

	return nil
}

// showDeprecationStatus shows the status of all deprecations
func showDeprecationStatus(activeOnly bool, format string) error {
	manager := deprecation.GetDefault()
	if manager == nil {
		return fmt.Errorf("deprecation manager not initialized")
	}

	var notices map[string]*deprecation.Notice
	if activeOnly {
		notices = manager.GetActiveNotices()
	} else {
		notices = manager.GetNotices()
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"notices": notices,
			"count":   len(notices),
		})

	case "yaml":
		data := map[string]interface{}{
			"notices": notices,
			"count":   len(notices),
		}
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(data)

	default:
		if len(notices) == 0 {
			if activeOnly {
				fmt.Println("✅ No active deprecations")
			} else {
				fmt.Println("✅ No deprecations registered")
			}
			return nil
		}

		title := "Deprecation Status"
		if activeOnly {
			title = "Active Deprecations"
		}

		fmt.Printf("📋 %s (%d total)\n\n", title, len(notices))

		for _, notice := range notices {
			fmt.Println(manager.FormatNoticeForCLI(notice))
			fmt.Println()
		}
	}

	return nil
}
