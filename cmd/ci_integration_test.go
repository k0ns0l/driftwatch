package cmd

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCIIntegrationWorkflow tests the complete CI/CD workflow
func TestCIIntegrationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Step 1: Create baseline
	t.Run("create_baseline", func(t *testing.T) {
		// Create temporary baseline file
		baselineFile, err := os.CreateTemp(".", "integration-baseline-*.json")
		require.NoError(t, err)
		defer os.Remove(baselineFile.Name())
		baselineFile.Close()

		// Create test configuration
		originalCfg := cfg
		defer func() { cfg = originalCfg }()

		cfg = &config.Config{
			Global: config.GlobalConfig{
				Timeout:    30 * time.Second,
				RetryCount: 3,
				RetryDelay: 5 * time.Second,
				UserAgent:  "driftwatch-integration-test/1.0.0",
			},
			Endpoints: []config.EndpointConfig{
				{
					ID:      "httpbin-json",
					URL:     "https://httpbin.org/json",
					Method:  "GET",
					Enabled: true,
					Timeout: 10 * time.Second,
				},
				{
					ID:      "httpbin-status",
					URL:     "https://httpbin.org/status/200",
					Method:  "GET",
					Enabled: true,
					Timeout: 10 * time.Second,
				},
			},
		}

		// Mock HTTP client for consistent results
		originalClient := httpClient.NewClient
		defer func() { httpClient.NewClient = originalClient }()

		mockClient := &MockHTTPClient{
			responses: map[string]*httpClient.Response{
				"GET https://httpbin.org/json": {
					StatusCode:   200,
					Headers:      map[string][]string{"Content-Type": {"application/json"}},
					Body:         []byte(`{"slideshow": {"author": "Yours Truly", "date": "date of publication", "slides": [{"title": "Wake up to WonderWidgets!", "type": "all"}], "title": "Sample Slide Show"}}`),
					ResponseTime: 120 * time.Millisecond,
				},
				"GET https://httpbin.org/status/200": {
					StatusCode:   200,
					Headers:      map[string][]string{"Content-Type": {"text/plain"}},
					Body:         []byte(""),
					ResponseTime: 80 * time.Millisecond,
				},
			},
		}

		httpClient.NewClient = func(config httpClient.ClientConfig) httpClient.Client {
			return mockClient
		}

		// Run baseline capture
		cmd := &cobra.Command{
			Use: "baseline",
		}
		cmd.Flags().String("output", baselineFile.Name(), "")
		cmd.Flags().StringSlice("endpoints", []string{}, "")
		cmd.Flags().Bool("pretty", false, "")
		cmd.Flags().Duration("timeout", 30*time.Second, "")
		cmd.Flags().Bool("include-headers", true, "")
		cmd.Flags().Bool("include-body", true, "")
		cmd.Flags().Bool("overwrite", true, "")

		err = runBaselineCapture(cmd, []string{})
		require.NoError(t, err)

		// Verify baseline file was created and contains expected data
		data, err := os.ReadFile(baselineFile.Name())
		require.NoError(t, err)

		var baseline map[string]*drift.Response
		err = json.Unmarshal(data, &baseline)
		require.NoError(t, err)

		assert.Len(t, baseline, 2)
		assert.Contains(t, baseline, "httpbin-json")
		assert.Contains(t, baseline, "httpbin-status")

		// Store baseline file path for next test
		t.Setenv("BASELINE_FILE", "baseline.json")
	})

	// Step 2: Run CI check with no changes (should pass)
	t.Run("ci_check_no_changes", func(t *testing.T) {
		baselineFile := os.Getenv("BASELINE_FILE")
		if baselineFile == "" {
			t.Skip("Baseline file not available from previous test")
		}

		// Create test configuration (same as baseline)
		originalCfg := cfg
		defer func() { cfg = originalCfg }()

		cfg = &config.Config{
			Global: config.GlobalConfig{
				Timeout:    30 * time.Second,
				RetryCount: 3,
				RetryDelay: 5 * time.Second,
				UserAgent:  "driftwatch-integration-test/1.0.0",
			},
			Endpoints: []config.EndpointConfig{
				{
					ID:      "httpbin-json",
					URL:     "https://httpbin.org/json",
					Method:  "GET",
					Enabled: true,
					Timeout: 10 * time.Second,
				},
				{
					ID:      "httpbin-status",
					URL:     "https://httpbin.org/status/200",
					Method:  "GET",
					Enabled: true,
					Timeout: 10 * time.Second,
				},
			},
		}

		// Load baseline data
		baselineData, err := loadBaselineData(baselineFile)
		require.NoError(t, err)

		// Create in-memory storage
		db, err := storage.NewInMemoryStorage()
		require.NoError(t, err)
		defer db.Close()

		// Mock HTTP client with same responses as baseline
		mockClient := &MockHTTPClient{
			responses: map[string]*httpClient.Response{
				"GET https://httpbin.org/json": {
					StatusCode:   200,
					Headers:      map[string][]string{"Content-Type": {"application/json"}},
					Body:         []byte(`{"slideshow": {"author": "Yours Truly", "date": "date of publication", "slides": [{"title": "Wake up to WonderWidgets!", "type": "all"}], "title": "Sample Slide Show"}}`),
					ResponseTime: 120 * time.Millisecond,
				},
				"GET https://httpbin.org/status/200": {
					StatusCode:   200,
					Headers:      map[string][]string{"Content-Type": {"text/plain"}},
					Body:         []byte(""),
					ResponseTime: 80 * time.Millisecond,
				},
			},
		}

		// Perform CI check
		ctx := context.Background()
		result := performCICheck(ctx, cfg, db, mockClient, baselineData, false)

		// Verify no changes detected
		assert.Equal(t, 2, result.EndpointsChecked)
		assert.Equal(t, 0, result.TotalChanges)
		assert.Equal(t, 0, result.BreakingChanges)

		// Verify exit code
		exitCode := determineExitCode(result, "high", true)
		assert.Equal(t, ExitCodeSuccess, exitCode)
	})

	// Step 3: Run CI check with breaking changes (should fail)
	t.Run("ci_check_with_breaking_changes", func(t *testing.T) {
		baselineFile := os.Getenv("BASELINE_FILE")
		if baselineFile == "" {
			t.Skip("Baseline file not available from previous test")
		}

		// Create test configuration
		originalCfg := cfg
		defer func() { cfg = originalCfg }()

		cfg = &config.Config{
			Global: config.GlobalConfig{
				Timeout:    30 * time.Second,
				RetryCount: 3,
				RetryDelay: 5 * time.Second,
				UserAgent:  "driftwatch-integration-test/1.0.0",
			},
			Endpoints: []config.EndpointConfig{
				{
					ID:      "httpbin-json",
					URL:     "https://httpbin.org/json",
					Method:  "GET",
					Enabled: true,
					Timeout: 10 * time.Second,
				},
			},
		}

		// Load baseline data
		baselineData, err := loadBaselineData(baselineFile)
		require.NoError(t, err)

		// Create in-memory storage
		db, err := storage.NewInMemoryStorage()
		require.NoError(t, err)
		defer db.Close()

		// Mock HTTP client with different response (breaking change)
		mockClient := &MockHTTPClient{
			responses: map[string]*httpClient.Response{
				"GET https://httpbin.org/json": {
					StatusCode:   200,
					Headers:      map[string][]string{"Content-Type": {"application/json"}},
					Body:         []byte(`{"slideshow": {"author": "Different Author", "slides": [{"title": "New Title", "type": "all"}], "title": "Modified Slide Show"}}`), // Removed 'date' field
					ResponseTime: 150 * time.Millisecond,
				},
			},
		}

		// Perform CI check
		ctx := context.Background()
		result := performCICheck(ctx, cfg, db, mockClient, baselineData, false)

		// Verify changes detected
		assert.Equal(t, 1, result.EndpointsChecked)
		assert.Greater(t, result.TotalChanges, 0)

		// Check that we have endpoint results with changes
		assert.Len(t, result.Endpoints, 1)
		endpoint := result.Endpoints[0]
		assert.Equal(t, "httpbin-json", endpoint.ID)
		assert.True(t, endpoint.Success)
		assert.Greater(t, len(endpoint.Changes), 0)

		// Verify exit code indicates breaking changes
		exitCode := determineExitCode(result, "high", true)
		assert.NotEqual(t, ExitCodeSuccess, exitCode)
	})

	// Step 4: Test different output formats
	t.Run("ci_output_formats", func(t *testing.T) {
		result := &CIResult{
			Success:          false,
			Timestamp:        time.Now(),
			Duration:         2 * time.Second,
			EndpointsChecked: 1,
			TotalChanges:     2,
			BreakingChanges:  1,
			CriticalChanges:  0,
			HighChanges:      1,
			MediumChanges:    1,
			LowChanges:       0,
			Summary:          "❌ CI check failed: 1 breaking changes, 1 high severity changes",
			ExitCode:         ExitCodeBreakingChanges,
			Endpoints: []CIEndpointResult{
				{
					ID:              "test-api",
					URL:             "https://api.example.com/test",
					Method:          "GET",
					Success:         true,
					StatusCode:      200,
					ResponseTime:    100 * time.Millisecond,
					BreakingChanges: 1,
					Changes: []CIChange{
						{
							Type:        "field_removed",
							Path:        "$.user.id",
							Severity:    "high",
							Breaking:    true,
							Description: "Field 'user.id' was removed",
							OldValue:    "123",
							NewValue:    "",
						},
						{
							Type:        "field_modified",
							Path:        "$.user.name",
							Severity:    "medium",
							Breaking:    false,
							Description: "Field 'user.name' changed from 'John' to 'Jane'",
							OldValue:    "John",
							NewValue:    "Jane",
						},
					},
				},
			},
		}

		// Test JSON output
		t.Run("json_output", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(".", "ci-json-*.json")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			tmpFile.Close()

			err = outputCIResults(result, "json", tmpFile.Name())
			require.NoError(t, err)

			// Verify JSON structure
			data, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)

			var parsed CIResult
			err = json.Unmarshal(data, &parsed)
			require.NoError(t, err)

			assert.Equal(t, result.Success, parsed.Success)
			assert.Equal(t, result.TotalChanges, parsed.TotalChanges)
			assert.Equal(t, result.BreakingChanges, parsed.BreakingChanges)
			assert.Len(t, parsed.Endpoints, 1)
			assert.Len(t, parsed.Endpoints[0].Changes, 2)
		})

		// Test JUnit output
		t.Run("junit_output", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(".", "ci-junit-*.xml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			tmpFile.Close()

			err = outputCIResults(result, "junit", tmpFile.Name())
			require.NoError(t, err)

			// Verify XML structure
			data, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)

			assert.Contains(t, string(data), `<?xml version="1.0" encoding="UTF-8"?>`)
			assert.Contains(t, string(data), `<testsuite name="DriftWatch CI Check"`)
			assert.Contains(t, string(data), `tests="1"`)
			assert.Contains(t, string(data), `<testcase name="endpoint_test-api"`)
		})

		// Test summary output
		t.Run("summary_output", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(".", "ci-summary-*.txt")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			tmpFile.Close()

			err = outputCIResults(result, "summary", tmpFile.Name())
			require.NoError(t, err)

			// Verify summary content
			data, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)

			content := string(data)
			assert.Contains(t, content, "❌ CI check failed")
			assert.Contains(t, content, "breaking changes")
		})
	})

	// Cleanup
	os.Remove("baseline.json")
}

// TestCIExitCodes tests various exit code scenarios
func TestCIExitCodes(t *testing.T) {
	tests := []struct {
		name           string
		result         *CIResult
		failOnSeverity string
		failOnBreaking bool
		expectedCode   int
	}{
		{
			name: "success_no_changes",
			result: &CIResult{
				EndpointsChecked: 3,
				TotalChanges:     0,
				BreakingChanges:  0,
				Endpoints: []CIEndpointResult{
					{Success: true},
					{Success: true},
					{Success: true},
				},
			},
			failOnSeverity: "high",
			failOnBreaking: true,
			expectedCode:   ExitCodeSuccess,
		},
		{
			name: "breaking_changes_detected",
			result: &CIResult{
				EndpointsChecked: 2,
				TotalChanges:     2,
				BreakingChanges:  1,
				HighChanges:      1,
				Endpoints: []CIEndpointResult{
					{Success: true, BreakingChanges: 1},
					{Success: true},
				},
			},
			failOnSeverity: "high",
			failOnBreaking: true,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "critical_changes_fail_on_critical",
			result: &CIResult{
				EndpointsChecked: 1,
				TotalChanges:     1,
				CriticalChanges:  1,
				Endpoints: []CIEndpointResult{
					{Success: true},
				},
			},
			failOnSeverity: "critical",
			failOnBreaking: false,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "high_changes_fail_on_high",
			result: &CIResult{
				EndpointsChecked: 1,
				TotalChanges:     1,
				HighChanges:      1,
				Endpoints: []CIEndpointResult{
					{Success: true},
				},
			},
			failOnSeverity: "high",
			failOnBreaking: false,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "high_changes_fail_on_critical_should_pass",
			result: &CIResult{
				EndpointsChecked: 1,
				TotalChanges:     1,
				HighChanges:      1,
				Endpoints: []CIEndpointResult{
					{Success: true},
				},
			},
			failOnSeverity: "critical",
			failOnBreaking: false,
			expectedCode:   ExitCodeSuccess,
		},
		{
			name: "endpoint_errors",
			result: &CIResult{
				EndpointsChecked: 2,
				TotalChanges:     0,
				Endpoints: []CIEndpointResult{
					{Success: true},
					{Success: false, Error: "connection timeout"},
				},
			},
			failOnSeverity: "high",
			failOnBreaking: false,
			expectedCode:   ExitCodeGeneralError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := determineExitCode(tt.result, tt.failOnSeverity, tt.failOnBreaking)
			assert.Equal(t, tt.expectedCode, code, "Exit code mismatch for test case: %s", tt.name)
		})
	}
}

// TestCIPerformanceMode tests CI with performance change detection
func TestCIPerformanceMode(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			UserAgent:  "driftwatch-test/1.0.0",
		},
		Endpoints: []config.EndpointConfig{
			{
				ID:      "perf-api",
				URL:     "https://api.example.com/perf",
				Method:  "GET",
				Enabled: true,
				Timeout: 10 * time.Second,
			},
		},
	}

	// Create baseline with fast response time
	baselineData := map[string]*drift.Response{
		"perf-api": {
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"data": "test"}`),
			ResponseTime: 50 * time.Millisecond, // Fast baseline
			Timestamp:    time.Now().Add(-1 * time.Hour),
		},
	}

	// Create in-memory storage
	db, err := storage.NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Mock HTTP client with slower response
	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://api.example.com/perf": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"data": "test"}`),
				ResponseTime: 500 * time.Millisecond, // Much slower
			},
		},
	}

	// Test with performance monitoring enabled
	ctx := context.Background()
	result := performCICheck(ctx, cfg, db, mockClient, baselineData, true)

	assert.Equal(t, 1, result.EndpointsChecked)
	assert.Greater(t, result.TotalChanges, 0) // Should detect performance change

	endpoint := result.Endpoints[0]
	assert.Equal(t, "perf-api", endpoint.ID)
	assert.True(t, endpoint.Success)
	assert.Greater(t, len(endpoint.Changes), 0)

	// Check for performance change
	hasPerformanceChange := false
	for _, change := range endpoint.Changes {
		if change.Type == "performance_change" {
			hasPerformanceChange = true
			break
		}
	}
	assert.True(t, hasPerformanceChange, "Should detect performance change")
}
