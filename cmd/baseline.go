package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/security"
	"github.com/spf13/cobra"
)

// baselineCmd represents the baseline command
var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Create baseline response data for CI/CD integration",
	Long: `Create baseline response data by capturing current API responses.

This command captures the current state of your API endpoints and saves them
as baseline data that can be used for drift detection in CI/CD pipelines.

The baseline file contains response data (status codes, headers, and bodies)
that will be used as the reference point for detecting changes.

Examples:
  driftwatch baseline                          # Create baseline for all endpoints
  driftwatch baseline --output baseline.json  # Save to specific file
  driftwatch baseline --endpoints api1,api2   # Create baseline for specific endpoints
  driftwatch baseline --pretty                # Pretty-print JSON output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBaselineCapture(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(baselineCmd)

	// Baseline command flags
	baselineCmd.Flags().StringP("output", "o", "baseline.json", "output file for baseline data")
	baselineCmd.Flags().StringSlice("endpoints", []string{}, "specific endpoints to capture (comma-separated)")
	baselineCmd.Flags().Bool("pretty", false, "pretty-print JSON output")
	baselineCmd.Flags().Duration("timeout", 30*time.Second, "timeout for each endpoint request")
	baselineCmd.Flags().Bool("include-headers", true, "include response headers in baseline")
	baselineCmd.Flags().Bool("include-body", true, "include response body in baseline")
	baselineCmd.Flags().Bool("overwrite", false, "overwrite existing baseline file")
}

// runBaselineCapture captures baseline response data
type baselineCaptureOptions struct {
	endpointIDs    []string
	outputFile     string
	timeout        time.Duration
	prettyPrint    bool
	includeHeaders bool
	includeBody    bool
	overwrite      bool
}

func parseBaselineCaptureFlags(cmd *cobra.Command) (*baselineCaptureOptions, error) {
	opts := &baselineCaptureOptions{}
	var err error

	if opts.outputFile, err = cmd.Flags().GetString("output"); err != nil {
		return nil, fmt.Errorf("failed to get output flag: %w", err)
	}
	if opts.endpointIDs, err = cmd.Flags().GetStringSlice("endpoints"); err != nil {
		return nil, fmt.Errorf("failed to get endpoints flag: %w", err)
	}
	if opts.prettyPrint, err = cmd.Flags().GetBool("pretty"); err != nil {
		return nil, fmt.Errorf("failed to get pretty flag: %w", err)
	}
	if opts.timeout, err = cmd.Flags().GetDuration("timeout"); err != nil {
		return nil, fmt.Errorf("failed to get timeout flag: %w", err)
	}
	if opts.includeHeaders, err = cmd.Flags().GetBool("include-headers"); err != nil {
		return nil, fmt.Errorf("failed to get include-headers flag: %w", err)
	}
	if opts.includeBody, err = cmd.Flags().GetBool("include-body"); err != nil {
		return nil, fmt.Errorf("failed to get include-body flag: %w", err)
	}
	if opts.overwrite, err = cmd.Flags().GetBool("overwrite"); err != nil {
		return nil, fmt.Errorf("failed to get overwrite flag: %w", err)
	}

	return opts, nil
}

func captureEndpointBaseline(endpointConfig config.EndpointConfig, client httpClient.Client, opts *baselineCaptureOptions) (*drift.Response, error) {
	fmt.Printf("Capturing baseline for %s (%s %s)...",
		endpointConfig.ID, endpointConfig.Method, endpointConfig.URL)

	// Create HTTP request
	req, err := httpClient.NewRequest(endpointConfig.Method, endpointConfig.URL, nil, endpointConfig.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set timeout
	reqTimeout := endpointConfig.Timeout
	if reqTimeout == 0 {
		reqTimeout = opts.timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()

	// Perform request
	startTime := time.Now()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Create baseline response
	baselineResponse := &drift.Response{
		StatusCode:   resp.StatusCode,
		ResponseTime: resp.ResponseTime,
		Timestamp:    startTime,
	}

	// Include headers if requested
	if opts.includeHeaders {
		baselineResponse.Headers = convertHeaders(resp.Headers)
	}

	// Include body if requested
	if opts.includeBody {
		baselineResponse.Body = resp.Body
	}

	fmt.Printf(" OK (status: %d, time: %v)\n", resp.StatusCode, resp.ResponseTime)
	return baselineResponse, nil
}

func saveBaselineData(baselineData map[string]*drift.Response, opts *baselineCaptureOptions) error {
	var jsonData []byte
	var err error

	if opts.prettyPrint {
		jsonData, err = json.MarshalIndent(baselineData, "", "  ")
	} else {
		jsonData, err = json.Marshal(baselineData)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal baseline data: %w", err)
	}

	// Use current working directory as allowed directory for baseline files
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if err := security.SafeWriteFile(opts.outputFile, jsonData, cwd); err != nil {
		return fmt.Errorf("failed to write baseline file: %w", err)
	}

	return nil
}

func displayBaselineSummary(baselineData map[string]*drift.Response, opts *baselineCaptureOptions) {
	fmt.Printf("\n✅ Baseline data saved to %s (%d endpoints)\n", opts.outputFile, len(baselineData))

	// Display summary
	fmt.Println("\nBaseline Summary:")
	for endpointID, response := range baselineData {
		fmt.Printf("  %s: HTTP %d, %v response time",
			endpointID, response.StatusCode, response.ResponseTime)

		if opts.includeBody && len(response.Body) > 0 {
			fmt.Printf(", %d bytes body", len(response.Body))
		}

		if opts.includeHeaders && len(response.Headers) > 0 {
			fmt.Printf(", %d headers", len(response.Headers))
		}

		fmt.Println()
	}

	fmt.Printf("\nTo use this baseline in CI/CD:\n")
	fmt.Printf("  driftwatch ci --baseline-file %s\n", opts.outputFile)
}

func runBaselineCapture(cmd *cobra.Command, _ []string) error {
	// Parse flags
	opts, err := parseBaselineCaptureFlags(cmd)
	if err != nil {
		return err
	}

	// Check if output file exists and overwrite flag
	if _, err := os.Stat(opts.outputFile); err == nil && !opts.overwrite {
		return fmt.Errorf("baseline file %s already exists, use --overwrite to replace it", opts.outputFile)
	}

	// Load configuration
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Filter endpoints if specified
	if len(opts.endpointIDs) > 0 {
		if err := filterEndpoints(cfg, opts.endpointIDs); err != nil {
			return fmt.Errorf("failed to filter endpoints: %w", err)
		}
	}

	// Create HTTP client
	client := httpClient.NewClient(httpClient.ClientConfig{
		Timeout:    opts.timeout,
		RetryCount: cfg.Global.RetryCount,
		RetryDelay: cfg.Global.RetryDelay,
		UserAgent:  cfg.Global.UserAgent,
	})

	// Capture baseline data
	fmt.Printf("Capturing baseline data for %d endpoints...\n", len(cfg.Endpoints))

	baselineData := make(map[string]*drift.Response)

	for _, endpointConfig := range cfg.Endpoints {
		if !endpointConfig.Enabled {
			fmt.Printf("Skipping disabled endpoint: %s\n", endpointConfig.ID)
			continue
		}

		response, err := captureEndpointBaseline(endpointConfig, client, opts)
		if err != nil {
			fmt.Printf(" ERROR: %v\n", err)
			continue
		}

		baselineData[endpointConfig.ID] = response
	}

	if len(baselineData) == 0 {
		return fmt.Errorf("no baseline data captured")
	}

	// Save baseline data to file
	if err := saveBaselineData(baselineData, opts); err != nil {
		return err
	}

	// Display summary
	displayBaselineSummary(baselineData, opts)

	return nil
}

// validateBaselineCmd represents the validate-baseline command
var validateBaselineCmd = &cobra.Command{
	Use:   "validate-baseline",
	Short: "Validate a baseline file",
	Long: `Validate the structure and content of a baseline file.

This command checks that a baseline file is properly formatted and contains
valid response data that can be used for drift detection.

Examples:
  driftwatch validate-baseline baseline.json
  driftwatch validate-baseline --file baseline.json --verbose`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBaselineValidation(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(validateBaselineCmd)

	// Validate baseline command flags
	validateBaselineCmd.Flags().StringP("file", "f", "", "baseline file to validate")
	validateBaselineCmd.Flags().BoolP("verbose", "v", false, "verbose validation output")
}

// runBaselineValidation validates a baseline file
func runBaselineValidation(cmd *cobra.Command, args []string) error {
	baselineFile, verbose, err := parseBaselineValidationFlags(cmd, args)
	if err != nil {
		return err
	}

	baseline, err := loadBaselineFile(baselineFile)
	if err != nil {
		return err
	}

	validEndpoints, issues := validateBaselineData(baseline, verbose)

	return reportValidationResults(baseline, validEndpoints, issues, baselineFile)
}

// parseBaselineValidationFlags parses command line flags for baseline validation
func parseBaselineValidationFlags(cmd *cobra.Command, args []string) (string, bool, error) {
	var baselineFile string
	if len(args) > 0 {
		baselineFile = args[0]
	} else {
		var err error
		baselineFile, err = cmd.Flags().GetString("file")
		if err != nil || baselineFile == "" {
			return "", false, fmt.Errorf("baseline file must be specified as argument or --file flag")
		}
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return "", false, fmt.Errorf("failed to get verbose flag: %w", err)
	}

	return baselineFile, verbose, nil
}

// loadBaselineFile loads and parses a baseline file
func loadBaselineFile(baselineFile string) (map[string]*drift.Response, error) {
	// Check if file exists
	if _, err := os.Stat(baselineFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("baseline file does not exist: %s", baselineFile)
	}

	fmt.Printf("Validating baseline file: %s\n", baselineFile)

	// Use current working directory as allowed directory for baseline files
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	data, err := security.SafeReadFile(baselineFile, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var baseline map[string]*drift.Response
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to parse baseline JSON: %w", err)
	}

	if len(baseline) == 0 {
		return nil, fmt.Errorf("baseline file contains no endpoint data")
	}

	return baseline, nil
}

// validateBaselineData validates each endpoint's baseline data
func validateBaselineData(baseline map[string]*drift.Response, verbose bool) (int, []string) {
	validEndpoints := 0
	var issues []string

	for endpointID, response := range baseline {
		if verbose {
			fmt.Printf("\nValidating endpoint: %s\n", endpointID)
		}

		endpointIssues := validateEndpointResponse(endpointID, response, verbose)
		issues = append(issues, endpointIssues...)

		if len(endpointIssues) == 0 {
			validEndpoints++
		}
	}

	return validEndpoints, issues
}

// validateEndpointResponse validates a single endpoint response
func validateEndpointResponse(endpointID string, response *drift.Response, verbose bool) []string {
	var issues []string

	// Check required fields
	if response == nil {
		return []string{fmt.Sprintf("endpoint %s has null response data", endpointID)}
	}

	if response.StatusCode == 0 {
		issues = append(issues, fmt.Sprintf("endpoint %s missing status code", endpointID))
	}

	if response.Timestamp.IsZero() {
		issues = append(issues, fmt.Sprintf("endpoint %s missing timestamp", endpointID))
	}

	// Validate status code range
	if response.StatusCode < 100 || response.StatusCode >= 600 {
		issues = append(issues, fmt.Sprintf("endpoint %s has invalid status code: %d", endpointID, response.StatusCode))
	}

	// Check for reasonable response time
	if response.ResponseTime < 0 {
		issues = append(issues, fmt.Sprintf("endpoint %s has negative response time", endpointID))
	}

	// Validate JSON body if present
	validateResponseBody(response.Body, verbose)

	if verbose {
		printEndpointDetails(response)
	}

	return issues
}

// validateResponseBody validates the response body format
func validateResponseBody(body []byte, verbose bool) {
	if len(body) > 0 {
		var jsonBody interface{}
		if err := json.Unmarshal(body, &jsonBody); err != nil {
			// Not JSON, which is fine, but note it
			if verbose {
				fmt.Printf("  Body is not JSON (length: %d bytes)\n", len(body))
			}
		} else if verbose {
			fmt.Printf("  Body contains valid JSON (length: %d bytes)\n", len(body))
		}
	}
}

// printEndpointDetails prints detailed information about an endpoint response
func printEndpointDetails(response *drift.Response) {
	fmt.Printf("  Status Code: %d\n", response.StatusCode)
	fmt.Printf("  Response Time: %v\n", response.ResponseTime)
	fmt.Printf("  Timestamp: %s\n", response.Timestamp.Format(time.RFC3339))
	fmt.Printf("  Headers: %d\n", len(response.Headers))
	fmt.Printf("  Body Size: %d bytes\n", len(response.Body))
}

// reportValidationResults reports the validation results
func reportValidationResults(baseline map[string]*drift.Response, validEndpoints int, issues []string, baselineFile string) error {
	fmt.Printf("\n📊 Validation Results:\n")
	fmt.Printf("  Total endpoints: %d\n", len(baseline))
	fmt.Printf("  Valid endpoints: %d\n", validEndpoints)
	fmt.Printf("  Issues found: %d\n", len(issues))

	if len(issues) > 0 {
		fmt.Printf("\n❌ Issues found:\n")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
		return fmt.Errorf("baseline validation failed with %d issues", len(issues))
	}

	fmt.Printf("\n✅ Baseline file is valid and ready for CI/CD use\n")

	// Show usage example
	fmt.Printf("\nUsage in CI/CD:\n")
	fmt.Printf("  driftwatch ci --baseline-file %s\n", baselineFile)

	return nil
}
