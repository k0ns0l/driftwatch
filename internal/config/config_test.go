package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "DriftWatch Project", config.Project.Name)
	assert.Equal(t, "driftwatch/1.0.0", config.Global.UserAgent)
	assert.Equal(t, 30*time.Second, config.Global.Timeout)
	assert.Equal(t, 3, config.Global.RetryCount)
	assert.Equal(t, 5*time.Second, config.Global.RetryDelay)
	assert.Equal(t, 10, config.Global.MaxWorkers)
	assert.Equal(t, "./driftwatch.db", config.Global.DatabaseURL)
	assert.False(t, config.Alerting.Enabled)
	assert.Equal(t, 30, config.Reporting.RetentionDays)
	assert.Equal(t, "json", config.Reporting.ExportFormat)
	assert.False(t, config.Reporting.IncludeBody)
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
project:
  name: "Test Project"
  description: "Test description"
  version: "1.0.0"

global:
  user_agent: "test-agent/1.0"
  timeout: 45s
  retry_count: 5
  retry_delay: 10s
  max_workers: 20
  database_url: "./test.db"

endpoints:
  - id: "test-endpoint"
    url: "https://api.test.com/v1/users"
    method: "GET"
    interval: 5m
    enabled: true
    validation:
      strict_mode: true

alerting:
  enabled: true
  channels:
    - type: "slack"
      name: "test-slack"
      enabled: true
      settings:
        webhook_url: "https://hooks.slack.com/test"
  rules:
    - name: "test-rule"
      severity: ["high"]
      channels: ["test-slack"]

reporting:
  retention_days: 60
  export_format: "csv"
  include_body: true
`

	err := os.WriteFile(configFile, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Load config
	config, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Verify loaded values
	assert.Equal(t, "Test Project", config.Project.Name)
	assert.Equal(t, "test-agent/1.0", config.Global.UserAgent)
	assert.Equal(t, 45*time.Second, config.Global.Timeout)
	assert.Equal(t, 5, config.Global.RetryCount)
	assert.Equal(t, 10*time.Second, config.Global.RetryDelay)
	assert.Equal(t, 20, config.Global.MaxWorkers)
	assert.Equal(t, "./test.db", config.Global.DatabaseURL)

	require.Len(t, config.Endpoints, 1)
	assert.Equal(t, "test-endpoint", config.Endpoints[0].ID)
	assert.Equal(t, "https://api.test.com/v1/users", config.Endpoints[0].URL)
	assert.Equal(t, "GET", config.Endpoints[0].Method)
	assert.Equal(t, 5*time.Minute, config.Endpoints[0].Interval)
	assert.True(t, config.Endpoints[0].Enabled)
	assert.True(t, config.Endpoints[0].Validation.StrictMode)

	assert.True(t, config.Alerting.Enabled)
	require.Len(t, config.Alerting.Channels, 1)
	assert.Equal(t, "slack", config.Alerting.Channels[0].Type)
	assert.Equal(t, "test-slack", config.Alerting.Channels[0].Name)

	assert.Equal(t, 60, config.Reporting.RetentionDays)
	assert.Equal(t, "csv", config.Reporting.ExportFormat)
	assert.True(t, config.Reporting.IncludeBody)
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	// Try to load non-existent config file (should use default name)
	config, err := LoadConfig("")

	// Should not error and return default config
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Should have default values
	assert.Equal(t, "DriftWatch Project", config.Project.Name)
	assert.Equal(t, "driftwatch/1.0.0", config.Global.UserAgent)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create temporary config file with invalid YAML
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-config.yaml")

	invalidContent := `
project:
  name: "Test Project"
  invalid_yaml: [
`

	err := os.WriteFile(configFile, []byte(invalidContent), 0o644)
	require.NoError(t, err)

	// Try to load invalid config
	_, err = LoadConfig(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestSubstituteEnvVars(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_TOKEN", "secret-token")
	os.Setenv("TEST_WEBHOOK", "https://hooks.slack.com/test")
	defer func() {
		os.Unsetenv("TEST_TOKEN")
		os.Unsetenv("TEST_WEBHOOK")
	}()

	config := &Config{
		Endpoints: []EndpointConfig{
			{
				Headers: map[string]string{
					"Authorization": "Bearer ${TEST_TOKEN}",
					"X-Custom":      "static-value",
				},
			},
		},
		Alerting: AlertingConfig{
			Channels: []AlertChannelConfig{
				{
					Settings: map[string]interface{}{
						"webhook_url": "${TEST_WEBHOOK}",
						"channel":     "#alerts",
					},
				},
			},
		},
	}

	substituteEnvVars(config)

	// Check substitution in endpoint headers
	assert.Equal(t, "Bearer secret-token", config.Endpoints[0].Headers["Authorization"])
	assert.Equal(t, "static-value", config.Endpoints[0].Headers["X-Custom"])

	// Check substitution in alert settings
	assert.Equal(t, "https://hooks.slack.com/test", config.Alerting.Channels[0].Settings["webhook_url"])
	assert.Equal(t, "#alerts", config.Alerting.Channels[0].Settings["channel"])
}

func TestSubstituteEnvVars_MissingVar(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointConfig{
			{
				Headers: map[string]string{
					"Authorization": "Bearer ${MISSING_TOKEN}",
				},
			},
		},
	}

	substituteEnvVars(config)

	// Should leave original value if env var not found
	assert.Equal(t, "Bearer ${MISSING_TOKEN}", config.Endpoints[0].Headers["Authorization"])
}

func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "save-test.yaml")

	config := DefaultConfig()
	config.Project.Name = "Save Test Project"

	err := SaveConfig(config, configFile)
	require.NoError(t, err)

	// Verify file was created
	assert.True(t, ConfigExists(configFile))

	// Load and verify content
	loadedConfig, err := LoadConfig(configFile)
	require.NoError(t, err)
	assert.Equal(t, "Save Test Project", loadedConfig.Project.Name)
}

func TestCreateDefaultConfigFile(t *testing.T) {
	// Set test environment variables to make validation pass
	os.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/test")
	defer os.Unsetenv("SLACK_WEBHOOK_URL")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "default-config.yaml")

	err := CreateDefaultConfigFile(configFile)
	require.NoError(t, err)

	// Verify file was created
	assert.True(t, ConfigExists(configFile))

	// Load and verify it has example content
	config, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Should have example endpoint
	require.Len(t, config.Endpoints, 1)
	assert.Equal(t, "example-api", config.Endpoints[0].ID)
	assert.Equal(t, "https://api.example.com/v1/users", config.Endpoints[0].URL)

	// Should have example alerting config
	require.Len(t, config.Alerting.Channels, 1)
	assert.Equal(t, "slack", config.Alerting.Channels[0].Type)
	assert.Equal(t, "dev-alerts", config.Alerting.Channels[0].Name)
}

func TestConfigEndpointManagement(t *testing.T) {
	config := DefaultConfig()

	// Test adding endpoint
	endpoint := EndpointConfig{
		ID:       "test-endpoint",
		URL:      "https://api.test.com/v1/users",
		Method:   "GET",
		Interval: 5 * time.Minute,
		Enabled:  true,
	}

	err := config.AddEndpoint(endpoint)
	require.NoError(t, err)
	assert.Len(t, config.Endpoints, 1)

	// Test duplicate ID
	err = config.AddEndpoint(endpoint)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test getting endpoint
	retrieved, err := config.GetEndpoint("test-endpoint")
	require.NoError(t, err)
	assert.Equal(t, "test-endpoint", retrieved.ID)

	// Test getting non-existent endpoint
	_, err = config.GetEndpoint("non-existent")
	assert.Error(t, err)

	// Test updating endpoint
	updated := endpoint
	updated.URL = "https://api.test.com/v2/users"
	err = config.UpdateEndpoint("test-endpoint", updated)
	require.NoError(t, err)

	retrieved, _ = config.GetEndpoint("test-endpoint")
	assert.Equal(t, "https://api.test.com/v2/users", retrieved.URL)

	// Test removing endpoint
	err = config.RemoveEndpoint("test-endpoint")
	require.NoError(t, err)
	assert.Len(t, config.Endpoints, 0)

	// Test removing non-existent endpoint
	err = config.RemoveEndpoint("non-existent")
	assert.Error(t, err)
}

func TestGetEnabledEndpoints(t *testing.T) {
	config := DefaultConfig()

	// Add enabled and disabled endpoints
	config.Endpoints = []EndpointConfig{
		{ID: "enabled-1", Enabled: true},
		{ID: "disabled-1", Enabled: false},
		{ID: "enabled-2", Enabled: true},
	}

	enabled := config.GetEnabledEndpoints()
	assert.Len(t, enabled, 2)
	assert.Equal(t, "enabled-1", enabled[0].ID)
	assert.Equal(t, "enabled-2", enabled[1].ID)
}

func TestGetConfigFilePath(t *testing.T) {
	// Test with provided config file
	path := GetConfigFilePath("custom-config.yaml")
	assert.Equal(t, "custom-config.yaml", path)

	// Test with empty config file (should return default)
	path = GetConfigFilePath("")
	assert.Equal(t, ".driftwatch.yaml", path)
}
