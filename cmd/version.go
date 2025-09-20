package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/k0ns0l/driftwatch/internal/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display version information for DriftWatch including version number,
git commit, build date, Go version, and platform information.

Examples:
  driftwatch version              # Show basic version info
  driftwatch version --output json  # Show version info in JSON format
  driftwatch version --detailed   # Show detailed version information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}
		detailed, err := cmd.Flags().GetBool("detailed")
		if err != nil {
			return fmt.Errorf("failed to get detailed flag: %w", err)
		}

		versionInfo := version.GetVersion()

		switch outputFormat {
		case "json":
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(versionInfo)
		case "yaml":
			encoder := yaml.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent(2)
			defer encoder.Close()
			return encoder.Encode(versionInfo)
		default:
			if detailed {
				fmt.Fprintln(cmd.OutOrStdout(), version.GetDetailedVersionString())
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), version.GetVersionString())
			}
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Add flags
	versionCmd.Flags().StringP("output", "o", "text", "output format (text, json, yaml)")
	versionCmd.Flags().BoolP("detailed", "d", false, "show detailed version information")
}
