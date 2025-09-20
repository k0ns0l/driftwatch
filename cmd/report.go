package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/security"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate drift reports and analysis",
	Long: `Generate comprehensive reports about detected API drifts and changes over time.

This command analyzes monitoring data and provides insights into API evolution,
breaking changes, and drift patterns across different time periods.

Examples:
  driftwatch report                    # Generate report for last 24 hours
  driftwatch report --period 7d       # Generate report for last 7 days
  driftwatch report --period 30d      # Generate report for last 30 days
  driftwatch report --endpoint my-api # Report for specific endpoint
  driftwatch report --severity high   # Show only high severity drifts
  driftwatch report --output json     # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		period, err := cmd.Flags().GetString("period")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "period", err)
		}
		endpointID, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "endpoint", err)
		}
		severity, err := cmd.Flags().GetString("severity")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "severity", err)
		}
		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}
		acknowledged, err := cmd.Flags().GetBool("acknowledged")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "acknowledged", err)
		}
		unacknowledged, err := cmd.Flags().GetBool("unacknowledged")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "unacknowledged", err)
		}

		// Parse time period
		duration, err := parsePeriod(period)
		if err != nil {
			return fmt.Errorf("invalid period: %w", err)
		}

		// Connect to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Build drift filters
		filters := storage.DriftFilters{
			EndpointID: endpointID,
			Severity:   severity,
			StartTime:  time.Now().Add(-duration),
			EndTime:    time.Now(),
		}

		// Handle acknowledged filter
		if acknowledged && !unacknowledged {
			ack := true
			filters.Acknowledged = &ack
		} else if unacknowledged && !acknowledged {
			ack := false
			filters.Acknowledged = &ack
		}

		// Get drifts
		drifts, err := db.GetDrifts(filters)
		if err != nil {
			return fmt.Errorf("failed to get drifts: %w", err)
		}

		// Generate report
		report := generateDriftReport(drifts, duration)

		// Output report based on format
		switch outputFormat {
		case "json":
			return outputReportJSON(report)
		case "yaml":
			return outputReportYAML(report)
		case "table":
			outputReportTable(report)
			return nil
		default:
			return fmt.Errorf("unsupported output format: %s (supported: table, json, yaml)", outputFormat)
		}
	},
}

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show endpoint health and monitoring status",
	Long: `Display the current health status of all monitored endpoints including
response times, success rates, and recent drift activity.

This command provides a quick overview of your API monitoring setup and
helps identify endpoints that may need attention.

Examples:
  driftwatch health                    # Show health for all endpoints
  driftwatch health --endpoint my-api # Show health for specific endpoint
  driftwatch health --unhealthy-only  # Show only unhealthy endpoints
  driftwatch health --output json     # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		endpointID, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "endpoint", err)
		}
		outputFormat, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}
		unhealthyOnly, err := cmd.Flags().GetBool("unhealthy-only")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "unhealthy-only", err)
		}

		// Connect to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Get endpoints to check
		var endpoints []string
		if endpointID != "" {
			endpoints = []string{endpointID}
		} else {
			// Get all endpoint IDs from config
			for _, ep := range cfg.Endpoints {
				endpoints = append(endpoints, ep.ID)
			}
		}

		// Generate status report
		statusReport := generateStatusReport(db, endpoints, unhealthyOnly)

		// Output status based on format
		switch outputFormat {
		case "json":
			return outputStatusJSON(statusReport)
		case "yaml":
			return outputStatusYAML(statusReport)
		case "table":
			outputStatusTable(statusReport)
			return nil
		default:
			return fmt.Errorf("unsupported output format: %s (supported: table, json, yaml)", outputFormat)
		}
	},
}

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export monitoring data and drift history",
	Long: `Export historical monitoring data and drift information to various formats
for analysis, reporting, or backup purposes.

This command allows you to extract data from the DriftWatch database
in structured formats suitable for external analysis tools.

Examples:
  driftwatch export --format csv      # Export to CSV format
  driftwatch export --format json     # Export to JSON format
  driftwatch export --period 30d      # Export last 30 days of data
  driftwatch export --endpoint my-api # Export data for specific endpoint
  driftwatch export --type drifts     # Export only drift data
  driftwatch export --type runs       # Export only monitoring runs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get flags
		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "format", err)
		}
		period, err := cmd.Flags().GetString("period")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "period", err)
		}
		endpointID, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "endpoint", err)
		}
		dataType, err := cmd.Flags().GetString("type")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "type", err)
		}
		output, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "output", err)
		}

		// Parse time period
		duration, err := parsePeriod(period)
		if err != nil {
			return fmt.Errorf("invalid period: %w", err)
		}

		// Connect to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Export data based on type
		switch dataType {
		case "drifts":
			return exportDrifts(db, format, endpointID, duration, output)
		case "runs":
			return exportMonitoringRuns(db, format, endpointID, duration, output)
		case "all":
			return exportAllData(db, format, endpointID, duration, output)
		default:
			return fmt.Errorf("unsupported data type: %s (supported: drifts, runs, all)", dataType)
		}
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(exportCmd)

	// Report command flags
	reportCmd.Flags().StringP("period", "p", "24h", "time period for report (24h, 7d, 30d)")
	reportCmd.Flags().StringP("endpoint", "e", "", "filter by specific endpoint ID")
	reportCmd.Flags().StringP("severity", "s", "", "filter by severity (low, medium, high, critical)")
	reportCmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	reportCmd.Flags().Bool("acknowledged", false, "show only acknowledged drifts")
	reportCmd.Flags().Bool("unacknowledged", false, "show only unacknowledged drifts")

	// Health command flags
	healthCmd.Flags().StringP("endpoint", "e", "", "show health for specific endpoint ID")
	healthCmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	healthCmd.Flags().Bool("unhealthy-only", false, "show only unhealthy endpoints")

	// Export command flags
	exportCmd.Flags().StringP("format", "f", "json", "export format (json, csv, yaml)")
	exportCmd.Flags().StringP("period", "p", "30d", "time period to export (24h, 7d, 30d)")
	exportCmd.Flags().StringP("endpoint", "e", "", "filter by specific endpoint ID")
	exportCmd.Flags().StringP("type", "t", "all", "data type to export (drifts, runs, all)")
	exportCmd.Flags().StringP("output", "o", "", "output file (default: stdout)")
}

// Data structures for reporting

// DriftReport represents a comprehensive drift analysis report
type DriftReport struct {
	Period    string           `json:"period" yaml:"period"`
	StartTime time.Time        `json:"start_time" yaml:"start_time"`
	EndTime   time.Time        `json:"end_time" yaml:"end_time"`
	Summary   DriftSummary     `json:"summary" yaml:"summary"`
	Drifts    []*storage.Drift `json:"drifts" yaml:"drifts"`
	Trends    DriftTrends      `json:"trends" yaml:"trends"`
}

// DriftSummary provides high-level statistics about drifts
type DriftSummary struct {
	TotalDrifts      int            `json:"total_drifts" yaml:"total_drifts"`
	BySeverity       map[string]int `json:"by_severity" yaml:"by_severity"`
	ByEndpoint       map[string]int `json:"by_endpoint" yaml:"by_endpoint"`
	ByType           map[string]int `json:"by_type" yaml:"by_type"`
	AcknowledgedRate float64        `json:"acknowledged_rate" yaml:"acknowledged_rate"`
}

// DriftTrends provides trend analysis over time
type DriftTrends struct {
	DailyBreakdown      []DayBreakdown     `json:"daily_breakdown" yaml:"daily_breakdown"`
	MostActiveEndpoints []EndpointActivity `json:"most_active_endpoints" yaml:"most_active_endpoints"`
}

// DayBreakdown represents drift activity for a single day
type DayBreakdown struct {
	Date   string `json:"date" yaml:"date"`
	Count  int    `json:"count" yaml:"count"`
	Severe int    `json:"severe" yaml:"severe"` // high + critical
}

// EndpointActivity represents drift activity for an endpoint
type EndpointActivity struct {
	EndpointID string `json:"endpoint_id" yaml:"endpoint_id"`
	Count      int    `json:"count" yaml:"count"`
	Severe     int    `json:"severe" yaml:"severe"`
}

// StatusReport represents endpoint health status
type StatusReport struct {
	GeneratedAt time.Time        `json:"generated_at" yaml:"generated_at"`
	Summary     StatusSummary    `json:"summary" yaml:"summary"`
	Endpoints   []EndpointStatus `json:"endpoints" yaml:"endpoints"`
}

// StatusSummary provides high-level health statistics
type StatusSummary struct {
	TotalEndpoints     int `json:"total_endpoints" yaml:"total_endpoints"`
	HealthyEndpoints   int `json:"healthy_endpoints" yaml:"healthy_endpoints"`
	UnhealthyEndpoints int `json:"unhealthy_endpoints" yaml:"unhealthy_endpoints"`
	UnknownEndpoints   int `json:"unknown_endpoints" yaml:"unknown_endpoints"`
}

// EndpointStatus represents the health status of a single endpoint
type EndpointStatus struct {
	ID               string    `json:"id" yaml:"id"`
	URL              string    `json:"url" yaml:"url"`
	Method           string    `json:"method" yaml:"method"`
	Status           string    `json:"status" yaml:"status"` // healthy, unhealthy, unknown
	LastChecked      time.Time `json:"last_checked" yaml:"last_checked"`
	LastResponseTime int64     `json:"last_response_time_ms" yaml:"last_response_time_ms"`
	SuccessRate      float64   `json:"success_rate" yaml:"success_rate"`
	RecentDrifts     int       `json:"recent_drifts" yaml:"recent_drifts"`
	Enabled          bool      `json:"enabled" yaml:"enabled"`
}

// Helper functions

// parsePeriod converts period strings to time.Duration
func parsePeriod(period string) (time.Duration, error) {
	switch strings.ToLower(period) {
	case "24h", "1d", "day":
		return 24 * time.Hour, nil
	case "7d", "week", "1w":
		return 7 * 24 * time.Hour, nil
	case "30d", "month", "1m":
		return 30 * 24 * time.Hour, nil
	default:
		// Try to parse as duration
		return time.ParseDuration(period)
	}
}

// generateDriftReport creates a comprehensive drift analysis report
func generateDriftReport(drifts []*storage.Drift, period time.Duration) *DriftReport {
	now := time.Now()
	startTime := now.Add(-period)

	report := &DriftReport{
		Period:    formatPeriod(period),
		StartTime: startTime,
		EndTime:   now,
		Drifts:    drifts,
		Summary:   generateDriftSummary(drifts),
		Trends:    generateDriftTrends(drifts, startTime, now),
	}

	return report
}

// generateDriftSummary creates summary statistics for drifts
func generateDriftSummary(drifts []*storage.Drift) DriftSummary {
	summary := DriftSummary{
		TotalDrifts: len(drifts),
		BySeverity:  make(map[string]int),
		ByEndpoint:  make(map[string]int),
		ByType:      make(map[string]int),
	}

	acknowledgedCount := 0

	for _, drift := range drifts {
		// Count by severity
		summary.BySeverity[drift.Severity]++

		// Count by endpoint
		summary.ByEndpoint[drift.EndpointID]++

		// Count by type
		summary.ByType[drift.DriftType]++

		// Count acknowledged
		if drift.Acknowledged {
			acknowledgedCount++
		}
	}

	// Calculate acknowledged rate
	if len(drifts) > 0 {
		summary.AcknowledgedRate = float64(acknowledgedCount) / float64(len(drifts)) * 100
	}

	return summary
}

// generateDriftTrends creates trend analysis for drifts
func generateDriftTrends(drifts []*storage.Drift, startTime time.Time, endTime time.Time) DriftTrends {
	trends := DriftTrends{
		DailyBreakdown:      make([]DayBreakdown, 0),
		MostActiveEndpoints: make([]EndpointActivity, 0),
	}

	// Generate daily breakdown
	dailyMap := make(map[string]*DayBreakdown)
	endpointMap := make(map[string]*EndpointActivity)

	for _, drift := range drifts {
		// Skip drifts outside the time range
		if drift.DetectedAt.Before(startTime) || drift.DetectedAt.After(endTime) {
			continue
		}

		// Daily breakdown
		dateKey := drift.DetectedAt.Format("2006-01-02")
		if _, exists := dailyMap[dateKey]; !exists {
			dailyMap[dateKey] = &DayBreakdown{
				Date:   dateKey,
				Count:  0,
				Severe: 0,
			}
		}
		dailyMap[dateKey].Count++
		if drift.Severity == "high" || drift.Severity == "critical" {
			dailyMap[dateKey].Severe++
		}

		// Endpoint activity
		if _, exists := endpointMap[drift.EndpointID]; !exists {
			endpointMap[drift.EndpointID] = &EndpointActivity{
				EndpointID: drift.EndpointID,
				Count:      0,
				Severe:     0,
			}
		}
		endpointMap[drift.EndpointID].Count++
		if drift.Severity == "high" || drift.Severity == "critical" {
			endpointMap[drift.EndpointID].Severe++
		}
	}

	// Convert maps to slices
	for _, breakdown := range dailyMap {
		trends.DailyBreakdown = append(trends.DailyBreakdown, *breakdown)
	}

	for _, activity := range endpointMap {
		trends.MostActiveEndpoints = append(trends.MostActiveEndpoints, *activity)
	}

	return trends
}

// generateStatusReport creates a comprehensive status report
func generateStatusReport(db storage.Storage, endpointIDs []string, unhealthyOnly bool) *StatusReport {
	report := &StatusReport{
		GeneratedAt: time.Now(),
		Endpoints:   make([]EndpointStatus, 0),
	}

	for _, endpointID := range endpointIDs {
		// Get endpoint info
		endpoint, err := db.GetEndpoint(endpointID)
		if err != nil {
			continue // Skip if endpoint not found
		}

		// Get recent monitoring runs (last 24 hours)
		runs, err := db.GetMonitoringHistory(endpointID, 24*time.Hour)
		if err != nil {
			continue
		}

		// Get recent drifts (last 7 days)
		drifts, err := db.GetDrifts(storage.DriftFilters{
			EndpointID: endpointID,
			StartTime:  time.Now().Add(-7 * 24 * time.Hour),
		})
		if err != nil {
			continue
		}

		// Calculate status
		status := calculateEndpointStatus(runs)
		successRate := calculateSuccessRate(runs)

		var lastChecked time.Time
		var lastResponseTime int64

		if len(runs) > 0 {
			lastRun := runs[0] // Most recent run
			lastChecked = lastRun.Timestamp
			lastResponseTime = lastRun.ResponseTimeMs
		}

		endpointStatus := EndpointStatus{
			ID:               endpointID,
			URL:              endpoint.URL,
			Method:           endpoint.Method,
			Status:           status,
			LastChecked:      lastChecked,
			LastResponseTime: lastResponseTime,
			SuccessRate:      successRate,
			RecentDrifts:     len(drifts),
			Enabled:          true, // We'll need to parse the config JSON to get this
		}

		// Filter unhealthy only if requested
		if unhealthyOnly && status == "healthy" {
			continue
		}

		report.Endpoints = append(report.Endpoints, endpointStatus)
	}

	// Generate summary
	report.Summary = generateStatusSummary(report.Endpoints)

	return report
}

// calculateEndpointStatus determines the health status of an endpoint
func calculateEndpointStatus(runs []*storage.MonitoringRun) string {
	if len(runs) == 0 {
		return "unknown"
	}

	// Check recent runs (last 5 or all if less than 5)
	checkCount := 5
	if len(runs) < checkCount {
		checkCount = len(runs)
	}

	healthyCount := 0
	for i := 0; i < checkCount; i++ {
		if runs[i].ResponseStatus >= 200 && runs[i].ResponseStatus < 300 {
			healthyCount++
		}
	}

	// Consider healthy if at least 80% of recent checks were successful
	if float64(healthyCount)/float64(checkCount) >= 0.8 {
		return "healthy"
	}

	return "unhealthy"
}

// calculateSuccessRate calculates the success rate over recent runs
func calculateSuccessRate(runs []*storage.MonitoringRun) float64 {
	if len(runs) == 0 {
		return 0.0
	}

	successCount := 0
	for _, run := range runs {
		if run.ResponseStatus >= 200 && run.ResponseStatus < 300 {
			successCount++
		}
	}

	return float64(successCount) / float64(len(runs)) * 100
}

// generateStatusSummary creates summary statistics for status report
func generateStatusSummary(endpoints []EndpointStatus) StatusSummary {
	summary := StatusSummary{
		TotalEndpoints: len(endpoints),
	}

	for _, ep := range endpoints {
		switch ep.Status {
		case "healthy":
			summary.HealthyEndpoints++
		case "unhealthy":
			summary.UnhealthyEndpoints++
		default:
			summary.UnknownEndpoints++
		}
	}

	return summary
}

// formatPeriod converts duration to human-readable string
func formatPeriod(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	days := hours / 24
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// Output functions for reports

// outputReportJSON outputs drift report in JSON format
func outputReportJSON(report *DriftReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputReportYAML outputs drift report in YAML format
func outputReportYAML(report *DriftReport) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(report)
}

// outputReportTable outputs drift report in table format
func outputReportTable(report *DriftReport) {
	fmt.Printf("DriftWatch Report - %s (%s to %s)\n",
		report.Period,
		report.StartTime.Format("2006-01-02 15:04"),
		report.EndTime.Format("2006-01-02 15:04"))
	fmt.Println(strings.Repeat("=", 80))

	// Summary section
	fmt.Printf("\nSUMMARY\n")
	fmt.Printf("Total Drifts: %d\n", report.Summary.TotalDrifts)
	fmt.Printf("Acknowledged Rate: %.1f%%\n", report.Summary.AcknowledgedRate)

	if len(report.Summary.BySeverity) > 0 {
		fmt.Printf("\nBy Severity:\n")
		for severity, count := range report.Summary.BySeverity {
			fmt.Printf("  %s: %d\n", strings.ToUpper(string(severity[0]))+severity[1:], count)
		}
	}

	if len(report.Summary.ByEndpoint) > 0 {
		fmt.Printf("\nBy Endpoint:\n")
		for endpoint, count := range report.Summary.ByEndpoint {
			fmt.Printf("  %s: %d\n", endpoint, count)
		}
	}

	// Recent drifts section
	if len(report.Drifts) > 0 {
		fmt.Printf("\nRECENT DRIFTS\n")
		fmt.Printf("%-20s %-10s %-15s %-30s %-10s\n",
			"ENDPOINT", "SEVERITY", "TYPE", "DESCRIPTION", "STATUS")
		fmt.Println(strings.Repeat("-", 95))

		// Show up to 10 most recent drifts
		displayCount := 10
		if len(report.Drifts) < displayCount {
			displayCount = len(report.Drifts)
		}

		for i := 0; i < displayCount; i++ {
			drift := report.Drifts[i]
			status := "New"
			if drift.Acknowledged {
				status = "Acked"
			}

			// Truncate long descriptions
			description := drift.Description
			if len(description) > 27 {
				description = description[:24] + "..."
			}

			// Truncate long endpoint IDs
			endpointID := drift.EndpointID
			if len(endpointID) > 17 {
				endpointID = endpointID[:14] + "..."
			}

			fmt.Printf("%-20s %-10s %-15s %-30s %-10s\n",
				endpointID,
				strings.ToUpper(string(drift.Severity[0]))+drift.Severity[1:],
				drift.DriftType,
				description,
				status)
		}

		if len(report.Drifts) > displayCount {
			fmt.Printf("\n... and %d more drifts\n", len(report.Drifts)-displayCount)
		}
	}

	// Trends section
	if len(report.Trends.DailyBreakdown) > 0 {
		fmt.Printf("\nTREND ANALYSIS\n")
		fmt.Printf("Daily Activity (last %d days):\n", len(report.Trends.DailyBreakdown))
		for _, day := range report.Trends.DailyBreakdown {
			fmt.Printf("  %s: %d drifts (%d severe)\n", day.Date, day.Count, day.Severe)
		}
	}
}

// outputStatusJSON outputs status report in JSON format
func outputStatusJSON(report *StatusReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputStatusYAML outputs status report in YAML format
func outputStatusYAML(report *StatusReport) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(report)
}

// outputStatusTable outputs status report in table format
func outputStatusTable(report *StatusReport) {
	fmt.Printf("DriftWatch Status Report - %s\n",
		report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 80))

	// Summary section
	fmt.Printf("\nSUMMARY\n")
	fmt.Printf("Total Endpoints: %d\n", report.Summary.TotalEndpoints)
	fmt.Printf("Healthy: %d | Unhealthy: %d | Unknown: %d\n",
		report.Summary.HealthyEndpoints,
		report.Summary.UnhealthyEndpoints,
		report.Summary.UnknownEndpoints)

	if len(report.Endpoints) == 0 {
		fmt.Printf("\nNo endpoints found.\n")
		return
	}

	// Endpoints section
	fmt.Printf("\nENDPOINT STATUS\n")
	fmt.Printf("%-20s %-8s %-10s %-12s %-8s %-8s %-6s\n",
		"ID", "METHOD", "STATUS", "LAST CHECKED", "RESP TIME", "SUCCESS", "DRIFTS")
	fmt.Println(strings.Repeat("-", 85))

	for _, ep := range report.Endpoints {
		// Format last checked time
		lastChecked := "never"
		if !ep.LastChecked.IsZero() {
			if time.Since(ep.LastChecked) < 24*time.Hour {
				lastChecked = ep.LastChecked.Format("15:04:05")
			} else {
				lastChecked = ep.LastChecked.Format("01-02 15:04")
			}
		}

		// Format response time
		respTime := "N/A"
		if ep.LastResponseTime > 0 {
			respTime = fmt.Sprintf("%dms", ep.LastResponseTime)
		}

		// Format success rate
		successRate := fmt.Sprintf("%.1f%%", ep.SuccessRate)

		// Truncate long endpoint IDs
		displayID := ep.ID
		if len(displayID) > 17 {
			displayID = displayID[:14] + "..."
		}

		fmt.Printf("%-20s %-8s %-10s %-12s %-8s %-8s %-6d\n",
			displayID,
			ep.Method,
			strings.ToUpper(string(ep.Status[0]))+ep.Status[1:],
			lastChecked,
			respTime,
			successRate,
			ep.RecentDrifts)
	}
}

// Export functions

// exportDrifts exports drift data in the specified format
func exportDrifts(db storage.Storage, format, endpointID string, period time.Duration, outputFile string) error {
	// Get drifts
	filters := storage.DriftFilters{
		EndpointID: endpointID,
		StartTime:  time.Now().Add(-period),
		EndTime:    time.Now(),
	}

	drifts, err := db.GetDrifts(filters)
	if err != nil {
		return fmt.Errorf("failed to get drifts: %w", err)
	}

	// Determine output destination
	var output *os.File
	if outputFile != "" {
		// Use current working directory as allowed directory for output files
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		file, err := security.SafeCreateFile(outputFile, cwd)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Export based on format
	switch format {
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(drifts)
	case "yaml":
		encoder := yaml.NewEncoder(output)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(drifts)
	case "csv":
		return exportDriftsCSV(drifts, output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportMonitoringRuns exports monitoring run data
func exportMonitoringRuns(db storage.Storage, format, endpointID string, period time.Duration, outputFile string) error {
	// Get all endpoints if none specified
	var endpointIDs []string
	if endpointID != "" {
		endpointIDs = []string{endpointID}
	} else {
		endpoints, err := db.ListEndpoints()
		if err != nil {
			return fmt.Errorf("failed to list endpoints: %w", err)
		}
		for _, ep := range endpoints {
			endpointIDs = append(endpointIDs, ep.ID)
		}
	}

	// Collect all runs
	var allRuns []*storage.MonitoringRun
	for _, epID := range endpointIDs {
		runs, err := db.GetMonitoringHistory(epID, period)
		if err != nil {
			continue // Skip endpoints with errors
		}
		allRuns = append(allRuns, runs...)
	}

	// Determine output destination
	var output *os.File
	if outputFile != "" {
		// Use current working directory as allowed directory for output files
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		file, err := security.SafeCreateFile(outputFile, cwd)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Export based on format
	switch format {
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(allRuns)
	case "yaml":
		encoder := yaml.NewEncoder(output)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(allRuns)
	case "csv":
		return exportRunsCSV(allRuns, output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportAllData exports both drifts and monitoring runs
func exportAllData(db storage.Storage, format, endpointID string, period time.Duration, outputFile string) error {
	// Get drifts
	driftFilters := storage.DriftFilters{
		EndpointID: endpointID,
		StartTime:  time.Now().Add(-period),
		EndTime:    time.Now(),
	}

	drifts, err := db.GetDrifts(driftFilters)
	if err != nil {
		return fmt.Errorf("failed to get drifts: %w", err)
	}

	// Get monitoring runs
	var endpointIDs []string
	if endpointID != "" {
		endpointIDs = []string{endpointID}
	} else {
		endpoints, err := db.ListEndpoints()
		if err != nil {
			return fmt.Errorf("failed to list endpoints: %w", err)
		}
		for _, ep := range endpoints {
			endpointIDs = append(endpointIDs, ep.ID)
		}
	}

	var allRuns []*storage.MonitoringRun
	for _, epID := range endpointIDs {
		runs, err := db.GetMonitoringHistory(epID, period)
		if err != nil {
			continue
		}
		allRuns = append(allRuns, runs...)
	}

	// Create combined data structure
	exportData := struct {
		Drifts         []*storage.Drift         `json:"drifts" yaml:"drifts"`
		MonitoringRuns []*storage.MonitoringRun `json:"monitoring_runs" yaml:"monitoring_runs"`
		ExportedAt     time.Time                `json:"exported_at" yaml:"exported_at"`
		Period         string                   `json:"period" yaml:"period"`
	}{
		Drifts:         drifts,
		MonitoringRuns: allRuns,
		ExportedAt:     time.Now(),
		Period:         formatPeriod(period),
	}

	// Determine output destination
	var output *os.File
	if outputFile != "" {
		// Use current working directory as allowed directory for output files
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		file, err := security.SafeCreateFile(outputFile, cwd)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Export based on format
	switch format {
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(exportData)
	case "yaml":
		encoder := yaml.NewEncoder(output)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(exportData)
	case "csv":
		// For CSV, we'll export drifts and runs separately
		fmt.Fprintln(output, "# DRIFTS")
		if err := exportDriftsCSV(drifts, output); err != nil {
			return err
		}
		fmt.Fprintln(output, "\n# MONITORING RUNS")
		return exportRunsCSV(allRuns, output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportDriftsCSV exports drifts in CSV format
func exportDriftsCSV(drifts []*storage.Drift, output *os.File) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{
		"ID", "EndpointID", "DetectedAt", "DriftType", "Severity",
		"Description", "BeforeValue", "AfterValue", "FieldPath", "Acknowledged",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, drift := range drifts {
		record := []string{
			strconv.FormatInt(drift.ID, 10),
			drift.EndpointID,
			drift.DetectedAt.Format(time.RFC3339),
			drift.DriftType,
			drift.Severity,
			drift.Description,
			drift.BeforeValue,
			drift.AfterValue,
			drift.FieldPath,
			strconv.FormatBool(drift.Acknowledged),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// exportRunsCSV exports monitoring runs in CSV format
func exportRunsCSV(runs []*storage.MonitoringRun, output *os.File) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{
		"ID", "EndpointID", "Timestamp", "ResponseStatus", "ResponseTimeMs",
		"ValidationResult",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, run := range runs {
		record := []string{
			strconv.FormatInt(run.ID, 10),
			run.EndpointID,
			run.Timestamp.Format(time.RFC3339),
			strconv.Itoa(run.ResponseStatus),
			strconv.FormatInt(run.ResponseTimeMs, 10),
			run.ValidationResult,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
