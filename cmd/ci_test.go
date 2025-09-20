package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"strings"
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

func TestCICommandDirectValidation(t *testing.T) {
	// Test format validation directly
	cmd := &cobra.Command{}
	cmd.Flags().StringP("format", "f", "invalid", "output format")
	cmd.Flags().String("fail-on", "high", "minimum severity to fail on")
	cmd.Flags().Duration("timeout", 5*time.Minute, "timeout for the entire CI operation")
	cmd.Flags().Bool("no-storage", false, "run without persistent storage")
	cmd.Flags().StringSlice("endpoints", []string{}, "specific endpoints to check")
	cmd.Flags().Bool("fail-on-breaking", true, "fail if any breaking changes are detected")
	cmd.Flags().Bool("include-performance", false, "include performance changes in results")
	cmd.Flags().String("baseline-file", "", "JSON file containing baseline responses")
	cmd.Flags().String("output-file", "", "write results to file instead of stdout")

	// Set up mock configuration
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			UserAgent:  "driftwatch-test/1.0.0",
		},
		Endpoints: []config.EndpointConfig{},
	}

	// Test invalid format
	err := runCIMode(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestCICommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedExit   int
		expectedOutput string
	}{
		{
			name:           "help flag",
			args:           []string{"ci", "--help"},
			expectedExit:   0,
			expectedOutput: "Run DriftWatch in CI/CD mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock configuration for tests that need it
			if tt.name == "invalid format" {
				originalCfg := cfg
				defer func() { cfg = originalCfg }()

				cfg = &config.Config{
					Global: config.GlobalConfig{
						Timeout:    30 * time.Second,
						RetryCount: 3,
						RetryDelay: 5 * time.Second,
						UserAgent:  "driftwatch-test/1.0.0",
					},
					Endpoints: []config.EndpointConfig{},
				}
			}

			// Capture output
			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			// Set args
			rootCmd.SetArgs(tt.args)

			// Execute command
			err := rootCmd.Execute()

			output := buf.String()

			if tt.expectedExit == 0 {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			if tt.expectedOutput != "" {
				assert.Contains(t, output, tt.expectedOutput)
			}

			// Reset for next test
			rootCmd.SetArgs([]string{})
		})
	}
}

func TestPerformCICheck(t *testing.T) {
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
				ID:      "test-api",
				URL:     "https://httpbin.org/json",
				Method:  "GET",
				Enabled: true,
				Timeout: 10 * time.Second,
			},
		},
	}

	// Create in-memory storage
	db, err := storage.NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Create mock HTTP client
	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://httpbin.org/json": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"test": "data"}`),
				ResponseTime: 100 * time.Millisecond,
			},
		},
	}

	// Test CI check without baseline
	ctx := context.Background()
	result := performCICheck(ctx, cfg, db, mockClient, nil, false)
	assert.Equal(t, 1, result.EndpointsChecked)
	assert.Equal(t, 0, result.TotalChanges)
	assert.Equal(t, 0, result.BreakingChanges)
	assert.Len(t, result.Endpoints, 1)

	endpoint := result.Endpoints[0]
	assert.Equal(t, "test-api", endpoint.ID)
	assert.True(t, endpoint.Success)
	assert.Equal(t, 200, endpoint.StatusCode)
	assert.Empty(t, endpoint.Error)
}

func TestPerformCICheckWithBaseline(t *testing.T) {
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
				ID:      "test-api",
				URL:     "https://httpbin.org/json",
				Method:  "GET",
				Enabled: true,
				Timeout: 10 * time.Second,
			},
		},
	}

	// Create in-memory storage
	db, err := storage.NewInMemoryStorage()
	require.NoError(t, err)
	defer db.Close()

	// Create baseline data (different from current response)
	baselineData := map[string]*drift.Response{
		"test-api": {
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"test": "old_data"}`),
			ResponseTime: 150 * time.Millisecond,
			Timestamp:    time.Now().Add(-1 * time.Hour),
		},
	}

	// Create mock HTTP client with different response
	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://httpbin.org/json": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"test": "new_data"}`),
				ResponseTime: 100 * time.Millisecond,
			},
		},
	}

	// Test CI check with baseline
	ctx := context.Background()
	result := performCICheck(ctx, cfg, db, mockClient, baselineData, false)
	assert.Equal(t, 1, result.EndpointsChecked)
	assert.Greater(t, result.TotalChanges, 0)
	assert.Len(t, result.Endpoints, 1)

	endpoint := result.Endpoints[0]
	assert.Equal(t, "test-api", endpoint.ID)
	assert.True(t, endpoint.Success)
	assert.Greater(t, len(endpoint.Changes), 0)
}

func TestDetermineExitCode(t *testing.T) {
	tests := []struct {
		name           string
		result         *CIResult
		failOnSeverity string
		failOnBreaking bool
		expectedCode   int
	}{
		{
			name: "no changes",
			result: &CIResult{
				TotalChanges:    0,
				BreakingChanges: 0,
			},
			failOnSeverity: "high",
			failOnBreaking: true,
			expectedCode:   ExitCodeSuccess,
		},
		{
			name: "breaking changes",
			result: &CIResult{
				TotalChanges:    1,
				BreakingChanges: 1,
			},
			failOnSeverity: "high",
			failOnBreaking: true,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "critical changes",
			result: &CIResult{
				TotalChanges:    1,
				CriticalChanges: 1,
			},
			failOnSeverity: "critical",
			failOnBreaking: false,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "high changes with high threshold",
			result: &CIResult{
				TotalChanges: 1,
				HighChanges:  1,
			},
			failOnSeverity: "high",
			failOnBreaking: false,
			expectedCode:   ExitCodeBreakingChanges,
		},
		{
			name: "high changes with critical threshold",
			result: &CIResult{
				TotalChanges: 1,
				HighChanges:  1,
			},
			failOnSeverity: "critical",
			failOnBreaking: false,
			expectedCode:   ExitCodeSuccess,
		},
		{
			name: "endpoint errors",
			result: &CIResult{
				Endpoints: []CIEndpointResult{
					{Error: "network error"},
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
			assert.Equal(t, tt.expectedCode, code)
		})
	}
}

func TestConvertToJUnit(t *testing.T) {
	result := &CIResult{
		Timestamp:        time.Now(),
		Duration:         2 * time.Second,
		EndpointsChecked: 2,
		Endpoints: []CIEndpointResult{
			{
				ID:           "success-api",
				Method:       "GET",
				URL:          "https://api.example.com/success",
				Success:      true,
				StatusCode:   200,
				ResponseTime: 100 * time.Millisecond,
			},
			{
				ID:              "failing-api",
				Method:          "GET",
				URL:             "https://api.example.com/fail",
				Success:         false,
				Error:           "connection timeout",
				BreakingChanges: 1,
				Changes: []CIChange{
					{
						Type:        "field_removed",
						Path:        "$.user.id",
						Severity:    "critical",
						Breaking:    true,
						Description: "Field 'user.id' was removed",
					},
				},
			},
		},
	}

	junitSuite := convertToJUnit(result)

	assert.Equal(t, "DriftWatch CI Check", junitSuite.Name)
	assert.Equal(t, 2, junitSuite.Tests)
	assert.Equal(t, 1, junitSuite.Errors)
	assert.Equal(t, 0, junitSuite.Failures) // This endpoint has an error, not a failure
	assert.Equal(t, 2.0, junitSuite.Time)
	assert.Len(t, junitSuite.TestCases, 2)

	// Check success test case
	successCase := junitSuite.TestCases[0]
	assert.Equal(t, "endpoint_success-api", successCase.Name)
	assert.Equal(t, "driftwatch.endpoint", successCase.ClassName)
	assert.Equal(t, 0.1, successCase.Time)
	assert.Nil(t, successCase.Error)
	assert.Nil(t, successCase.Failure)

	// Check failing test case
	failCase := junitSuite.TestCases[1]
	assert.Equal(t, "endpoint_failing-api", failCase.Name)
	assert.NotNil(t, failCase.Error)
	assert.Equal(t, "connection timeout", failCase.Error.Message)
	assert.Equal(t, "EndpointError", failCase.Error.Type)
}

func TestOutputCIResults(t *testing.T) {
	result := &CIResult{
		Success:          true,
		Timestamp:        time.Now(),
		Duration:         1 * time.Second,
		EndpointsChecked: 1,
		TotalChanges:     0,
		Summary:          "✅ CI check passed",
		ExitCode:         0,
		Endpoints: []CIEndpointResult{
			{
				ID:           "test-api",
				Success:      true,
				StatusCode:   200,
				ResponseTime: 100 * time.Millisecond,
			},
		},
	}

	t.Run("JSON output", func(t *testing.T) {
		// Create temporary file
		tmpFile, err := os.CreateTemp(".", "ci-result-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Output to file
		err = outputCIResults(result, "json", tmpFile.Name())
		require.NoError(t, err)

		// Read and verify
		data, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)

		var parsed CIResult
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, result.Success, parsed.Success)
		assert.Equal(t, result.EndpointsChecked, parsed.EndpointsChecked)
		assert.Equal(t, result.ExitCode, parsed.ExitCode)
	})

	t.Run("JUnit output", func(t *testing.T) {
		// Create temporary file
		tmpFile, err := os.CreateTemp(".", "ci-result-*.xml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Output to file
		err = outputCIResults(result, "junit", tmpFile.Name())
		require.NoError(t, err)

		// Read and verify
		data, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)

		var parsed JUnitTestSuite
		err = xml.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "DriftWatch CI Check", parsed.Name)
		assert.Equal(t, 1, parsed.Tests)
		assert.Len(t, parsed.TestCases, 1)
	})

	t.Run("Summary output", func(t *testing.T) {
		// Create temporary file
		tmpFile, err := os.CreateTemp(".", "ci-result-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Output to file
		err = outputCIResults(result, "summary", tmpFile.Name())
		require.NoError(t, err)

		// Read and verify
		data, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)

		content := string(data)
		assert.Contains(t, content, "✅ CI check passed")
	})
}

func TestLoadBaselineData(t *testing.T) {
	// Create test baseline data
	baselineData := map[string]*drift.Response{
		"test-api": {
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"test": "data"}`),
			ResponseTime: 100 * time.Millisecond,
			Timestamp:    time.Now(),
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(".", "baseline-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write baseline data
	jsonData, err := json.Marshal(baselineData)
	require.NoError(t, err)

	err = os.WriteFile(tmpFile.Name(), jsonData, 0o644)
	require.NoError(t, err)

	// Load baseline data
	loaded, err := loadBaselineData(tmpFile.Name())
	require.NoError(t, err)

	assert.Len(t, loaded, 1)
	assert.Contains(t, loaded, "test-api")

	response := loaded["test-api"]
	assert.Equal(t, 200, response.StatusCode)
	assert.Equal(t, "application/json", response.Headers["Content-Type"])
	assert.Equal(t, `{"test": "data"}`, string(response.Body))
}

func TestGenerateCISummary(t *testing.T) {
	tests := []struct {
		name     string
		result   *CIResult
		expected string
	}{
		{
			name: "success",
			result: &CIResult{
				Success:          true,
				EndpointsChecked: 3,
				TotalChanges:     0,
				BreakingChanges:  0,
			},
			expected: "✅ CI check passed: 3 endpoints checked, no breaking changes detected",
		},
		{
			name: "breaking changes",
			result: &CIResult{
				Success:          false,
				EndpointsChecked: 2,
				BreakingChanges:  1,
				CriticalChanges:  1,
			},
			expected: "❌ CI check failed: 1 breaking changes, 1 critical changes",
		},
		{
			name: "endpoint errors",
			result: &CIResult{
				Success:          false,
				EndpointsChecked: 2,
				Endpoints: []CIEndpointResult{
					{Success: true},
					{Success: false, Error: "timeout"},
				},
			},
			expected: "❌ CI check failed: 1 endpoint errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := generateCISummary(tt.result)
			assert.Contains(t, summary, strings.Split(tt.expected, ":")[0])
		})
	}
}
