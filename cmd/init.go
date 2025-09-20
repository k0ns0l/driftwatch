package cmd

import (
	"fmt"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new DriftWatch project",
	Long: `Initialize a new DriftWatch project by creating a default configuration file.

This command creates a .driftwatch.yaml configuration file in the current directory
with sensible defaults and example settings to get you started quickly.

Examples:
  driftwatch init                    # Create config in current directory
  driftwatch init --config my.yaml  # Create config with custom filename
  driftwatch init --force           # Overwrite existing config file`,
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

		fmt.Printf("✓ Configuration file created at %s\n", configPath)
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("1. Edit the configuration file to add your API endpoints\n")
		fmt.Printf("2. Set environment variables for sensitive values (e.g., API_TOKEN, SLACK_WEBHOOK_URL)\n")
		fmt.Printf("3. Validate your configuration: driftwatch config validate\n")
		fmt.Printf("4. Start monitoring: driftwatch monitor\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Add flags
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing configuration file")
}
