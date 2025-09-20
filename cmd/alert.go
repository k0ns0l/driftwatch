package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/alerting"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// alertCmd represents the alert command
var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Manage alert configuration and testing",
	Long: `The alert command provides functionality to manage alert channels,
test alert configurations, and view alert history.

Examples:
  driftwatch alert test                    # Test all configured alert channels
  driftwatch alert test --channel slack   # Test specific alert channel
  driftwatch alert history                # View alert history
  driftwatch alert history --drift-id 123 # View alerts for specific drift`,
}

// alertTestCmd represents the alert test command
var alertTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test alert channel configurations",
	Long: `Test alert channel configurations by sending test messages to verify
that the channels are properly configured and can receive alerts.

This command will send a test alert message to all enabled channels or
to a specific channel if specified with the --channel flag.`,
	RunE: runAlertTest,
}

// alertHistoryCmd represents the alert history command
var alertHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "View alert delivery history",
	Long: `View the history of alert deliveries, including successful sends,
failures, and retry attempts. Results can be filtered by various criteria.`,
	RunE: runAlertHistory,
}

var (
	alertChannelName string
	alertDriftID     int64
	alertStatus      string
	alertStartTime   string
	alertEndTime     string
	alertLimit       int
)

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.AddCommand(alertTestCmd)
	alertCmd.AddCommand(alertHistoryCmd)

	// Test command flags
	alertTestCmd.Flags().StringVar(&alertChannelName, "channel", "", "Test specific alert channel")

	// History command flags
	alertHistoryCmd.Flags().Int64Var(&alertDriftID, "drift-id", 0, "Filter by drift ID")
	alertHistoryCmd.Flags().StringVar(&alertChannelName, "channel", "", "Filter by channel name")
	alertHistoryCmd.Flags().StringVar(&alertStatus, "status", "", "Filter by status (sent, failed, pending, retry)")
	alertHistoryCmd.Flags().StringVar(&alertStartTime, "start", "", "Start time filter (RFC3339 format)")
	alertHistoryCmd.Flags().StringVar(&alertEndTime, "end", "", "End time filter (RFC3339 format)")
	alertHistoryCmd.Flags().IntVar(&alertLimit, "limit", 50, "Maximum number of results to return")
}

func runAlertTest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if alerting is enabled
	if !cfg.Alerting.Enabled {
		return fmt.Errorf("alerting is disabled in configuration")
	}

	// Create storage instance
	storage, err := storage.NewStorage(cfg.Global.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Create alert manager
	alertManager, err := alerting.NewAlertManager(cfg, storage)
	if err != nil {
		return fmt.Errorf("failed to create alert manager: %w", err)
	}

	// Test configuration
	fmt.Println("Testing alert configuration...")

	if alertChannelName != "" {
		fmt.Printf("Testing channel: %s\n", alertChannelName)
		// For specific channel testing, we would need to extend the interface
		// For now, test all channels and filter output
	}

	if err := alertManager.TestConfiguration(ctx); err != nil {
		fmt.Printf("❌ Alert test failed: %v\n", err)
		return err
	}

	fmt.Println("✅ All alert channels tested successfully!")
	return nil
}

func runAlertHistory(cmd *cobra.Command, args []string) error {
	alertManager, err := initializeAlertManager()
	if err != nil {
		return err
	}

	filters, err := parseAlertFilters()
	if err != nil {
		return err
	}

	alerts, err := fetchAlertHistory(alertManager, filters)
	if err != nil {
		return err
	}

	return displayAlertResults(cmd, alerts)
}

// initializeAlertManager creates storage and alert manager instances
func initializeAlertManager() (alerting.AlertManager, error) {
	storage, err := storage.NewStorage(cfg.Global.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	alertManager, err := alerting.NewAlertManager(cfg, storage)
	if err != nil {
		storage.Close()
		return nil, fmt.Errorf("failed to create alert manager: %w", err)
	}

	return alertManager, nil
}

// parseAlertFilters parses command line filters for alert history
func parseAlertFilters() (alerting.AlertFilters, error) {
	filters := alerting.AlertFilters{}

	if alertDriftID > 0 {
		filters.DriftID = &alertDriftID
	}

	if alertChannelName != "" {
		filters.ChannelName = alertChannelName
	}

	if alertStatus != "" {
		filters.Status = alertStatus
	}

	if err := parseTimeFilters(&filters); err != nil {
		return filters, err
	}

	return filters, nil
}

// parseTimeFilters parses start and end time filters
func parseTimeFilters(filters *alerting.AlertFilters) error {
	if alertStartTime != "" {
		startTime, err := time.Parse(time.RFC3339, alertStartTime)
		if err != nil {
			return fmt.Errorf("invalid start time format (use RFC3339): %w", err)
		}
		filters.StartTime = startTime
	}

	if alertEndTime != "" {
		endTime, err := time.Parse(time.RFC3339, alertEndTime)
		if err != nil {
			return fmt.Errorf("invalid end time format (use RFC3339): %w", err)
		}
		filters.EndTime = endTime
	}

	return nil
}

// fetchAlertHistory retrieves and limits alert history
func fetchAlertHistory(alertManager alerting.AlertManager, filters alerting.AlertFilters) ([]*alerting.Alert, error) {
	alerts, err := alertManager.GetAlertHistory(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert history: %w", err)
	}

	if alertLimit > 0 && len(alerts) > alertLimit {
		alerts = alerts[:alertLimit]
	}

	return alerts, nil
}

// displayAlertResults displays alert results in the requested format
func displayAlertResults(cmd *cobra.Command, alerts []*alerting.Alert) error {
	if len(alerts) == 0 {
		fmt.Println("No alerts found matching the specified criteria.")
		return nil
	}

	outputFormat, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("failed to get output format: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(alerts)
	case "yaml":
		return outputYAML(alerts)
	default:
		displayAlertHistoryTable(alerts)
		return nil
	}
}

func displayAlertHistoryTable(alerts []*alerting.Alert) {
	// Create table headers
	fmt.Printf("%-5s %-10s %-12s %-15s %-20s %-8s %-6s %s\n",
		"ID", "Drift ID", "Type", "Channel", "Sent At", "Status", "Retry", "Error")
	fmt.Println(strings.Repeat("-", 100))

	// Display each alert
	for _, alert := range alerts {
		sentAt := alert.SentAt.Format("2006-01-02 15:04")
		errorMsg := alert.ErrorMessage
		if len(errorMsg) > 30 {
			errorMsg = errorMsg[:27] + "..."
		}

		fmt.Printf("%-5d %-10d %-12s %-15s %-20s %-8s %-6d %s\n",
			alert.ID,
			alert.DriftID,
			alert.AlertType,
			alert.ChannelName,
			sentAt,
			alert.Status,
			alert.RetryCount,
			errorMsg)
	}

	fmt.Printf("\nTotal: %d alerts\n", len(alerts))
}
