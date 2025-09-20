package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/monitor"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start continuous monitoring of endpoints",
	Long: `Start continuous monitoring of all configured endpoints.

This command starts a background process that polls all registered endpoints
according to their configured intervals. The monitoring will continue until
stopped with Ctrl+C or a termination signal.

Examples:
  driftwatch monitor                    # Start monitoring all endpoints
  driftwatch monitor --duration 1h     # Monitor for 1 hour then stop
  driftwatch monitor --endpoints api1,api2  # Monitor specific endpoints only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		duration, err := cmd.Flags().GetDuration("duration")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "duration", err)
		}
		endpointIDs, err := cmd.Flags().GetStringSlice("endpoints")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "endpoints", err)
		}
		daemon, err := cmd.Flags().GetBool("daemon")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "daemon", err)
		}

		// Connect to storage
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Create HTTP client
		client := httpClient.NewClient(httpClient.ClientConfig{
			Timeout:    cfg.Global.Timeout,
			RetryCount: cfg.Global.RetryCount,
			RetryDelay: cfg.Global.RetryDelay,
			UserAgent:  cfg.Global.UserAgent,
		})

		// Create scheduler
		scheduler := monitor.NewCronScheduler(cfg, db, client)

		// Filter endpoints if specified
		if len(endpointIDs) > 0 {
			if err := filterEndpoints(cfg, endpointIDs); err != nil {
				return fmt.Errorf("failed to filter endpoints: %w", err)
			}
		}

		// Create context
		ctx := context.Background()
		if duration > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, duration)
			defer cancel()
		}

		// Start monitoring
		fmt.Printf("Starting monitoring of %d endpoints...\n", len(cfg.Endpoints))
		if err := scheduler.Start(ctx); err != nil {
			return fmt.Errorf("failed to start monitoring: %w", err)
		}

		if daemon {
			fmt.Println("Monitoring started in daemon mode")
			return nil
		}

		// Wait for completion or interruption
		if duration > 0 {
			fmt.Printf("Monitoring for %s... Press Ctrl+C to stop early\n", duration)
		} else {
			fmt.Println("Monitoring started... Press Ctrl+C to stop")
		}

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Wait for signal or context completion
		select {
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, stopping monitoring...\n", sig)
		case <-ctx.Done():
			if duration > 0 {
				fmt.Printf("\nMonitoring duration completed, stopping...\n")
			}
		}

		// Stop scheduler
		if err := scheduler.Stop(); err != nil {
			return fmt.Errorf("error stopping scheduler: %w", err)
		}

		fmt.Println("Monitoring stopped")
		return nil
	},
}

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Perform a one-time check of all endpoints",
	Long: `Perform a one-time check of all configured endpoints.

This command checks all registered endpoints once and reports the results.
It does not start continuous monitoring.

Examples:
  driftwatch check                      # Check all endpoints
  driftwatch check --endpoints api1,api2  # Check specific endpoints only
  driftwatch check --timeout 30s       # Use custom timeout`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		endpointIDs, err := cmd.Flags().GetStringSlice("endpoints")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "endpoints", err)
		}
		timeout, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "timeout", err)
		}
		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}

		// Connect to storage
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Create HTTP client
		client := httpClient.NewClient(httpClient.ClientConfig{
			Timeout:    cfg.Global.Timeout,
			RetryCount: cfg.Global.RetryCount,
			RetryDelay: cfg.Global.RetryDelay,
			UserAgent:  cfg.Global.UserAgent,
		})

		// Create scheduler
		scheduler := monitor.NewCronScheduler(cfg, db, client)

		// Filter endpoints if specified
		if len(endpointIDs) > 0 {
			if err := filterEndpoints(cfg, endpointIDs); err != nil {
				return fmt.Errorf("failed to filter endpoints: %w", err)
			}
		}

		// Create context with timeout
		ctx := context.Background()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Perform one-time check
		fmt.Printf("Checking %d endpoints...\n", len(cfg.Endpoints))
		start := time.Now()

		if err := scheduler.CheckOnce(ctx); err != nil {
			return fmt.Errorf("check failed: %w", err)
		}

		duration := time.Since(start)
		fmt.Printf("Check completed in %s\n", duration)

		// Display results based on output format
		status := scheduler.GetStatus()
		return displaySchedulerStatus(status, outputFormat)
	},
}

// statusCmd represents the status command for monitoring
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show monitoring status and endpoint health",
	Long: `Show the current status of the monitoring system and health of all endpoints.

This command displays information about the monitoring scheduler, endpoint statuses,
recent check results, and overall system health.

Examples:
  driftwatch status                     # Show status in table format
  driftwatch status --output json      # Show status in JSON format
  driftwatch status --detailed         # Show detailed endpoint information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}
		detailed, err := cmd.Flags().GetBool("detailed")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "detailed", err)
		}

		// Connect to storage
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Create HTTP client (for status only, not used)
		client := httpClient.NewClient(httpClient.ClientConfig{
			Timeout:    cfg.Global.Timeout,
			RetryCount: cfg.Global.RetryCount,
			RetryDelay: cfg.Global.RetryDelay,
			UserAgent:  cfg.Global.UserAgent,
		})

		// Create scheduler to get status
		scheduler := monitor.NewCronScheduler(cfg, db, client)

		// Get status
		status := scheduler.GetStatus()

		// Display status
		if detailed {
			return displayDetailedStatus(status, db, outputFormat)
		}

		return displaySchedulerStatus(status, outputFormat)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(statusCmd)

	// Monitor command flags
	monitorCmd.Flags().Duration("duration", 0, "monitoring duration (0 for indefinite)")
	monitorCmd.Flags().StringSlice("endpoints", []string{}, "specific endpoints to monitor (comma-separated)")
	monitorCmd.Flags().Bool("daemon", false, "run in daemon mode (background)")

	// Check command flags
	checkCmd.Flags().StringSlice("endpoints", []string{}, "specific endpoints to check (comma-separated)")
	checkCmd.Flags().Duration("timeout", 0, "timeout for the entire check operation")
	checkCmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")

	// Status command flags
	statusCmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	statusCmd.Flags().Bool("detailed", false, "show detailed endpoint information")
}

// Helper functions

// filterEndpoints filters the configuration to only include specified endpoints
func filterEndpoints(cfg *config.Config, endpointIDs []string) error {
	if len(endpointIDs) == 0 {
		return nil
	}

	// Create a map for quick lookup
	idMap := make(map[string]bool)
	for _, id := range endpointIDs {
		idMap[id] = true
	}

	// Filter endpoints
	var filteredEndpoints []config.EndpointConfig
	for _, ep := range cfg.Endpoints {
		if idMap[ep.ID] {
			filteredEndpoints = append(filteredEndpoints, ep)
			delete(idMap, ep.ID)
		}
	}

	// Check for non-existent endpoints
	if len(idMap) > 0 {
		var missing []string
		for id := range idMap {
			missing = append(missing, id)
		}
		return fmt.Errorf("endpoints not found: %v", missing)
	}

	cfg.Endpoints = filteredEndpoints
	return nil
}

// displaySchedulerStatus displays the scheduler status in the specified format
func displaySchedulerStatus(status monitor.SchedulerStatus, format string) error {
	switch format {
	case "json":
		return outputJSON(status)
	case "yaml":
		return outputYAML(status)
	case "table":
		return displayStatusTable(status)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// displayStatusTable displays status in table format
func displayStatusTable(status monitor.SchedulerStatus) error {
	fmt.Printf("Scheduler Status:\n")
	fmt.Printf("  Running: %t\n", status.Running)
	if !status.StartedAt.IsZero() {
		fmt.Printf("  Started: %s\n", status.StartedAt.Format(time.RFC3339))
	}
	fmt.Printf("  Endpoints: %d\n", status.EndpointsScheduled)
	if !status.LastCheckAt.IsZero() {
		fmt.Printf("  Last Check: %s\n", status.LastCheckAt.Format(time.RFC3339))
	}

	if len(status.EndpointStatuses) > 0 {
		fmt.Printf("\nEndpoint Status:\n")
		fmt.Printf("%-20s %-8s %-12s %-8s %-8s %-8s\n",
			"ID", "ENABLED", "LAST CHECK", "STATUS", "CHECKS", "ERRORS")
		fmt.Println(strings.Repeat("-", 80))

		for _, epStatus := range status.EndpointStatuses {
			lastCheck := "never"
			if !epStatus.LastCheck.IsZero() {
				lastCheck = epStatus.LastCheck.Format("15:04:05")
			}

			statusStr := "unknown"
			if epStatus.LastStatus > 0 {
				statusStr = fmt.Sprintf("%d", epStatus.LastStatus)
			}

			fmt.Printf("%-20s %-8t %-12s %-8s %-8d %-8d\n",
				truncateString(epStatus.ID, 17),
				epStatus.Enabled,
				lastCheck,
				statusStr,
				epStatus.CheckCount,
				epStatus.ErrorCount)
		}
	}

	return nil
}

// displayDetailedStatus displays detailed status information
func displayDetailedStatus(status monitor.SchedulerStatus, _ storage.Storage, format string) error {
	// For detailed status, we'd fetch additional information from the database
	// This is a simplified implementation
	return displaySchedulerStatus(status, format)
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// outputJSON outputs data in JSON format
func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// outputYAML outputs data in YAML format
func outputYAML(data interface{}) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}
