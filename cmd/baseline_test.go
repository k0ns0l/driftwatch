package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaselineCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "help flag",
			args:           []string{"baseline", "--help"},
			expectedOutput: "Create baseline response data",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			// Set args
			rootCmd.SetArgs(tt.args)

			// Execute command
			err := rootCmd.Execute()

			output := buf.String()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedOutput != "" {
				assert.Contains(t, output, tt.expectedOutput)
			}

			// Reset for next test
			rootCmd.SetArgs([]string{})
		})
	}
}

func TestRunBaselineCapture(t *testing.T) {
	// Create temporary output file in current directory
	tmpFile, err := os.CreateTemp(".", "baseline-test-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create test configuration
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			UserAgent:  "driftwatch-test/1.0.0",
		},
		Endpoints: []config.EndpointConfig{
			{
				ID:      "test-api-1",
				URL:     "https://httpbin.org/json",
				Method:  "GET",
				Enabled: true,
				Timeout: 10 * time.Second,
			},
			{
				ID:      "test-api-2",
				URL:     "https://httpbin.org/status/200",
				Method:  "GET",
				Enabled: true,
				Timeout: 10 * time.Second,
			},
		},
	}

	// Mock the HTTP client
	originalClient := httpClient.NewClient
	defer func() { httpClient.NewClient = originalClient }()

	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://httpbin.org/json": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"test": "data1"}`),
				ResponseTime: 100 * time.Millisecond,
			},
			"GET https://httpbin.org/status/200": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"text/plain"}},
				Body:         []byte("OK"),
				ResponseTime: 50 * time.Millisecond,
			},
		},
	}

	httpClient.NewClient = func(config httpClient.ClientConfig) httpClient.Client {
		return mockClient
	}

	// Create mock command
	cmd := &cobra.Command{}
	cmd.Flags().String("output", tmpFile.Name(), "")
	cmd.Flags().StringSlice("endpoints", []string{}, "")
	cmd.Flags().Bool("pretty", true, "")
	cmd.Flags().Duration("timeout", 30*time.Second, "")
	cmd.Flags().Bool("include-headers", true, "")
	cmd.Flags().Bool("include-body", true, "")
	cmd.Flags().Bool("overwrite", true, "")

	// Run baseline capture
	err = runBaselineCapture(cmd, []string{})
	require.NoError(t, err)

	// Verify output file exists and contains expected data
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	var baseline map[string]*drift.Response
	err = json.Unmarshal(data, &baseline)
	require.NoError(t, err)

	assert.Len(t, baseline, 2)
	assert.Contains(t, baseline, "test-api-1")
	assert.Contains(t, baseline, "test-api-2")

	// Verify first endpoint
	api1 := baseline["test-api-1"]
	assert.Equal(t, 200, api1.StatusCode)
	assert.Equal(t, "application/json", api1.Headers["Content-Type"])
	assert.Equal(t, `{"test": "data1"}`, string(api1.Body))
	assert.Equal(t, 100*time.Millisecond, api1.ResponseTime)

	// Verify second endpoint
	api2 := baseline["test-api-2"]
	assert.Equal(t, 200, api2.StatusCode)
	assert.Equal(t, "text/plain", api2.Headers["Content-Type"])
	assert.Equal(t, "OK", string(api2.Body))
	assert.Equal(t, 50*time.Millisecond, api2.ResponseTime)
}

func TestRunBaselineCaptureWithFilters(t *testing.T) {
	// Create temporary output file
	tmpFile, err := os.CreateTemp(".", "baseline-filtered-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create test configuration
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			UserAgent:  "driftwatch-test/1.0.0",
		},
		Endpoints: []config.EndpointConfig{
			{
				ID:      "api-1",
				URL:     "https://httpbin.org/json",
				Method:  "GET",
				Enabled: true,
			},
			{
				ID:      "api-2",
				URL:     "https://httpbin.org/status/200",
				Method:  "GET",
				Enabled: true,
			},
		},
	}

	// Mock the HTTP client
	originalClient := httpClient.NewClient
	defer func() { httpClient.NewClient = originalClient }()

	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://httpbin.org/json": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"filtered": "data"}`),
				ResponseTime: 75 * time.Millisecond,
			},
		},
	}

	httpClient.NewClient = func(config httpClient.ClientConfig) httpClient.Client {
		return mockClient
	}

	// Create mock command with endpoint filter
	cmd := &cobra.Command{}
	cmd.Flags().String("output", tmpFile.Name(), "")
	cmd.Flags().StringSlice("endpoints", []string{"api-1"}, "") // Only capture api-1
	cmd.Flags().Bool("pretty", false, "")
	cmd.Flags().Duration("timeout", 30*time.Second, "")
	cmd.Flags().Bool("include-headers", true, "")
	cmd.Flags().Bool("include-body", true, "")
	cmd.Flags().Bool("overwrite", true, "")

	// Run baseline capture
	err = runBaselineCapture(cmd, []string{})
	require.NoError(t, err)

	// Verify output file contains only filtered endpoint
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	var baseline map[string]*drift.Response
	err = json.Unmarshal(data, &baseline)
	require.NoError(t, err)

	assert.Len(t, baseline, 1)
	assert.Contains(t, baseline, "api-1")
	assert.NotContains(t, baseline, "api-2")

	// Verify the captured endpoint
	api1 := baseline["api-1"]
	assert.Equal(t, 200, api1.StatusCode)
	assert.Equal(t, `{"filtered": "data"}`, string(api1.Body))
}

func TestRunBaselineCaptureWithoutHeaders(t *testing.T) {
	// Create temporary output file
	tmpFile, err := os.CreateTemp(".", "baseline-no-headers-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create test configuration
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
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
			},
		},
	}

	// Mock the HTTP client
	originalClient := httpClient.NewClient
	defer func() { httpClient.NewClient = originalClient }()

	mockClient := &MockHTTPClient{
		responses: map[string]*httpClient.Response{
			"GET https://httpbin.org/json": {
				StatusCode:   200,
				Headers:      map[string][]string{"Content-Type": {"application/json"}},
				Body:         []byte(`{"no": "headers"}`),
				ResponseTime: 80 * time.Millisecond,
			},
		},
	}

	httpClient.NewClient = func(config httpClient.ClientConfig) httpClient.Client {
		return mockClient
	}

	// Create mock command without headers
	cmd := &cobra.Command{}
	cmd.Flags().String("output", tmpFile.Name(), "")
	cmd.Flags().StringSlice("endpoints", []string{}, "")
	cmd.Flags().Bool("pretty", true, "")
	cmd.Flags().Duration("timeout", 30*time.Second, "")
	cmd.Flags().Bool("include-headers", false, "") // Exclude headers
	cmd.Flags().Bool("include-body", true, "")
	cmd.Flags().Bool("overwrite", true, "")

	// Run baseline capture
	err = runBaselineCapture(cmd, []string{})
	require.NoError(t, err)

	// Verify output file
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	var baseline map[string]*drift.Response
	err = json.Unmarshal(data, &baseline)
	require.NoError(t, err)

	assert.Len(t, baseline, 1)

	api := baseline["test-api"]
	assert.Equal(t, 200, api.StatusCode)
	assert.Empty(t, api.Headers) // Headers should be empty
	assert.Equal(t, `{"no": "headers"}`, string(api.Body))
}

func TestRunBaselineValidation(t *testing.T) {
	t.Run("valid baseline file", func(t *testing.T) {
		// Create valid baseline data
		baselineData := map[string]*drift.Response{
			"test-api": {
				StatusCode:   200,
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"valid": "data"}`),
				ResponseTime: 100 * time.Millisecond,
				Timestamp:    time.Now(),
			},
		}

		// Create temporary file
		tmpFile, err := os.CreateTemp(".", "valid-baseline-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write baseline data
		jsonData, err := json.MarshalIndent(baselineData, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(tmpFile.Name(), jsonData, 0o644)
		require.NoError(t, err)

		// Create mock command
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		cmd.Flags().Bool("verbose", true, "")

		// Run validation
		err = runBaselineValidation(cmd, []string{tmpFile.Name()})
		assert.NoError(t, err)
	})

	t.Run("invalid baseline file - missing status code", func(t *testing.T) {
		// Create invalid baseline data
		baselineData := map[string]*drift.Response{
			"test-api": {
				// StatusCode: 0, // Missing status code
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"invalid": "data"}`),
				ResponseTime: 100 * time.Millisecond,
				Timestamp:    time.Now(),
			},
		}

		// Create temporary file
		tmpFile, err := os.CreateTemp(".", "invalid-baseline-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write baseline data
		jsonData, err := json.Marshal(baselineData)
		require.NoError(t, err)

		err = os.WriteFile(tmpFile.Name(), jsonData, 0o644)
		require.NoError(t, err)

		// Create mock command
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		cmd.Flags().Bool("verbose", false, "")

		// Run validation
		err = runBaselineValidation(cmd, []string{tmpFile.Name()})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "baseline validation failed")
	})

	t.Run("invalid baseline file - malformed JSON", func(t *testing.T) {
		// Create temporary file with invalid JSON
		tmpFile, err := os.CreateTemp(".", "malformed-baseline-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write malformed JSON
		err = os.WriteFile(tmpFile.Name(), []byte(`{"invalid": json}`), 0o644)
		require.NoError(t, err)

		// Create mock command
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		cmd.Flags().Bool("verbose", false, "")

		// Run validation
		err = runBaselineValidation(cmd, []string{tmpFile.Name()})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse baseline JSON")
	})

	t.Run("nonexistent baseline file", func(t *testing.T) {
		// Create mock command
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		cmd.Flags().Bool("verbose", false, "")

		// Run validation with nonexistent file
		err := runBaselineValidation(cmd, []string{"nonexistent-file.json"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("empty baseline file", func(t *testing.T) {
		// Create temporary file with empty baseline
		tmpFile, err := os.CreateTemp(".", "empty-baseline-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// Write empty baseline
		err = os.WriteFile(tmpFile.Name(), []byte(`{}`), 0o644)
		require.NoError(t, err)

		// Create mock command
		cmd := &cobra.Command{}
		cmd.Flags().String("file", "", "")
		cmd.Flags().Bool("verbose", false, "")

		// Run validation
		err = runBaselineValidation(cmd, []string{tmpFile.Name()})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "contains no endpoint data")
	})
}

func TestBaselineOverwriteProtection(t *testing.T) {
	// Create existing baseline file
	tmpFile, err := os.CreateTemp(".", "existing-baseline-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	existingData := map[string]*drift.Response{
		"existing-api": {
			StatusCode: 200,
			Timestamp:  time.Now(),
		},
	}

	jsonData, err := json.Marshal(existingData)
	require.NoError(t, err)

	err = os.WriteFile(tmpFile.Name(), jsonData, 0o644)
	require.NoError(t, err)

	// Create test configuration
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			UserAgent:  "driftwatch-test/1.0.0",
		},
		Endpoints: []config.EndpointConfig{
			{
				ID:      "new-api",
				URL:     "https://httpbin.org/json",
				Method:  "GET",
				Enabled: true,
			},
		},
	}

	// Create mock command without overwrite flag
	cmd := &cobra.Command{}
	cmd.Flags().String("output", tmpFile.Name(), "")
	cmd.Flags().StringSlice("endpoints", []string{}, "")
	cmd.Flags().Bool("pretty", false, "")
	cmd.Flags().Duration("timeout", 30*time.Second, "")
	cmd.Flags().Bool("include-headers", true, "")
	cmd.Flags().Bool("include-body", true, "")
	cmd.Flags().Bool("overwrite", false, "") // Don't overwrite

	// Run baseline capture - should fail
	err = runBaselineCapture(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Verify original file is unchanged
	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	var baseline map[string]*drift.Response
	err = json.Unmarshal(data, &baseline)
	require.NoError(t, err)

	assert.Contains(t, baseline, "existing-api")
	assert.NotContains(t, baseline, "new-api")
}

// MockHTTPClient implements the httpClient.Client interface for testing
type MockHTTPClient struct {
	responses map[string]*httpClient.Response
	errors    map[string]error
}

func (m *MockHTTPClient) Do(req *http.Request) (*httpClient.Response, error) {
	key := req.Method + " " + req.URL.String()

	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	if resp, exists := m.responses[key]; exists {
		return resp, nil
	}

	// Default response
	return &httpClient.Response{
		StatusCode:   404,
		Headers:      map[string][]string{"Content-Type": {"application/json"}},
		Body:         []byte(`{"error": "not found"}`),
		ResponseTime: 50 * time.Millisecond,
		Timestamp:    time.Now(),
		Attempt:      1,
	}, nil
}

func (m *MockHTTPClient) SetTimeout(duration time.Duration) {}

func (m *MockHTTPClient) SetRetryPolicy(policy httpClient.RetryPolicy) {}

func (m *MockHTTPClient) GetMetrics() *httpClient.Metrics {
	return &httpClient.Metrics{}
}
