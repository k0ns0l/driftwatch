package cmd

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/monitor"
	"github.com/k0ns0l/driftwatch/internal/security"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
)

// CI/CD exit codes
const (
	ExitCodeSuccess         = 0
	ExitCodeGeneralError    = 1
	ExitCodeBreakingChanges = 2
	ExitCodeConfigError     = 3
	ExitCodeNetworkError    = 4
	ExitCodeValidationError = 5
)

// ciCmd represents the ci command for CI/CD integration
var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Run DriftWatch in CI/CD mode",
	Long: `Run DriftWatch in CI/CD mode with machine-readable output and appropriate exit codes.

This command is designed for integration with CI/CD pipelines. It performs a single
check of all endpoints and exits with specific codes based on the results:

Exit Codes:
  0 - Success (no breaking changes detected)
  1 - General error (configuration, network, etc.)
  2 - Breaking changes detected
  3 - Configuration error
  4 - Network error
  5 - Validation error

Examples:
  driftwatch ci                        # Run CI check with default settings
  driftwatch ci --format json         # Output results in JSON format
  driftwatch ci --format junit        # Output results in JUnit XML format
  driftwatch ci --fail-on high        # Fail on high severity changes or above
  driftwatch ci --timeout 60s         # Set timeout for the entire operation
  driftwatch ci --no-storage          # Run without persistent storage
  driftwatch ci --endpoints api1,api2 # Check specific endpoints only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCIMode(cmd, args)
	},
}

// CIResult represents the result of a CI/CD run
type CIResult struct {
	Endpoints        []CIEndpointResult `json:"endpoints"`
	Summary          string             `json:"summary"`
	Timestamp        time.Time          `json:"timestamp"`
	Duration         time.Duration      `json:"duration"`
	EndpointsChecked int                `json:"endpoints_checked"`
	TotalChanges     int                `json:"total_changes"`
	BreakingChanges  int                `json:"breaking_changes"`
	CriticalChanges  int                `json:"critical_changes"`
	HighChanges      int                `json:"high_changes"`
	MediumChanges    int                `json:"medium_changes"`
	LowChanges       int                `json:"low_changes"`
	ExitCode         int                `json:"exit_code"`
	Success          bool               `json:"success"`
}

// CIEndpointResult represents the result for a single endpoint
type CIEndpointResult struct {
	Changes          []CIChange                `json:"changes,omitempty"`
	ValidationErrors []monitor.ValidationError `json:"validation_errors,omitempty"`
	ID               string                    `json:"id"`
	URL              string                    `json:"url"`
	Method           string                    `json:"method"`
	Error            string                    `json:"error,omitempty"`
	ResponseTime     time.Duration             `json:"response_time,omitempty"`
	StatusCode       int                       `json:"status_code,omitempty"`
	BreakingChanges  int                       `json:"breaking_changes"`
	Success          bool                      `json:"success"`
}

// CIChange represents a detected change in CI format
type CIChange struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
	Breaking    bool   `json:"breaking"`
}

// JUnitTestSuite represents a JUnit XML test suite
type JUnitTestSuite struct {
	TestCases []JUnitTestCase `xml:"testcase"`
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Timestamp string          `xml:"timestamp,attr"`
	Time      float64         `xml:"time,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
}

// JUnitTestCase represents a JUnit XML test case
type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

// JUnitFailure represents a JUnit XML failure
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// JUnitError represents a JUnit XML error
type JUnitError struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

func init() {
	rootCmd.AddCommand(ciCmd)

	// CI command flags
	ciCmd.Flags().StringP("format", "f", "json", "output format (json, junit, summary)")
	ciCmd.Flags().String("fail-on", "high", "minimum severity to fail on (low, medium, high, critical)")
	ciCmd.Flags().Duration("timeout", 5*time.Minute, "timeout for the entire CI operation")
	ciCmd.Flags().Bool("no-storage", false, "run without persistent storage (in-memory only)")
	ciCmd.Flags().StringSlice("endpoints", []string{}, "specific endpoints to check (comma-separated)")
	ciCmd.Flags().Bool("fail-on-breaking", true, "fail if any breaking changes are detected")
	ciCmd.Flags().Bool("include-performance", false, "include performance changes in results")
	ciCmd.Flags().String("baseline-file", "", "JSON file containing baseline responses for comparison")
	ciCmd.Flags().String("output-file", "", "write results to file instead of stdout")
}

// runCIMode executes the CI/CD mode
func runCIMode(cmd *cobra.Command, _ []string) error {
	startTime := time.Now()

	ciOptions, err := parseCIFlags(cmd)
	if err != nil {
		return err
	}

	if err := validateCIOptions(ciOptions); err != nil {
		return err
	}

	cfg, ctx, db, client, err := initializeCIEnvironment(ciOptions)
	if err != nil {
		return err
	}
	defer db.Close()

	baselineData, err := loadCIBaseline(ciOptions.BaselineFile)
	if err != nil {
		exitWithCode(ExitCodeConfigError, fmt.Sprintf("failed to load baseline data: %v", err))
		return nil
	}

	if err := applyCIFilters(cfg, ciOptions.EndpointIDs); err != nil {
		exitWithCode(ExitCodeConfigError, fmt.Sprintf("failed to filter endpoints: %v", err))
		return nil
	}

	result := performCICheck(ctx, cfg, db, client, baselineData, ciOptions.IncludePerformance)

	finalizeCIResult(result, startTime, ciOptions)

	if err := outputCIResults(result, ciOptions.OutputFormat, ciOptions.OutputFile); err != nil {
		exitWithCode(ExitCodeGeneralError, fmt.Sprintf("failed to output results: %v", err))
		return nil
	}

	os.Exit(result.ExitCode)
	return nil
}

// CIOptions holds all CI command options
type CIOptions struct {
	OutputFormat       string
	FailOnSeverity     string
	BaselineFile       string
	OutputFile         string
	Timeout            time.Duration
	NoStorage          bool
	FailOnBreaking     bool
	IncludePerformance bool
	EndpointIDs        []string
}

// parseCIFlags parses all CI command flags
func parseCIFlags(cmd *cobra.Command) (*CIOptions, error) {
	options := &CIOptions{}
	var err error

	if options.OutputFormat, err = cmd.Flags().GetString("format"); err != nil {
		return nil, fmt.Errorf("failed to get format flag: %w", err)
	}
	if options.FailOnSeverity, err = cmd.Flags().GetString("fail-on"); err != nil {
		return nil, fmt.Errorf("failed to get fail-on flag: %w", err)
	}
	if options.Timeout, err = cmd.Flags().GetDuration("timeout"); err != nil {
		return nil, fmt.Errorf("failed to get timeout flag: %w", err)
	}
	if options.NoStorage, err = cmd.Flags().GetBool("no-storage"); err != nil {
		return nil, fmt.Errorf("failed to get no-storage flag: %w", err)
	}
	if options.EndpointIDs, err = cmd.Flags().GetStringSlice("endpoints"); err != nil {
		return nil, fmt.Errorf("failed to get endpoints flag: %w", err)
	}
	if options.FailOnBreaking, err = cmd.Flags().GetBool("fail-on-breaking"); err != nil {
		return nil, fmt.Errorf("failed to get fail-on-breaking flag: %w", err)
	}
	if options.IncludePerformance, err = cmd.Flags().GetBool("include-performance"); err != nil {
		return nil, fmt.Errorf("failed to get include-performance flag: %w", err)
	}
	if options.BaselineFile, err = cmd.Flags().GetString("baseline-file"); err != nil {
		return nil, fmt.Errorf("failed to get baseline-file flag: %w", err)
	}
	if options.OutputFile, err = cmd.Flags().GetString("output-file"); err != nil {
		return nil, fmt.Errorf("failed to get output-file flag: %w", err)
	}

	return options, nil
}

// validateCIOptions validates CI command options
func validateCIOptions(options *CIOptions) error {
	validFormats := []string{"json", "junit", "summary"}
	for _, validFormat := range validFormats {
		if strings.ToLower(options.OutputFormat) == validFormat {
			return nil
		}
	}
	return fmt.Errorf("unsupported output format: %s", options.OutputFormat)
}

// initializeCIEnvironment sets up the CI environment
func initializeCIEnvironment(options *CIOptions) (*config.Config, context.Context, storage.Storage, httpClient.Client, error) {
	cfg := GetConfig()
	if cfg == nil {
		exitWithCode(ExitCodeConfigError, "configuration not loaded")
		return nil, nil, nil, nil, fmt.Errorf("configuration not loaded")
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	var db storage.Storage
	var err error

	if options.NoStorage {
		db, err = storage.NewInMemoryStorage()
	} else {
		db, err = storage.NewStorage(cfg.Global.DatabaseURL)
	}

	if err != nil {
		exitWithCode(ExitCodeGeneralError, fmt.Sprintf("failed to initialize storage: %v", err))
		return nil, nil, nil, nil, err
	}

	client := httpClient.NewClient(httpClient.ClientConfig{
		Timeout:    cfg.Global.Timeout,
		RetryCount: cfg.Global.RetryCount,
		RetryDelay: cfg.Global.RetryDelay,
		UserAgent:  cfg.Global.UserAgent,
	})

	return cfg, ctx, db, client, nil
}

// loadCIBaseline loads baseline data if provided
func loadCIBaseline(baselineFile string) (map[string]*drift.Response, error) {
	if baselineFile == "" {
		return nil, nil
	}
	return loadBaselineData(baselineFile)
}

// applyCIFilters applies endpoint filters if specified
func applyCIFilters(cfg *config.Config, endpointIDs []string) error {
	if len(endpointIDs) > 0 {
		return filterEndpoints(cfg, endpointIDs)
	}
	return nil
}

// finalizeCIResult finalizes the CI result with timing and exit code
func finalizeCIResult(result *CIResult, startTime time.Time, options *CIOptions) {
	result.Duration = time.Since(startTime)
	result.Timestamp = startTime

	exitCode := determineExitCode(result, options.FailOnSeverity, options.FailOnBreaking)
	result.ExitCode = exitCode
	result.Success = exitCode == ExitCodeSuccess
	result.Summary = generateCISummary(result)
}

// performCICheck performs the actual CI check
func performCICheck(ctx context.Context, cfg *config.Config, db storage.Storage, client httpClient.Client, baselineData map[string]*drift.Response, includePerformance bool) *CIResult {
	result := &CIResult{
		Endpoints: make([]CIEndpointResult, 0, len(cfg.Endpoints)),
	}

	diffEngine := drift.NewDiffEngine()

	for _, endpointConfig := range cfg.Endpoints {
		if !endpointConfig.Enabled {
			continue
		}

		endpointResult := checkSingleEndpoint(ctx, cfg, db, client, diffEngine, endpointConfig, baselineData, includePerformance)
		result.Endpoints = append(result.Endpoints, endpointResult)
	}

	calculateCITotals(result)
	return result
}

// checkSingleEndpoint performs CI check for a single endpoint
func checkSingleEndpoint(ctx context.Context, cfg *config.Config, db storage.Storage, client httpClient.Client, diffEngine drift.DiffEngine, endpointConfig config.EndpointConfig, baselineData map[string]*drift.Response, includePerformance bool) CIEndpointResult {
	endpointResult := CIEndpointResult{
		ID:     endpointConfig.ID,
		URL:    endpointConfig.URL,
		Method: endpointConfig.Method,
	}

	currentResponse, err := performEndpointRequest(ctx, cfg, client, endpointConfig)
	if err != nil {
		endpointResult.Error = err.Error()
		return endpointResult
	}

	endpointResult.Success = true
	endpointResult.StatusCode = currentResponse.StatusCode
	endpointResult.ResponseTime = currentResponse.ResponseTime

	performDriftComparison(&endpointResult, diffEngine, db, endpointConfig, currentResponse, baselineData, includePerformance)
	return endpointResult
}

// performEndpointRequest executes HTTP request for an endpoint
func performEndpointRequest(ctx context.Context, cfg *config.Config, client httpClient.Client, endpointConfig config.EndpointConfig) (*drift.Response, error) {
	req, err := httpClient.NewRequest(endpointConfig.Method, endpointConfig.URL, nil, endpointConfig.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	timeout := endpointConfig.Timeout
	if timeout == 0 {
		timeout = cfg.Global.Timeout
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()
	resp, err := client.Do(req.WithContext(reqCtx))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	return &drift.Response{
		StatusCode:   resp.StatusCode,
		Headers:      convertHeaders(resp.Headers),
		Body:         resp.Body,
		ResponseTime: resp.ResponseTime,
		Timestamp:    startTime,
	}, nil
}

// performDriftComparison compares current response with baseline or previous response
func performDriftComparison(endpointResult *CIEndpointResult, diffEngine drift.DiffEngine, db storage.Storage, endpointConfig config.EndpointConfig, currentResponse *drift.Response, baselineData map[string]*drift.Response, includePerformance bool) {
	var baseline *drift.Response

	if baselineData != nil {
		if baselineResp, exists := baselineData[endpointConfig.ID]; exists {
			baseline = baselineResp
		}
	} else {
		baseline = getBaselineFromStorage(db, endpointConfig.ID)
	}

	if baseline != nil {
		compareDriftResults(endpointResult, diffEngine, baseline, currentResponse, includePerformance)
	}
}

// getBaselineFromStorage retrieves baseline response from storage
func getBaselineFromStorage(db storage.Storage, endpointID string) *drift.Response {
	previousRuns, err := db.GetMonitoringHistory(endpointID, 24*time.Hour)
	if err != nil || len(previousRuns) == 0 {
		return nil
	}

	lastRun := previousRuns[0]
	return &drift.Response{
		StatusCode:   lastRun.ResponseStatus,
		Headers:      lastRun.ResponseHeaders,
		Body:         []byte(lastRun.ResponseBody),
		ResponseTime: time.Duration(lastRun.ResponseTimeMs) * time.Millisecond,
		Timestamp:    lastRun.Timestamp,
	}
}

// compareDriftResults performs drift comparison and updates endpoint result
func compareDriftResults(endpointResult *CIEndpointResult, diffEngine drift.DiffEngine, baseline, current *drift.Response, includePerformance bool) {
	diffResult, err := diffEngine.CompareResponses(baseline, current)
	if err != nil {
		endpointResult.Error = fmt.Sprintf("drift comparison failed: %v", err)
		return
	}

	if diffResult.HasChanges {
		endpointResult.Changes = convertDriftToCIChanges(diffResult, includePerformance)
		endpointResult.BreakingChanges = len(diffResult.BreakingChanges)
	}
}

// calculateCITotals calculates total statistics for CI result
func calculateCITotals(result *CIResult) {
	result.EndpointsChecked = len(result.Endpoints)
	for _, ep := range result.Endpoints {
		for _, change := range ep.Changes {
			result.TotalChanges++
			if change.Breaking {
				result.BreakingChanges++
			}
			switch change.Severity {
			case "critical":
				result.CriticalChanges++
			case "high":
				result.HighChanges++
			case "medium":
				result.MediumChanges++
			case "low":
				result.LowChanges++
			}
		}
	}
}

// convertDriftToCIChanges converts drift results to CI change format
func convertDriftToCIChanges(diffResult *drift.DiffResult, includePerformance bool) []CIChange {
	var changes []CIChange

	// Convert structural changes
	for _, change := range diffResult.StructuralChanges {
		ciChange := CIChange{
			Type:        string(change.Type),
			Path:        change.Path,
			Severity:    string(change.Severity),
			Breaking:    change.Breaking,
			Description: change.Description,
		}

		if change.OldValue != nil {
			ciChange.OldValue = fmt.Sprintf("%v", change.OldValue)
		}
		if change.NewValue != nil {
			ciChange.NewValue = fmt.Sprintf("%v", change.NewValue)
		}

		changes = append(changes, ciChange)
	}

	// Convert data changes
	for _, change := range diffResult.DataChanges {
		ciChange := CIChange{
			Type:        string(change.ChangeType),
			Path:        change.Path,
			Severity:    string(change.Severity),
			Breaking:    false, // Data changes are typically not breaking
			Description: change.Description,
			OldValue:    fmt.Sprintf("%v", change.OldValue),
			NewValue:    fmt.Sprintf("%v", change.NewValue),
		}

		changes = append(changes, ciChange)
	}

	// Convert performance changes if requested
	if includePerformance && diffResult.PerformanceChanges != nil {
		ciChange := CIChange{
			Type:        "performance_change",
			Path:        "$.response_time",
			Severity:    string(diffResult.PerformanceChanges.Severity),
			Breaking:    false,
			Description: diffResult.PerformanceChanges.Description,
		}

		changes = append(changes, ciChange)
	}

	return changes
}

// determineExitCode determines the appropriate exit code based on results
func determineExitCode(result *CIResult, failOnSeverity string, failOnBreaking bool) int {
	if failOnBreaking && result.BreakingChanges > 0 {
		return ExitCodeBreakingChanges
	}

	if hasEndpointErrors(result) {
		return ExitCodeGeneralError
	}

	return checkSeverityThreshold(result, failOnSeverity)
}

// hasEndpointErrors checks if any endpoints have errors
func hasEndpointErrors(result *CIResult) bool {
	for _, ep := range result.Endpoints {
		if ep.Error != "" {
			return true
		}
	}
	return false
}

// checkSeverityThreshold checks if changes exceed the severity threshold
func checkSeverityThreshold(result *CIResult, failOnSeverity string) int {
	severityCheckers := map[string]func(*CIResult) bool{
		"critical": func(r *CIResult) bool { return r.CriticalChanges > 0 },
		"high":     func(r *CIResult) bool { return r.CriticalChanges > 0 || r.HighChanges > 0 },
		"medium":   func(r *CIResult) bool { return r.CriticalChanges > 0 || r.HighChanges > 0 || r.MediumChanges > 0 },
		"low":      func(r *CIResult) bool { return r.TotalChanges > 0 },
	}

	if checker, exists := severityCheckers[strings.ToLower(failOnSeverity)]; exists && checker(result) {
		return ExitCodeBreakingChanges
	}

	return ExitCodeSuccess
}

// generateCISummary generates a human-readable summary
func generateCISummary(result *CIResult) string {
	if result.Success {
		return fmt.Sprintf("✅ CI check passed: %d endpoints checked, no breaking changes detected", result.EndpointsChecked)
	}

	var issues []string
	if result.BreakingChanges > 0 {
		issues = append(issues, fmt.Sprintf("%d breaking changes", result.BreakingChanges))
	}
	if result.CriticalChanges > 0 {
		issues = append(issues, fmt.Sprintf("%d critical changes", result.CriticalChanges))
	}
	if result.HighChanges > 0 {
		issues = append(issues, fmt.Sprintf("%d high severity changes", result.HighChanges))
	}

	errorCount := 0
	for _, ep := range result.Endpoints {
		if ep.Error != "" {
			errorCount++
		}
	}
	if errorCount > 0 {
		issues = append(issues, fmt.Sprintf("%d endpoint errors", errorCount))
	}

	return fmt.Sprintf("❌ CI check failed: %s", strings.Join(issues, ", "))
}

// outputCIResults outputs the CI results in the specified format
func outputCIResults(result *CIResult, format, outputFile string) error {
	var output []byte
	var err error

	switch strings.ToLower(format) {
	case "json":
		output, err = json.MarshalIndent(result, "", "  ")
	case "junit":
		junitSuite := convertToJUnit(result)
		output, err = xml.MarshalIndent(junitSuite, "", "  ")
		if err == nil {
			output = append([]byte(xml.Header), output...)
		}
	case "summary":
		output = []byte(result.Summary + "\n")
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	// Write to file or stdout
	if outputFile != "" {
		// Use current working directory as allowed directory for CI output files
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		return security.SafeWriteFile(outputFile, output, cwd)
	}

	_, err = os.Stdout.Write(output)
	return err
}

// convertToJUnit converts CI results to JUnit XML format
func convertToJUnit(result *CIResult) *JUnitTestSuite {
	suite := &JUnitTestSuite{
		Name:      "DriftWatch CI Check",
		Tests:     result.EndpointsChecked,
		Time:      result.Duration.Seconds(),
		Timestamp: result.Timestamp.Format(time.RFC3339),
		TestCases: make([]JUnitTestCase, 0, result.EndpointsChecked),
	}

	for _, ep := range result.Endpoints {
		testCase := JUnitTestCase{
			Name:      fmt.Sprintf("endpoint_%s", ep.ID),
			ClassName: "driftwatch.endpoint",
			Time:      ep.ResponseTime.Seconds(),
		}

		if ep.Error != "" {
			suite.Errors++
			testCase.Error = &JUnitError{
				Message: ep.Error,
				Type:    "EndpointError",
				Content: fmt.Sprintf("Endpoint %s (%s %s) failed: %s", ep.ID, ep.Method, ep.URL, ep.Error),
			}
		} else if ep.BreakingChanges > 0 {
			suite.Failures++
			testCase.Failure = &JUnitFailure{
				Message: fmt.Sprintf("%d breaking changes detected", ep.BreakingChanges),
				Type:    "BreakingChanges",
				Content: formatChangesForJUnit(ep.Changes),
			}
		}

		// Add system output with endpoint details
		systemOut := fmt.Sprintf("Endpoint: %s\nURL: %s %s\nStatus: %d\nResponse Time: %v\n",
			ep.ID, ep.Method, ep.URL, ep.StatusCode, ep.ResponseTime)

		if len(ep.Changes) > 0 {
			systemOut += fmt.Sprintf("Changes detected: %d\n", len(ep.Changes))
			for _, change := range ep.Changes {
				systemOut += fmt.Sprintf("  - %s: %s (%s)\n", change.Type, change.Description, change.Severity)
			}
		}

		testCase.SystemOut = systemOut
		suite.TestCases = append(suite.TestCases, testCase)
	}

	return suite
}

// formatChangesForJUnit formats changes for JUnit XML output
func formatChangesForJUnit(changes []CIChange) string {
	if len(changes) == 0 {
		return "No changes detected"
	}

	var lines []string
	for _, change := range changes {
		line := fmt.Sprintf("%s at %s: %s (severity: %s)", change.Type, change.Path, change.Description, change.Severity)
		if change.Breaking {
			line += " [BREAKING]"
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// loadBaselineData loads baseline response data from a JSON file
func loadBaselineData(filename string) (map[string]*drift.Response, error) {
	// Use current working directory as allowed directory for baseline files
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	data, err := security.SafeReadFile(filename, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var baseline map[string]*drift.Response
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to parse baseline JSON: %w", err)
	}

	return baseline, nil
}

// convertHeaders converts http.Header to map[string]string
func convertHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// exitWithCode prints an error message and exits with the specified code
func exitWithCode(code int, message string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	os.Exit(code)
}
