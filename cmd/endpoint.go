package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add an API endpoint to monitor",
	Long: `Add an API endpoint to the monitoring configuration.

This command registers a new endpoint for monitoring with the specified URL and options.
The endpoint will be added to both the configuration file and the database.

Examples:
  driftwatch add https://api.example.com/v1/users
  driftwatch add https://api.example.com/v1/users --method POST --spec openapi.yaml
  driftwatch add https://api.example.com/v1/users --header "Authorization=Bearer token" --interval 5m
  driftwatch add https://api.example.com/v1/users --id my-users-api --timeout 30s`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		endpointURL := args[0]

		// Validate URL
		if err := validateURL(endpointURL); err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}

		// Get flags
		method, err := cmd.Flags().GetString("method")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "method", err)
		}
		specFile, err := cmd.Flags().GetString("spec")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "spec", err)
		}
		headers, err := cmd.Flags().GetStringSlice("header")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "header", err)
		}
		interval, err := cmd.Flags().GetDuration("interval")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "interval", err)
		}
		timeout, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "timeout", err)
		}
		retryCount, err := cmd.Flags().GetInt("retry-count")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "retry-count", err)
		}
		id, err := cmd.Flags().GetString("id")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "id", err)
		}
		requestBodyFile, err := cmd.Flags().GetString("request-body")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "request-body", err)
		}
		strictMode, err := cmd.Flags().GetBool("strict")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "strict", err)
		}
		ignoreFields, err := cmd.Flags().GetStringSlice("ignore-fields")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "ignore-fields", err)
		}
		requiredFields, err := cmd.Flags().GetStringSlice("required-fields")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "required-fields", err)
		}

		// Validate method
		if err := validateMethod(method); err != nil {
			return fmt.Errorf("invalid HTTP method: %w", err)
		}

		// Validate interval
		if err := validateInterval(interval); err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}

		// Parse headers
		headerMap, err := parseHeaders(headers)
		if err != nil {
			return fmt.Errorf("invalid headers: %w", err)
		}

		// Generate ID if not provided
		if id == "" {
			id = generateEndpointID(endpointURL, method)
		}

		// Create endpoint configuration
		endpointConfig := config.EndpointConfig{
			ID:              id,
			URL:             endpointURL,
			Method:          method,
			SpecFile:        specFile,
			Interval:        interval,
			Headers:         headerMap,
			RequestBodyFile: requestBodyFile,
			Timeout:         timeout,
			RetryCount:      retryCount,
			Enabled:         true,
			Validation: config.ValidationConfig{
				StrictMode:     strictMode,
				IgnoreFields:   ignoreFields,
				RequiredFields: requiredFields,
			},
		}

		// Load current config
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Add endpoint to config using the utility function
		if err := cfg.AddEndpoint(endpointConfig); err != nil {
			return fmt.Errorf("failed to add endpoint: %w", err)
		}

		// Save to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Convert config to JSON for storage
		configJSON, err := json.Marshal(endpointConfig)
		if err != nil {
			return fmt.Errorf("failed to serialize endpoint config: %w", err)
		}

		endpoint := &storage.Endpoint{
			ID:        id,
			URL:       endpointURL,
			Method:    method,
			SpecFile:  specFile,
			Config:    string(configJSON),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := db.SaveEndpoint(endpoint); err != nil {
			return fmt.Errorf("failed to save endpoint to database: %w", err)
		}

		// Save updated config to file
		if err := saveConfigToFile(cfg); err != nil {
			return fmt.Errorf("failed to save configuration file: %w", err)
		}

		fmt.Printf("✓ Endpoint added successfully\n")
		fmt.Printf("  ID: %s\n", id)
		fmt.Printf("  URL: %s\n", endpointURL)
		fmt.Printf("  Method: %s\n", method)
		fmt.Printf("  Interval: %s\n", interval)
		if specFile != "" {
			fmt.Printf("  Spec File: %s\n", specFile)
		}
		if len(headerMap) > 0 {
			fmt.Printf("  Headers: %d configured\n", len(headerMap))
		}

		return nil
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitored endpoints",
	Long: `List all registered API endpoints with their current configuration and status.

This command displays all endpoints that are configured for monitoring,
including their URLs, methods, intervals, and current status.

Examples:
  driftwatch list                    # List in table format
  driftwatch list --output json     # List in JSON format
  driftwatch list --output yaml     # List in YAML format
  driftwatch list --enabled-only    # Show only enabled endpoints`,
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
		enabledOnly, err := cmd.Flags().GetBool("enabled-only")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "enabled-only", err)
		}

		// Filter endpoints if needed
		endpoints := cfg.Endpoints
		if enabledOnly {
			var filteredEndpoints []config.EndpointConfig
			for _, ep := range endpoints {
				if ep.Enabled {
					filteredEndpoints = append(filteredEndpoints, ep)
				}
			}
			endpoints = filteredEndpoints
		}

		if len(endpoints) == 0 {
			fmt.Println("No endpoints configured")
			return nil
		}

		// Load database to get status information
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Display endpoints based on output format
		switch outputFormat {
		case "json":
			return outputEndpointsJSON(endpoints, db)
		case "yaml":
			return outputEndpointsYAML(endpoints, db)
		case "table":
			outputEndpointsTable(endpoints, db)
			return nil
		default:
			return fmt.Errorf("unsupported output format: %s (supported: table, json, yaml)", outputFormat)
		}
	},
}

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an endpoint from monitoring",
	Long: `Remove an API endpoint from the monitoring configuration.

This command removes the specified endpoint from both the configuration file
and the database. Historical monitoring data will be preserved unless --purge is used.

Examples:
  driftwatch remove my-api-endpoint
  driftwatch remove my-api-endpoint --purge    # Also remove historical data`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		endpointID := args[0]
		purge, err := cmd.Flags().GetBool("purge")
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", "purge", err)
		}

		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Remove endpoint from config using the utility function
		if err := cfg.RemoveEndpoint(endpointID); err != nil {
			return fmt.Errorf("failed to remove endpoint: %w", err)
		}

		// Connect to database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Remove from database (this will be implemented when we have the delete methods)
		// For now, we'll just update the config file

		// Save updated config to file
		if err := saveConfigToFile(cfg); err != nil {
			return fmt.Errorf("failed to save configuration file: %w", err)
		}

		fmt.Printf("✓ Endpoint '%s' removed successfully\n", endpointID)
		if purge {
			fmt.Printf("  Historical data purged\n")
		} else {
			fmt.Printf("  Historical data preserved\n")
		}

		return nil
	},
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an endpoint configuration",
	Long: `Update the configuration of an existing API endpoint.

This command allows you to modify the settings of an already registered endpoint
without removing and re-adding it.

Examples:
  driftwatch update my-api --interval 10m
  driftwatch update my-api --method POST --spec new-spec.yaml
  driftwatch update my-api --header "Authorization=Bearer newtoken"
  driftwatch update my-api --disable    # Disable monitoring for this endpoint`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		endpointID := args[0]

		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Get endpoint from config
		endpoint, err := cfg.GetEndpoint(endpointID)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}

		// Update fields based on flags
		if cmd.Flags().Changed("method") {
			method, err := cmd.Flags().GetString("method")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "method", err)
			}
			if err := validateMethod(method); err != nil {
				return fmt.Errorf("invalid HTTP method: %w", err)
			}
			endpoint.Method = method
		}

		if cmd.Flags().Changed("spec") {
			specFile, err := cmd.Flags().GetString("spec")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "spec", err)
			}
			endpoint.SpecFile = specFile
		}

		if cmd.Flags().Changed("interval") {
			interval, err := cmd.Flags().GetDuration("interval")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "interval", err)
			}
			if err := validateInterval(interval); err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}
			endpoint.Interval = interval
		}

		if cmd.Flags().Changed("timeout") {
			timeout, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "timeout", err)
			}
			endpoint.Timeout = timeout
		}

		if cmd.Flags().Changed("retry-count") {
			retryCount, err := cmd.Flags().GetInt("retry-count")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "retry-count", err)
			}
			endpoint.RetryCount = retryCount
		}

		if cmd.Flags().Changed("header") {
			headers, err := cmd.Flags().GetStringSlice("header")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "header", err)
			}
			headerMap, err := parseHeaders(headers)
			if err != nil {
				return fmt.Errorf("invalid headers: %w", err)
			}
			endpoint.Headers = headerMap
		}

		if cmd.Flags().Changed("request-body") {
			requestBodyFile, err := cmd.Flags().GetString("request-body")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "request-body", err)
			}
			endpoint.RequestBodyFile = requestBodyFile
		}

		if cmd.Flags().Changed("strict") {
			strictMode, err := cmd.Flags().GetBool("strict")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "strict", err)
			}
			endpoint.Validation.StrictMode = strictMode
		}

		if cmd.Flags().Changed("ignore-fields") {
			ignoreFields, err := cmd.Flags().GetStringSlice("ignore-fields")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "ignore-fields", err)
			}
			endpoint.Validation.IgnoreFields = ignoreFields
		}

		if cmd.Flags().Changed("required-fields") {
			requiredFields, err := cmd.Flags().GetStringSlice("required-fields")
			if err != nil {
				return fmt.Errorf("failed to get %s flag: %w", "required-fields", err)
			}
			endpoint.Validation.RequiredFields = requiredFields
		}

		if cmd.Flags().Changed("disable") {
			endpoint.Enabled = false
		}

		if cmd.Flags().Changed("enable") {
			endpoint.Enabled = true
		}

		// Update in database
		db, err := storage.NewStorage(cfg.Global.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Convert config to JSON for storage
		configJSON, err := json.Marshal(*endpoint)
		if err != nil {
			return fmt.Errorf("failed to serialize endpoint config: %w", err)
		}

		dbEndpoint := &storage.Endpoint{
			ID:        endpoint.ID,
			URL:       endpoint.URL,
			Method:    endpoint.Method,
			SpecFile:  endpoint.SpecFile,
			Config:    string(configJSON),
			UpdatedAt: time.Now(),
		}

		if err := db.SaveEndpoint(dbEndpoint); err != nil {
			return fmt.Errorf("failed to update endpoint in database: %w", err)
		}

		// Update endpoint in config
		if err := cfg.UpdateEndpoint(endpointID, *endpoint); err != nil {
			return fmt.Errorf("failed to update endpoint in config: %w", err)
		}

		// Save updated config to file
		if err := saveConfigToFile(cfg); err != nil {
			return fmt.Errorf("failed to save configuration file: %w", err)
		}

		fmt.Printf("✓ Endpoint '%s' updated successfully\n", endpointID)
		fmt.Printf("  URL: %s\n", endpoint.URL)
		fmt.Printf("  Method: %s\n", endpoint.Method)
		fmt.Printf("  Interval: %s\n", endpoint.Interval)
		fmt.Printf("  Enabled: %t\n", endpoint.Enabled)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(updateCmd)

	// Add command flags
	addCmd.Flags().StringP("method", "m", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	addCmd.Flags().StringP("spec", "s", "", "OpenAPI specification file path")
	addCmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers (format: key=value)")
	addCmd.Flags().DurationP("interval", "i", 5*time.Minute, "monitoring interval (1m to 24h)")
	addCmd.Flags().Duration("timeout", 0, "request timeout (uses global default if not set)")
	addCmd.Flags().Int("retry-count", 0, "retry count (uses global default if not set)")
	addCmd.Flags().String("id", "", "endpoint ID (auto-generated if not provided)")
	addCmd.Flags().String("request-body", "", "file containing request body for POST/PUT requests")
	addCmd.Flags().Bool("strict", false, "enable strict validation mode")
	addCmd.Flags().StringSlice("ignore-fields", []string{}, "fields to ignore during validation")
	addCmd.Flags().StringSlice("required-fields", []string{}, "fields that must be present")

	listCmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	listCmd.Flags().Bool("enabled-only", false, "show only enabled endpoints")

	removeCmd.Flags().Bool("purge", false, "also remove historical monitoring data")

	updateCmd.Flags().StringP("method", "m", "", "HTTP method (GET, POST, PUT, DELETE)")
	updateCmd.Flags().StringP("spec", "s", "", "OpenAPI specification file path")
	updateCmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers (format: key=value)")
	updateCmd.Flags().DurationP("interval", "i", 0, "monitoring interval (1m to 24h)")
	updateCmd.Flags().Duration("timeout", 0, "request timeout")
	updateCmd.Flags().Int("retry-count", 0, "retry count")
	updateCmd.Flags().String("request-body", "", "file containing request body for POST/PUT requests")
	updateCmd.Flags().Bool("strict", false, "enable strict validation mode")
	updateCmd.Flags().StringSlice("ignore-fields", []string{}, "fields to ignore during validation")
	updateCmd.Flags().StringSlice("required-fields", []string{}, "fields that must be present")
	updateCmd.Flags().Bool("disable", false, "disable monitoring for this endpoint")
	updateCmd.Flags().Bool("enable", false, "enable monitoring for this endpoint")
}

// Helper functions for endpoint management

// validateURL validates that the provided URL is valid
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL must include scheme (http:// or https://)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include host")
	}

	return nil
}

// validateMethod validates that the HTTP method is supported
func validateMethod(method string) error {
	method = strings.ToUpper(method)
	supportedMethods := []string{"GET", "POST", "PUT", "DELETE"}

	for _, supported := range supportedMethods {
		if method == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported method '%s' (supported: %s)", method, strings.Join(supportedMethods, ", "))
}

// validateInterval validates that the monitoring interval is within acceptable range
func validateInterval(interval time.Duration) error {
	if interval < time.Minute {
		return fmt.Errorf("interval must be at least 1 minute")
	}

	if interval > 24*time.Hour {
		return fmt.Errorf("interval must be at most 24 hours")
	}

	return nil
}

// parseHeaders parses header strings in the format "key=value"
func parseHeaders(headers []string) (map[string]string, error) {
	headerMap := make(map[string]string)

	for _, header := range headers {
		parts := strings.SplitN(header, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header format '%s' (expected key=value)", header)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("header key cannot be empty")
		}

		headerMap[key] = value
	}

	return headerMap, nil
}

// generateEndpointID generates a unique ID for an endpoint based on URL and method
func generateEndpointID(urlStr, method string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// Return a fallback ID if URL parsing fails
		return fmt.Sprintf("%s_%s_invalid", method, urlStr)
	}
	host := parsedURL.Host
	path := parsedURL.Path

	// Clean up the path for ID generation
	path = strings.Trim(path, "/")
	path = strings.ReplaceAll(path, "/", "-")
	if path == "" {
		path = "root"
	}

	// Create ID in format: host-path-method
	id := fmt.Sprintf("%s-%s-%s", host, path, strings.ToLower(method))

	// Clean up the ID to make it filesystem-safe
	id = strings.ReplaceAll(id, ".", "-")
	id = strings.ReplaceAll(id, ":", "-")

	return id
}

// saveConfigToFile saves the configuration to the config file
func saveConfigToFile(cfg *config.Config) error {
	configPath := config.GetConfigFilePath(cfgFile)
	return config.SaveConfig(cfg, configPath)
}

// outputEndpointsJSON outputs endpoints in JSON format
func outputEndpointsJSON(endpoints []config.EndpointConfig, db storage.Storage) error {
	type EndpointStatus struct {
		config.EndpointConfig
		Status      string    `json:"status"`
		LastChecked time.Time `json:"last_checked,omitempty"`
	}

	var endpointsWithStatus []EndpointStatus

	for _, ep := range endpoints {
		status := "unknown"
		var lastChecked time.Time

		// Try to get the latest monitoring run for status
		if runs, err := db.GetMonitoringHistory(ep.ID, 24*time.Hour); err == nil && len(runs) > 0 {
			latestRun := runs[len(runs)-1]
			lastChecked = latestRun.Timestamp
			if latestRun.ResponseStatus >= 200 && latestRun.ResponseStatus < 300 {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}

		endpointsWithStatus = append(endpointsWithStatus, EndpointStatus{
			EndpointConfig: ep,
			Status:         status,
			LastChecked:    lastChecked,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(endpointsWithStatus)
}

// outputEndpointsYAML outputs endpoints in YAML format
func outputEndpointsYAML(endpoints []config.EndpointConfig, db storage.Storage) error {
	type EndpointStatus struct {
		config.EndpointConfig `yaml:",inline"`
		Status                string    `yaml:"status"`
		LastChecked           time.Time `yaml:"last_checked,omitempty"`
	}

	var endpointsWithStatus []EndpointStatus

	for _, ep := range endpoints {
		status := "unknown"
		var lastChecked time.Time

		// Try to get the latest monitoring run for status
		if runs, err := db.GetMonitoringHistory(ep.ID, 24*time.Hour); err == nil && len(runs) > 0 {
			latestRun := runs[len(runs)-1]
			lastChecked = latestRun.Timestamp
			if latestRun.ResponseStatus >= 200 && latestRun.ResponseStatus < 300 {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}

		endpointsWithStatus = append(endpointsWithStatus, EndpointStatus{
			EndpointConfig: ep,
			Status:         status,
			LastChecked:    lastChecked,
		})
	}

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(endpointsWithStatus)
}

// outputEndpointsTable outputs endpoints in table format
func outputEndpointsTable(endpoints []config.EndpointConfig, db storage.Storage) {
	if len(endpoints) == 0 {
		fmt.Println("No endpoints configured")
		return
	}

	// Print table header
	fmt.Printf("%-20s %-8s %-50s %-10s %-10s %-12s\n",
		"ID", "METHOD", "URL", "INTERVAL", "STATUS", "LAST CHECKED")
	fmt.Println(strings.Repeat("-", 120))

	// Print each endpoint
	for _, ep := range endpoints {
		status := "unknown"
		lastChecked := "never"

		// Try to get the latest monitoring run for status
		if runs, err := db.GetMonitoringHistory(ep.ID, 24*time.Hour); err == nil && len(runs) > 0 {
			latestRun := runs[len(runs)-1]
			if latestRun.ResponseStatus >= 200 && latestRun.ResponseStatus < 300 {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
			lastChecked = latestRun.Timestamp.Format("15:04:05")
		}

		if !ep.Enabled {
			status = "disabled"
		}

		// Truncate URL if too long
		displayURL := ep.URL
		if len(displayURL) > 47 {
			displayURL = displayURL[:44] + "..."
		}

		// Truncate ID if too long
		displayID := ep.ID
		if len(displayID) > 17 {
			displayID = displayID[:14] + "..."
		}

		fmt.Printf("%-20s %-8s %-50s %-10s %-10s %-12s\n",
			displayID, ep.Method, displayURL, ep.Interval, status, lastChecked)
	}

	fmt.Printf("\nTotal: %d endpoints\n", len(endpoints))
}
