package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage DriftWatch configuration including viewing, validating, and initializing config files.

Examples:
  driftwatch config show          # Show current configuration
  driftwatch config validate     # Validate configuration
  driftwatch config init         # Initialize default configuration file`,
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current DriftWatch configuration in the specified format.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}

		switch outputFormat {
		case "json":
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(cfg)
		case "yaml":
			encoder := yaml.NewEncoder(os.Stdout)
			encoder.SetIndent(2)
			defer encoder.Close()
			return encoder.Encode(cfg)
		default:
			return fmt.Errorf("unsupported output format: %s (supported: json, yaml)", outputFormat)
		}
	},
}

// configValidateCmd validates the configuration
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  `Validate the current DriftWatch configuration and report any errors.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "config", err)
		}

		// Load config to trigger validation
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration validation failed:\n%v\n", err)
			return err
		}

		fmt.Printf("Configuration is valid ✓\n")
		fmt.Printf("- Project: %s\n", cfg.Project.Name)
		fmt.Printf("- Endpoints: %d configured\n", len(cfg.Endpoints))
		fmt.Printf("- Alerting: %s\n", map[bool]string{true: "enabled", false: "disabled"}[cfg.Alerting.Enabled])

		return nil
	},
}

// configInitCmd initializes a default configuration file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default configuration file",
	Long:  `Create a default DriftWatch configuration file with example settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "config", err)
		}
		configPath := config.GetConfigFilePath(configFile)

		// Check if config file already exists
		if config.ConfigExists(configPath) {
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "force", err)
			}
			if !force {
				return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
			}
		}

		// Create default config file
		if err := config.CreateDefaultConfigFile(configPath); err != nil {
			return fmt.Errorf("failed to create configuration file: %w", err)
		}

		fmt.Printf("Configuration file created at %s ✓\n", configPath)
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("1. Edit the configuration file to add your API endpoints\n")
		fmt.Printf("2. Set environment variables for sensitive values (e.g., API_TOKEN, SLACK_WEBHOOK_URL)\n")
		fmt.Printf("3. Validate your configuration: driftwatch config validate\n")
		fmt.Printf("4. Start monitoring: driftwatch monitor\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configInitCmd)

	// Add flags
	configShowCmd.Flags().StringP("output", "o", "yaml", "output format (json, yaml)")
	configInitCmd.Flags().BoolP("force", "f", false, "overwrite existing configuration file")
}
