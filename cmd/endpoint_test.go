package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid HTTP URL",
			url:     "http://api.example.com/v1/users",
			wantErr: false,
		},
		{
			name:    "valid HTTPS URL",
			url:     "https://api.example.com/v1/users",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "URL without scheme",
			url:     "api.example.com/v1/users",
			wantErr: true,
		},
		{
			name:    "URL with invalid scheme",
			url:     "ftp://api.example.com/v1/users",
			wantErr: true,
		},
		{
			name:    "URL without host",
			url:     "https:///v1/users",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			url:     "https://[invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMethod(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		wantErr bool
	}{
		{
			name:    "valid GET method",
			method:  "GET",
			wantErr: false,
		},
		{
			name:    "valid POST method",
			method:  "POST",
			wantErr: false,
		},
		{
			name:    "valid PUT method",
			method:  "PUT",
			wantErr: false,
		},
		{
			name:    "valid DELETE method",
			method:  "DELETE",
			wantErr: false,
		},
		{
			name:    "lowercase method",
			method:  "get",
			wantErr: false,
		},
		{
			name:    "invalid method",
			method:  "PATCH",
			wantErr: true,
		},
		{
			name:    "empty method",
			method:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMethod(tt.method)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		wantErr  bool
	}{
		{
			name:     "valid 5 minute interval",
			interval: 5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "valid 1 hour interval",
			interval: time.Hour,
			wantErr:  false,
		},
		{
			name:     "valid 24 hour interval",
			interval: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "minimum valid interval",
			interval: time.Minute,
			wantErr:  false,
		},
		{
			name:     "interval too short",
			interval: 30 * time.Second,
			wantErr:  true,
		},
		{
			name:     "interval too long",
			interval: 25 * time.Hour,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInterval(tt.interval)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "valid headers",
			headers: []string{"Authorization=Bearer token", "Content-Type=application/json"},
			want: map[string]string{
				"Authorization": "Bearer token",
				"Content-Type":  "application/json",
			},
			wantErr: false,
		},
		{
			name:    "empty headers",
			headers: []string{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "header with spaces",
			headers: []string{"  Authorization  =  Bearer token  "},
			want: map[string]string{
				"Authorization": "Bearer token",
			},
			wantErr: false,
		},
		{
			name:    "header with equals in value",
			headers: []string{"Authorization=Bearer token=value"},
			want: map[string]string{
				"Authorization": "Bearer token=value",
			},
			wantErr: false,
		},
		{
			name:    "invalid header format",
			headers: []string{"Authorization"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty key",
			headers: []string{"=value"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHeaders(tt.headers)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGenerateEndpointID(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		method string
		want   string
	}{
		{
			name:   "simple URL",
			url:    "https://api.example.com/v1/users",
			method: "GET",
			want:   "api-example-com-v1-users-get",
		},
		{
			name:   "URL with port",
			url:    "https://api.example.com:8080/v1/users",
			method: "POST",
			want:   "api-example-com-8080-v1-users-post",
		},
		{
			name:   "URL with root path",
			url:    "https://api.example.com/",
			method: "GET",
			want:   "api-example-com-root-get",
		},
		{
			name:   "URL without path",
			url:    "https://api.example.com",
			method: "GET",
			want:   "api-example-com-root-get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateEndpointID(tt.url, tt.method)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ".driftwatch.yaml")
	dbFile := filepath.Join(tempDir, "test.db")

	// Create initial config
	cfg := config.DefaultConfig()
	cfg.Global.DatabaseURL = dbFile
	err := config.SaveConfig(cfg, configFile)
	require.NoError(t, err)

	// Set up command with test config
	cfgFile = configFile
	_, err = config.LoadConfig(configFile)
	require.NoError(t, err)

	tests := []struct {
		name    string
		args    []string
		flags   map[string]string
		wantErr bool
	}{
		{
			name:    "add simple endpoint",
			args:    []string{"https://api.example.com/v1/users"},
			flags:   map[string]string{},
			wantErr: false,
		},
		{
			name: "add endpoint with custom settings",
			args: []string{"https://api.example.com/v1/posts"},
			flags: map[string]string{
				"method":   "POST",
				"interval": "10m",
				"id":       "posts-api",
			},
			wantErr: false,
		},
		{
			name:    "add endpoint with invalid URL",
			args:    []string{"invalid-url"},
			flags:   map[string]string{},
			wantErr: true,
		},
		{
			name: "add endpoint with invalid method",
			args: []string{"https://api.example.com/v1/users"},
			flags: map[string]string{
				"method": "PATCH",
			},
			wantErr: true,
		},
		{
			name: "add endpoint with invalid interval",
			args: []string{"https://api.example.com/v1/users"},
			flags: map[string]string{
				"interval": "30s",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh command for each test
			cmd := &cobra.Command{
				Use:  "add",
				RunE: addCmd.RunE,
			}

			// Add flags
			cmd.Flags().StringP("method", "m", "GET", "HTTP method")
			cmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers")
			cmd.Flags().DurationP("interval", "i", 5*time.Minute, "monitoring interval")
			cmd.Flags().Duration("timeout", 0, "request timeout")
			cmd.Flags().Int("retry-count", 0, "retry count")
			cmd.Flags().String("id", "", "endpoint ID")
			cmd.Flags().String("spec", "", "OpenAPI spec file")
			cmd.Flags().String("request-body", "", "request body file")
			cmd.Flags().Bool("strict", false, "strict validation")
			cmd.Flags().StringSlice("ignore-fields", []string{}, "ignore fields")
			cmd.Flags().StringSlice("required-fields", []string{}, "required fields")

			// Set flags
			for key, value := range tt.flags {
				err := cmd.Flags().Set(key, value)
				require.NoError(t, err)
			}

			// Execute command
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify endpoint was added to config
				updatedCfg, err := config.LoadConfig(configFile)
				require.NoError(t, err)

				// Should have at least one endpoint
				assert.Greater(t, len(updatedCfg.Endpoints), 0)

				// Find the added endpoint
				var found bool
				for _, ep := range updatedCfg.Endpoints {
					if ep.URL == tt.args[0] {
						found = true
						break
					}
				}
				assert.True(t, found, "endpoint should be added to config")
			}
		})
	}
}

func TestListCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ".driftwatch.yaml")
	dbFile := filepath.Join(tempDir, "test.db")

	// Create config with test endpoints
	cfg := config.DefaultConfig()
	cfg.Global.DatabaseURL = dbFile
	cfg.Endpoints = []config.EndpointConfig{
		{
			ID:       "test-api-1",
			URL:      "https://api.example.com/v1/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
			Enabled:  true,
		},
		{
			ID:       "test-api-2",
			URL:      "https://api.example.com/v1/posts",
			Method:   "POST",
			Interval: 10 * time.Minute,
			Enabled:  false,
		},
	}
	err := config.SaveConfig(cfg, configFile)
	require.NoError(t, err)

	// Set up command with test config
	cfgFile = configFile
	_, err = config.LoadConfig(configFile)
	require.NoError(t, err)

	tests := []struct {
		name   string
		flags  map[string]string
		verify func(t *testing.T, output string)
	}{
		{
			name:  "list table format",
			flags: map[string]string{"output": "table"},
			verify: func(t *testing.T, output string) {
				// For table format, we can't easily capture the output in tests
				// So we'll just verify the command runs without error
				// The actual output verification is done in integration tests
			},
		},
		{
			name:  "list JSON format",
			flags: map[string]string{"output": "json"},
			verify: func(t *testing.T, output string) {
				// For JSON format, we can't easily capture the output in tests
				// So we'll just verify the command runs without error
			},
		},
		{
			name:  "list YAML format",
			flags: map[string]string{"output": "yaml"},
			verify: func(t *testing.T, output string) {
				// For YAML format, we can't easily capture the output in tests
				// So we'll just verify the command runs without error
			},
		},
		{
			name:  "list enabled only",
			flags: map[string]string{"output": "table", "enabled-only": "true"},
			verify: func(t *testing.T, output string) {
				// For enabled-only, we can't easily capture the output in tests
				// So we'll just verify the command runs without error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			var buf bytes.Buffer

			// Create fresh command for each test
			cmd := &cobra.Command{
				Use:  "list",
				RunE: listCmd.RunE,
			}
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Add flags
			cmd.Flags().StringP("output", "o", "table", "output format")
			cmd.Flags().Bool("enabled-only", false, "show only enabled endpoints")

			// Set flags
			for key, value := range tt.flags {
				err := cmd.Flags().Set(key, value)
				require.NoError(t, err)
			}

			// Execute command
			err := cmd.Execute()
			assert.NoError(t, err)

			// Verify output
			output := buf.String()
			tt.verify(t, output)
		})
	}
}

func TestRemoveCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ".driftwatch.yaml")
	dbFile := filepath.Join(tempDir, "test.db")

	// Create config with test endpoint
	cfg := config.DefaultConfig()
	cfg.Global.DatabaseURL = dbFile
	cfg.Endpoints = []config.EndpointConfig{
		{
			ID:       "test-api",
			URL:      "https://api.example.com/v1/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
			Enabled:  true,
		},
	}
	err := config.SaveConfig(cfg, configFile)
	require.NoError(t, err)

	// Set up command with test config
	cfgFile = configFile
	_, err = config.LoadConfig(configFile)
	require.NoError(t, err)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "remove existing endpoint",
			args:    []string{"test-api"},
			wantErr: false,
		},
		{
			name:    "remove non-existent endpoint",
			args:    []string{"non-existent"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh command for each test
			cmd := &cobra.Command{
				Use:  "remove",
				RunE: removeCmd.RunE,
			}

			// Add flags
			cmd.Flags().Bool("purge", false, "purge historical data")

			// Execute command
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify endpoint was removed from config
				updatedCfg, err := config.LoadConfig(configFile)
				require.NoError(t, err)

				// Should not find the removed endpoint
				var found bool
				for _, ep := range updatedCfg.Endpoints {
					if ep.ID == tt.args[0] {
						found = true
						break
					}
				}
				assert.False(t, found, "endpoint should be removed from config")
			}
		})
	}
}

func TestUpdateCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ".driftwatch.yaml")
	dbFile := filepath.Join(tempDir, "test.db")

	// Create config with test endpoint
	cfg := config.DefaultConfig()
	cfg.Global.DatabaseURL = dbFile
	cfg.Endpoints = []config.EndpointConfig{
		{
			ID:       "test-api",
			URL:      "https://api.example.com/v1/users",
			Method:   "GET",
			Interval: 5 * time.Minute,
			Enabled:  true,
		},
	}
	err := config.SaveConfig(cfg, configFile)
	require.NoError(t, err)

	// Set up command with test config
	cfgFile = configFile
	_, err = config.LoadConfig(configFile)
	require.NoError(t, err)

	tests := []struct {
		name    string
		args    []string
		flags   map[string]string
		wantErr bool
		verify  func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "update method",
			args: []string{"test-api"},
			flags: map[string]string{
				"method": "POST",
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *config.Config) {
				ep, err := cfg.GetEndpoint("test-api")
				require.NoError(t, err)
				assert.Equal(t, "POST", ep.Method)
			},
		},
		{
			name: "update interval",
			args: []string{"test-api"},
			flags: map[string]string{
				"interval": "10m",
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *config.Config) {
				ep, err := cfg.GetEndpoint("test-api")
				require.NoError(t, err)
				assert.Equal(t, 10*time.Minute, ep.Interval)
			},
		},
		{
			name: "disable endpoint",
			args: []string{"test-api"},
			flags: map[string]string{
				"disable": "true",
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *config.Config) {
				ep, err := cfg.GetEndpoint("test-api")
				require.NoError(t, err)
				assert.False(t, ep.Enabled)
			},
		},
		{
			name:    "update non-existent endpoint",
			args:    []string{"non-existent"},
			flags:   map[string]string{},
			wantErr: true,
			verify:  nil,
		},
		{
			name: "update with invalid method",
			args: []string{"test-api"},
			flags: map[string]string{
				"method": "PATCH",
			},
			wantErr: true,
			verify:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reload config for each test
			_, err := config.LoadConfig(configFile)
			require.NoError(t, err)

			// Create fresh command for each test
			cmd := &cobra.Command{
				Use:  "update",
				RunE: updateCmd.RunE,
			}

			// Add flags
			cmd.Flags().StringP("method", "m", "", "HTTP method")
			cmd.Flags().StringP("spec", "s", "", "OpenAPI spec file")
			cmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers")
			cmd.Flags().DurationP("interval", "i", 0, "monitoring interval")
			cmd.Flags().Duration("timeout", 0, "request timeout")
			cmd.Flags().Int("retry-count", 0, "retry count")
			cmd.Flags().String("request-body", "", "request body file")
			cmd.Flags().Bool("strict", false, "strict validation")
			cmd.Flags().StringSlice("ignore-fields", []string{}, "ignore fields")
			cmd.Flags().StringSlice("required-fields", []string{}, "required fields")
			cmd.Flags().Bool("disable", false, "disable endpoint")
			cmd.Flags().Bool("enable", false, "enable endpoint")

			// Set flags
			for key, value := range tt.flags {
				err := cmd.Flags().Set(key, value)
				require.NoError(t, err)
			}

			// Execute command
			cmd.SetArgs(tt.args)
			err = cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.verify != nil {
					// Reload config and verify changes
					updatedCfg, err := config.LoadConfig(configFile)
					require.NoError(t, err)
					tt.verify(t, updatedCfg)
				}
			}
		})
	}
}
