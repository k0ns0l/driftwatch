package config

import (
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/spf13/viper"
)

// Config represents the complete DriftWatch configuration
type Config struct {
	Project   ProjectConfig    `yaml:"project" mapstructure:"project"`
	Global    GlobalConfig     `yaml:"global" mapstructure:"global"`
	Endpoints []EndpointConfig `yaml:"endpoints" mapstructure:"endpoints"`
	Alerting  AlertingConfig   `yaml:"alerting" mapstructure:"alerting"`
	Reporting ReportingConfig  `yaml:"reporting" mapstructure:"reporting"`
	Retention RetentionConfig  `yaml:"retention" mapstructure:"retention"`
}

// ProjectConfig contains project-level settings
type ProjectConfig struct {
	Name        string `yaml:"name" mapstructure:"name"`
	Description string `yaml:"description" mapstructure:"description"`
	Version     string `yaml:"version" mapstructure:"version"`
}

// GlobalConfig contains global settings that apply to all endpoints
type GlobalConfig struct {
	UserAgent   string        `yaml:"user_agent" mapstructure:"user_agent"`
	Timeout     time.Duration `yaml:"timeout" mapstructure:"timeout"`
	RetryCount  int           `yaml:"retry_count" mapstructure:"retry_count"`
	RetryDelay  time.Duration `yaml:"retry_delay" mapstructure:"retry_delay"`
	MaxWorkers  int           `yaml:"max_workers" mapstructure:"max_workers"`
	DatabaseURL string        `yaml:"database_url" mapstructure:"database_url"`
}

// EndpointConfig represents configuration for a single API endpoint
type EndpointConfig struct {
	ID              string            `yaml:"id" mapstructure:"id"`
	URL             string            `yaml:"url" mapstructure:"url"`
	Method          string            `yaml:"method" mapstructure:"method"`
	SpecFile        string            `yaml:"spec_file,omitempty" mapstructure:"spec_file"`
	Interval        time.Duration     `yaml:"interval" mapstructure:"interval"`
	Headers         map[string]string `yaml:"headers,omitempty" mapstructure:"headers"`
	Auth            *AuthConfig       `yaml:"auth,omitempty" mapstructure:"auth"`
	Validation      ValidationConfig  `yaml:"validation" mapstructure:"validation"`
	RequestBodyFile string            `yaml:"request_body_file,omitempty" mapstructure:"request_body_file"`
	Timeout         time.Duration     `yaml:"timeout,omitempty" mapstructure:"timeout"`
	RetryCount      int               `yaml:"retry_count,omitempty" mapstructure:"retry_count"`
	Enabled         bool              `yaml:"enabled" mapstructure:"enabled"`
}

// AuthConfig contains authentication configuration for endpoints
type AuthConfig struct {
	Type   AuthType    `yaml:"type" mapstructure:"type"`
	Bearer *BearerAuth `yaml:"bearer,omitempty" mapstructure:"bearer"`
	Basic  *BasicAuth  `yaml:"basic,omitempty" mapstructure:"basic"`
	APIKey *APIKeyAuth `yaml:"api_key,omitempty" mapstructure:"api_key"`
	OAuth2 *OAuth2Auth `yaml:"oauth2,omitempty" mapstructure:"oauth2"`
}

// AuthType represents the type of authentication
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth2 AuthType = "oauth2"
)

// BearerAuth represents Bearer token authentication
type BearerAuth struct {
	Token string `yaml:"token" mapstructure:"token"`
}

// BasicAuth represents HTTP Basic authentication
type BasicAuth struct {
	Username string `yaml:"username" mapstructure:"username"`
	Password string `yaml:"password" mapstructure:"password"`
}

// APIKeyAuth represents API key authentication via custom headers
type APIKeyAuth struct {
	Header string `yaml:"header" mapstructure:"header"`
	Value  string `yaml:"value" mapstructure:"value"`
}

// OAuth2Auth represents OAuth 2.0 client credentials flow
type OAuth2Auth struct {
	TokenURL     string            `yaml:"token_url" mapstructure:"token_url"`
	ClientID     string            `yaml:"client_id" mapstructure:"client_id"`
	ClientSecret string            `yaml:"client_secret" mapstructure:"client_secret"`
	Scopes       []string          `yaml:"scopes,omitempty" mapstructure:"scopes"`
	ExtraParams  map[string]string `yaml:"extra_params,omitempty" mapstructure:"extra_params"`
}

// ValidationConfig contains validation-specific settings
type ValidationConfig struct {
	StrictMode     bool     `yaml:"strict_mode" mapstructure:"strict_mode"`
	IgnoreFields   []string `yaml:"ignore_fields,omitempty" mapstructure:"ignore_fields"`
	RequiredFields []string `yaml:"required_fields,omitempty" mapstructure:"required_fields"`
}

// AlertingConfig contains alerting configuration
type AlertingConfig struct {
	Enabled  bool                 `yaml:"enabled" mapstructure:"enabled"`
	Channels []AlertChannelConfig `yaml:"channels" mapstructure:"channels"`
	Rules    []AlertRuleConfig    `yaml:"rules" mapstructure:"rules"`
}

// AlertChannelConfig represents a single alert channel
type AlertChannelConfig struct {
	Type     string                 `yaml:"type" mapstructure:"type"` // slack, email, webhook
	Name     string                 `yaml:"name" mapstructure:"name"`
	Enabled  bool                   `yaml:"enabled" mapstructure:"enabled"`
	Settings map[string]interface{} `yaml:"settings" mapstructure:"settings"`
}

// AlertRuleConfig defines when alerts should be triggered
type AlertRuleConfig struct {
	Name      string   `yaml:"name" mapstructure:"name"`
	Severity  []string `yaml:"severity" mapstructure:"severity"`             // low, medium, high, critical
	Endpoints []string `yaml:"endpoints,omitempty" mapstructure:"endpoints"` // empty means all
	Channels  []string `yaml:"channels" mapstructure:"channels"`
}

// ReportingConfig contains reporting configuration
type ReportingConfig struct {
	RetentionDays int    `yaml:"retention_days" mapstructure:"retention_days"`
	ExportFormat  string `yaml:"export_format" mapstructure:"export_format"` // json, csv, yaml
	IncludeBody   bool   `yaml:"include_body" mapstructure:"include_body"`
}

// RetentionConfig contains data retention policies
type RetentionConfig struct {
	MonitoringRunsDays int           `yaml:"monitoring_runs_days" mapstructure:"monitoring_runs_days"`
	DriftsDays         int           `yaml:"drifts_days" mapstructure:"drifts_days"`
	AlertsDays         int           `yaml:"alerts_days" mapstructure:"alerts_days"`
	AutoCleanup        bool          `yaml:"auto_cleanup" mapstructure:"auto_cleanup"`
	CleanupInterval    time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Project: ProjectConfig{
			Name:        "DriftWatch Project",
			Description: "API drift monitoring project",
			Version:     "1.0.0",
		},
		Global: GlobalConfig{
			UserAgent:   "driftwatch/1.0.0",
			Timeout:     30 * time.Second,
			RetryCount:  3,
			RetryDelay:  5 * time.Second,
			MaxWorkers:  10,
			DatabaseURL: "./driftwatch.db",
		},
		Endpoints: []EndpointConfig{},
		Alerting: AlertingConfig{
			Enabled:  false,
			Channels: []AlertChannelConfig{},
			Rules:    []AlertRuleConfig{},
		},
		Reporting: ReportingConfig{
			RetentionDays: 30,
			ExportFormat:  "json",
			IncludeBody:   false,
		},
		Retention: RetentionConfig{
			MonitoringRunsDays: 30,
			DriftsDays:         90,
			AlertsDays:         30,
			AutoCleanup:        true,
			CleanupInterval:    24 * time.Hour,
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configFile string) (*Config, error) {
	// Set up Viper
	v := viper.New()

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.AddConfigPath(".")
		v.SetConfigType("yaml")
		v.SetConfigName(".driftwatch")
	}

	// Enable environment variable substitution
	v.AutomaticEnv()
	v.SetEnvPrefix("DRIFTWATCH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK, we'll use defaults
		} else {
			return nil, errors.WrapError(err, errors.ErrorTypeConfig, "CONFIG_READ_ERROR", "failed to read config file").
				WithSeverity(errors.SeverityHigh).
				WithGuidance("Check file permissions and YAML syntax")
		}
	}

	// Unmarshal into config struct
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, errors.WrapError(err, errors.ErrorTypeConfig, "CONFIG_UNMARSHAL_ERROR", "failed to unmarshal config").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check configuration file structure and field types")
	}

	// Perform environment variable substitution
	substituteEnvVars(config)

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		if dwe, ok := err.(*errors.DriftWatchError); ok {
			return nil, dwe
		}
		return nil, errors.WrapError(err, errors.ErrorTypeConfig, "CONFIG_VALIDATION_ERROR", "configuration validation failed").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Run 'driftwatch config validate' for detailed error information")
	}

	return config, nil
}

// setDefaults sets default values in Viper
func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()

	v.SetDefault("project.name", defaults.Project.Name)
	v.SetDefault("project.description", defaults.Project.Description)
	v.SetDefault("project.version", defaults.Project.Version)

	v.SetDefault("global.user_agent", defaults.Global.UserAgent)
	v.SetDefault("global.timeout", defaults.Global.Timeout)
	v.SetDefault("global.retry_count", defaults.Global.RetryCount)
	v.SetDefault("global.retry_delay", defaults.Global.RetryDelay)
	v.SetDefault("global.max_workers", defaults.Global.MaxWorkers)
	v.SetDefault("global.database_url", defaults.Global.DatabaseURL)

	v.SetDefault("alerting.enabled", defaults.Alerting.Enabled)

	v.SetDefault("reporting.retention_days", defaults.Reporting.RetentionDays)
	v.SetDefault("reporting.export_format", defaults.Reporting.ExportFormat)
	v.SetDefault("reporting.include_body", defaults.Reporting.IncludeBody)

	v.SetDefault("retention.monitoring_runs_days", defaults.Retention.MonitoringRunsDays)
	v.SetDefault("retention.drifts_days", defaults.Retention.DriftsDays)
	v.SetDefault("retention.alerts_days", defaults.Retention.AlertsDays)
	v.SetDefault("retention.auto_cleanup", defaults.Retention.AutoCleanup)
	v.SetDefault("retention.cleanup_interval", defaults.Retention.CleanupInterval)
}

// substituteEnvVars performs environment variable substitution in configuration values
func substituteEnvVars(config *Config) {
	envVarRegex := regexp.MustCompile(`\$\{([^}]+)\}`)

	// Substitute in endpoint headers
	for i := range config.Endpoints {
		for key, value := range config.Endpoints[i].Headers {
			substituted := envVarRegex.ReplaceAllStringFunc(value, func(match string) string {
				envVar := strings.Trim(match, "${}")
				if envValue := os.Getenv(envVar); envValue != "" {
					return envValue
				}
				return match // Return original if env var not found
			})
			config.Endpoints[i].Headers[key] = substituted
		}
	}

	// Substitute in alert channel settings
	for i := range config.Alerting.Channels {
		for key, value := range config.Alerting.Channels[i].Settings {
			if strValue, ok := value.(string); ok {
				substituted := envVarRegex.ReplaceAllStringFunc(strValue, func(match string) string {
					envVar := strings.Trim(match, "${}")
					if envValue := os.Getenv(envVar); envValue != "" {
						return envValue
					}
					return match
				})
				config.Alerting.Channels[i].Settings[key] = substituted
			}
		}
	}
}
