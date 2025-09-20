// Package cmd contains all CLI commands for DriftWatch
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/k0ns0l/driftwatch/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
	logger  *logging.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "driftwatch",
	Short: "Monitor API endpoints for drift and changes",
	Long: `DriftWatch is a CLI tool that continuously monitors API endpoints 
and detects when their actual behavior drifts from their documented specifications.

The tool helps development teams catch breaking changes, undocumented modifications, 
and API evolution before they impact downstream consumers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --version flag
		versionFlag, err := cmd.Flags().GetBool("version")
		if err != nil {
			return fmt.Errorf("failed to get version flag: %w", err)
		}
		if versionFlag {
			fmt.Println(version.GetVersionString())
			return nil
		}
		return cmd.Help()
	},
}

// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .driftwatch.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "output format (table, json, yaml)")

	rootCmd.Flags().BoolP("version", "", false, "show version information")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Initialize logger first with basic configuration
	logConfig := logging.DefaultLoggerConfig()

	// Set log level based on verbose flag
	if rootCmd.Flag("verbose").Changed {
		logConfig.Level = logging.LogLevelDebug
	}

	var err error
	logger, err = logging.NewLogger(logConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}

	// Initialize global logger
	if err := logging.InitGlobalLogger(logConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing global logger: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		if dwe, ok := err.(*errors.DriftWatchError); ok {
			logger.LogError(context.TODO(), dwe, "Failed to load configuration")
			fmt.Fprintf(os.Stderr, "Configuration Error: %s\n", dwe.Message)
			if dwe.Guidance != "" {
				fmt.Fprintf(os.Stderr, "Guidance: %s\n", dwe.Guidance)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		}
		os.Exit(1)
	}

	// Print config file location if verbose
	if rootCmd.Flag("verbose").Changed {
		configPath := config.GetConfigFilePath(cfgFile)
		if config.ConfigExists(configPath) {
			logger.Info("Using config file", "path", configPath)
		} else {
			logger.Info("Using default configuration (no config file found)")
		}
	}
}

// GetConfig returns the loaded configuration
func GetConfig() *config.Config {
	return cfg
}

// GetLogger returns the initialized logger
func GetLogger() *logging.Logger {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}
	return logger
}
