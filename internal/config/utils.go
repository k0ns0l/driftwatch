package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveConfig saves the configuration to a YAML file
func SaveConfig(config *Config, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// CreateDefaultConfigFile creates a default configuration file
func CreateDefaultConfigFile(filename string) error {
	config := DefaultConfig()

	// Add example endpoint
	config.Endpoints = []EndpointConfig{
		{
			ID:       "example-api",
			URL:      "https://api.example.com/v1/users",
			Method:   "GET",
			Interval: 300000000000, // 5 minutes in nanoseconds
			Headers: map[string]string{
				"Authorization": "${API_TOKEN}",
				"User-Agent":    "driftwatch/1.0.0",
			},
			Validation: ValidationConfig{
				StrictMode:   false,
				IgnoreFields: []string{"timestamp", "request_id"},
			},
			Enabled: true,
		},
	}

	// Add example alerting configuration
	config.Alerting = AlertingConfig{
		Enabled: false,
		Channels: []AlertChannelConfig{
			{
				Type:    "slack",
				Name:    "dev-alerts",
				Enabled: false,
				Settings: map[string]interface{}{
					"webhook_url": "${SLACK_WEBHOOK_URL}",
					"channel":     "#api-alerts",
				},
			},
		},
		Rules: []AlertRuleConfig{
			{
				Name:     "critical-changes",
				Severity: []string{"high", "critical"},
				Channels: []string{"dev-alerts"},
			},
		},
	}

	return SaveConfig(config, filename)
}

// GetConfigFilePath returns the path to the configuration file
func GetConfigFilePath(configFile string) string {
	if configFile != "" {
		return configFile
	}
	return ".driftwatch.yaml"
}

// ConfigExists checks if a configuration file exists
func ConfigExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// AddEndpoint adds a new endpoint to the configuration
func (c *Config) AddEndpoint(endpoint EndpointConfig) error {
	// Check for duplicate ID
	for _, existing := range c.Endpoints {
		if existing.ID == endpoint.ID {
			return fmt.Errorf("endpoint with ID '%s' already exists", endpoint.ID)
		}
	}

	// Validate the endpoint
	if err := validateEndpoint(&endpoint, "endpoint"); err != nil {
		return fmt.Errorf("invalid endpoint configuration: %w", err)
	}

	c.Endpoints = append(c.Endpoints, endpoint)
	return nil
}

// RemoveEndpoint removes an endpoint from the configuration
func (c *Config) RemoveEndpoint(id string) error {
	for i, endpoint := range c.Endpoints {
		if endpoint.ID == id {
			c.Endpoints = append(c.Endpoints[:i], c.Endpoints[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("endpoint with ID '%s' not found", id)
}

// GetEndpoint retrieves an endpoint by ID
func (c *Config) GetEndpoint(id string) (*EndpointConfig, error) {
	for _, endpoint := range c.Endpoints {
		if endpoint.ID == id {
			return &endpoint, nil
		}
	}
	return nil, fmt.Errorf("endpoint with ID '%s' not found", id)
}

// UpdateEndpoint updates an existing endpoint
func (c *Config) UpdateEndpoint(id string, updated EndpointConfig) error {
	for i, endpoint := range c.Endpoints {
		if endpoint.ID == id {
			// Validate the updated endpoint
			if err := validateEndpoint(&updated, "endpoint"); err != nil {
				return fmt.Errorf("invalid endpoint configuration: %w", err)
			}

			c.Endpoints[i] = updated
			return nil
		}
	}
	return fmt.Errorf("endpoint with ID '%s' not found", id)
}

// ListEndpoints returns all endpoints
func (c *Config) ListEndpoints() []EndpointConfig {
	return c.Endpoints
}

// GetEnabledEndpoints returns only enabled endpoints
func (c *Config) GetEnabledEndpoints() []EndpointConfig {
	var enabled []EndpointConfig
	for _, endpoint := range c.Endpoints {
		if endpoint.Enabled {
			enabled = append(enabled, endpoint)
		}
	}
	return enabled
}
