package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("configuration validation failed with %d error(s):\n- %s",
		len(e), strings.Join(messages, "\n- "))
}

// ValidateConfig validates the entire configuration
func ValidateConfig(config *Config) error {
	var errors ValidationErrors

	// Validate project configuration
	if err := validateProject(&config.Project); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrs...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "project",
				Message: err.Error(),
			})
		}
	}

	// Validate global configuration
	if err := validateGlobal(&config.Global); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrs...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "global",
				Message: err.Error(),
			})
		}
	}

	// Validate endpoints
	endpointIDs := make(map[string]bool)
	for i, endpoint := range config.Endpoints {
		fieldPrefix := fmt.Sprintf("endpoints[%d]", i)

		if err := validateEndpoint(&endpoint, fieldPrefix); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				errors = append(errors, validationErrs...)
			} else {
				errors = append(errors, ValidationError{
					Field:   fieldPrefix,
					Message: err.Error(),
				})
			}
		}

		// Check for duplicate endpoint IDs
		if endpoint.ID != "" {
			if endpointIDs[endpoint.ID] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.id", fieldPrefix),
					Value:   endpoint.ID,
					Message: "duplicate endpoint ID",
				})
			}
			endpointIDs[endpoint.ID] = true
		}
	}

	// Validate alerting configuration
	if err := validateAlerting(&config.Alerting); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrs...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "alerting",
				Message: err.Error(),
			})
		}
	}

	// Validate reporting configuration
	if err := validateReporting(&config.Reporting); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrs...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "reporting",
				Message: err.Error(),
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateProject validates project configuration
func validateProject(project *ProjectConfig) error {
	var errors ValidationErrors

	if strings.TrimSpace(project.Name) == "" {
		errors = append(errors, ValidationError{
			Field:   "project.name",
			Value:   project.Name,
			Message: "project name cannot be empty",
		})
	}

	if len(project.Name) > 100 {
		errors = append(errors, ValidationError{
			Field:   "project.name",
			Value:   project.Name,
			Message: "project name cannot exceed 100 characters",
		})
	}

	if len(project.Description) > 500 {
		errors = append(errors, ValidationError{
			Field:   "project.description",
			Value:   project.Description,
			Message: "project description cannot exceed 500 characters",
		})
	}

	if project.Version != "" {
		versionRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`)
		if !versionRegex.MatchString(project.Version) {
			errors = append(errors, ValidationError{
				Field:   "project.version",
				Value:   project.Version,
				Message: "invalid version format (expected semver format like 1.0.0)",
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateGlobal validates global configuration
func validateGlobal(global *GlobalConfig) error {
	var errors ValidationErrors

	if strings.TrimSpace(global.UserAgent) == "" {
		errors = append(errors, ValidationError{
			Field:   "global.user_agent",
			Value:   global.UserAgent,
			Message: "user agent cannot be empty",
		})
	}

	if global.Timeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "global.timeout",
			Value:   global.Timeout,
			Message: "timeout must be positive",
		})
	}

	if global.Timeout > 5*time.Minute {
		errors = append(errors, ValidationError{
			Field:   "global.timeout",
			Value:   global.Timeout,
			Message: "timeout cannot exceed 5 minutes",
		})
	}

	if global.RetryCount < 0 {
		errors = append(errors, ValidationError{
			Field:   "global.retry_count",
			Value:   global.RetryCount,
			Message: "retry count cannot be negative",
		})
	}

	if global.RetryCount > 10 {
		errors = append(errors, ValidationError{
			Field:   "global.retry_count",
			Value:   global.RetryCount,
			Message: "retry count cannot exceed 10",
		})
	}

	if global.RetryDelay <= 0 {
		errors = append(errors, ValidationError{
			Field:   "global.retry_delay",
			Value:   global.RetryDelay,
			Message: "retry delay must be positive",
		})
	}

	if global.MaxWorkers <= 0 {
		errors = append(errors, ValidationError{
			Field:   "global.max_workers",
			Value:   global.MaxWorkers,
			Message: "max workers must be positive",
		})
	}

	if global.MaxWorkers > 100 {
		errors = append(errors, ValidationError{
			Field:   "global.max_workers",
			Value:   global.MaxWorkers,
			Message: "max workers cannot exceed 100",
		})
	}

	if strings.TrimSpace(global.DatabaseURL) == "" {
		errors = append(errors, ValidationError{
			Field:   "global.database_url",
			Value:   global.DatabaseURL,
			Message: "database URL cannot be empty",
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateEndpoint validates endpoint configuration
func validateEndpoint(endpoint *EndpointConfig, fieldPrefix string) error {
	var errors ValidationErrors

	// Validate basic fields
	errors = append(errors, validateEndpointID(endpoint.ID, fieldPrefix)...)
	errors = append(errors, validateEndpointURL(endpoint.URL, fieldPrefix)...)
	errors = append(errors, validateEndpointMethod(endpoint.Method, fieldPrefix)...)

	// Validate timing configuration
	errors = append(errors, validateEndpointTiming(endpoint, fieldPrefix)...)

	// Validate retry configuration
	errors = append(errors, validateEndpointRetry(endpoint.RetryCount, fieldPrefix)...)

	// Validate authentication configuration
	if endpoint.Auth != nil {
		if err := validateAuth(endpoint.Auth, fmt.Sprintf("%s.auth", fieldPrefix)); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				errors = append(errors, validationErrs...)
			} else {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.auth", fieldPrefix),
					Message: err.Error(),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateEndpointID validates endpoint ID
func validateEndpointID(id, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if strings.TrimSpace(id) == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.id", fieldPrefix),
			Value:   id,
			Message: "endpoint ID cannot be empty",
		})
	} else {
		idRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !idRegex.MatchString(id) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.id", fieldPrefix),
				Value:   id,
				Message: "endpoint ID can only contain letters, numbers, underscores, and hyphens",
			})
		}
	}

	return errors
}

// validateEndpointURL validates endpoint URL
func validateEndpointURL(endpointURL, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if strings.TrimSpace(endpointURL) == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.url", fieldPrefix),
			Value:   endpointURL,
			Message: "endpoint URL cannot be empty",
		})
	} else {
		parsedURL, err := url.Parse(endpointURL)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Value:   endpointURL,
				Message: fmt.Sprintf("invalid URL format: %v", err),
			})
		} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Value:   endpointURL,
				Message: "invalid URL format: missing scheme or host",
			})
		} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", fieldPrefix),
				Value:   endpointURL,
				Message: "URL scheme must be http or https",
			})
		}
	}

	return errors
}

// validateEndpointMethod validates HTTP method
func validateEndpointMethod(method, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.method", fieldPrefix),
			Value:   method,
			Message: "HTTP method cannot be empty",
		})
	} else if !validMethods[normalizedMethod] {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.method", fieldPrefix),
			Value:   method,
			Message: "invalid HTTP method (supported: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)",
		})
	}

	return errors
}

// validateEndpointTiming validates interval and timeout configuration
func validateEndpointTiming(endpoint *EndpointConfig, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	// Validate interval
	if endpoint.Interval <= 0 {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.interval", fieldPrefix),
			Value:   endpoint.Interval,
			Message: "monitoring interval must be positive",
		})
	}

	if endpoint.Interval < time.Minute {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.interval", fieldPrefix),
			Value:   endpoint.Interval,
			Message: "monitoring interval cannot be less than 1 minute",
		})
	}

	if endpoint.Interval > 24*time.Hour {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.interval", fieldPrefix),
			Value:   endpoint.Interval,
			Message: "monitoring interval cannot exceed 24 hours",
		})
	}

	// Validate timeout (if specified)
	if endpoint.Timeout > 0 {
		if endpoint.Timeout > 5*time.Minute {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.timeout", fieldPrefix),
				Value:   endpoint.Timeout,
				Message: "endpoint timeout cannot exceed 5 minutes",
			})
		}
	}

	return errors
}

// validateEndpointRetry validates retry count configuration
func validateEndpointRetry(retryCount int, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if retryCount < 0 {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.retry_count", fieldPrefix),
			Value:   retryCount,
			Message: "retry count cannot be negative",
		})
	}

	if retryCount > 10 {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.retry_count", fieldPrefix),
			Value:   retryCount,
			Message: "retry count cannot exceed 10",
		})
	}

	return errors
}

// validateAuth validates authentication configuration
func validateAuth(auth *AuthConfig, fieldPrefix string) error {
	var errors ValidationErrors

	// Validate auth type
	if err := validateAuthType(auth.Type, fieldPrefix); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			return validationErrs
		}
		return err
	}

	// Validate type-specific configuration
	switch auth.Type {
	case AuthTypeBearer:
		errors = append(errors, validateBearerAuth(auth.Bearer, fieldPrefix)...)
	case AuthTypeBasic:
		errors = append(errors, validateBasicAuth(auth.Basic, fieldPrefix)...)
	case AuthTypeAPIKey:
		errors = append(errors, validateAPIKeyAuth(auth.APIKey, fieldPrefix)...)
	case AuthTypeOAuth2:
		errors = append(errors, validateOAuth2Auth(auth.OAuth2, fieldPrefix)...)
	case AuthTypeNone:
		// No validation needed for none type
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateAuthType validates the authentication type
func validateAuthType(authType AuthType, fieldPrefix string) error {
	validTypes := map[AuthType]bool{
		AuthTypeNone:   true,
		AuthTypeBearer: true,
		AuthTypeBasic:  true,
		AuthTypeAPIKey: true,
		AuthTypeOAuth2: true,
	}

	if !validTypes[authType] {
		return ValidationErrors{ValidationError{
			Field:   fmt.Sprintf("%s.type", fieldPrefix),
			Value:   string(authType),
			Message: "invalid auth type (supported: none, bearer, basic, api_key, oauth2)",
		}}
	}

	return nil
}

// validateBearerAuth validates bearer authentication configuration
func validateBearerAuth(bearer *BearerAuth, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if bearer == nil {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.bearer", fieldPrefix),
			Message: "bearer auth configuration is required when type is 'bearer'",
		})
	} else {
		if strings.TrimSpace(bearer.Token) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.bearer.token", fieldPrefix),
				Value:   bearer.Token,
				Message: "bearer token cannot be empty",
			})
		}
	}

	return errors
}

// validateBasicAuth validates basic authentication configuration
func validateBasicAuth(basic *BasicAuth, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if basic == nil {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.basic", fieldPrefix),
			Message: "basic auth configuration is required when type is 'basic'",
		})
	} else {
		if strings.TrimSpace(basic.Username) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.basic.username", fieldPrefix),
				Value:   basic.Username,
				Message: "username cannot be empty for basic auth",
			})
		}
		// Note: password can be empty for some basic auth scenarios
	}

	return errors
}

// validateAPIKeyAuth validates API key authentication configuration
func validateAPIKeyAuth(apiKey *APIKeyAuth, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if apiKey == nil {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.api_key", fieldPrefix),
			Message: "API key auth configuration is required when type is 'api_key'",
		})
	} else {
		if strings.TrimSpace(apiKey.Header) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.api_key.header", fieldPrefix),
				Value:   apiKey.Header,
				Message: "header name cannot be empty for API key auth",
			})
		}
		if strings.TrimSpace(apiKey.Value) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.api_key.value", fieldPrefix),
				Value:   apiKey.Value,
				Message: "API key value cannot be empty",
			})
		}
	}

	return errors
}

// validateOAuth2Auth validates OAuth2 authentication configuration
func validateOAuth2Auth(oauth2 *OAuth2Auth, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if oauth2 == nil {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.oauth2", fieldPrefix),
			Message: "OAuth2 auth configuration is required when type is 'oauth2'",
		})
	} else {
		errors = append(errors, validateOAuth2TokenURL(oauth2.TokenURL, fieldPrefix)...)
		errors = append(errors, validateOAuth2Credentials(oauth2, fieldPrefix)...)
	}

	return errors
}

// validateOAuth2TokenURL validates OAuth2 token URL
func validateOAuth2TokenURL(tokenURL, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if strings.TrimSpace(tokenURL) == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.oauth2.token_url", fieldPrefix),
			Value:   tokenURL,
			Message: "token URL cannot be empty for OAuth2 auth",
		})
	} else {
		if parsedURL, err := url.Parse(tokenURL); err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.oauth2.token_url", fieldPrefix),
				Value:   tokenURL,
				Message: fmt.Sprintf("invalid token URL format: %v", err),
			})
		} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.oauth2.token_url", fieldPrefix),
				Value:   tokenURL,
				Message: "OAuth2 token URL must be absolute with scheme and host",
			})
		} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.oauth2.token_url", fieldPrefix),
				Value:   tokenURL,
				Message: "OAuth2 token URL scheme must be http or https",
			})
		}
	}

	return errors
}

// validateOAuth2Credentials validates OAuth2 client credentials
func validateOAuth2Credentials(oauth2 *OAuth2Auth, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors

	if strings.TrimSpace(oauth2.ClientID) == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.oauth2.client_id", fieldPrefix),
			Value:   oauth2.ClientID,
			Message: "client ID cannot be empty for OAuth2 auth",
		})
	}

	if strings.TrimSpace(oauth2.ClientSecret) == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.oauth2.client_secret", fieldPrefix),
			Value:   oauth2.ClientSecret,
			Message: "client secret cannot be empty for OAuth2 auth",
		})
	}

	return errors
}

// validateAlerting validates alerting configuration
func validateAlerting(alerting *AlertingConfig) error {
	var errors ValidationErrors

	// Validate alert channels
	channelNames := make(map[string]bool)
	for i, channel := range alerting.Channels {
		fieldPrefix := fmt.Sprintf("alerting.channels[%d]", i)

		if strings.TrimSpace(channel.Name) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.name", fieldPrefix),
				Value:   channel.Name,
				Message: "alert channel name cannot be empty",
			})
		} else {
			if channelNames[channel.Name] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.name", fieldPrefix),
					Value:   channel.Name,
					Message: "duplicate alert channel name",
				})
			}
			channelNames[channel.Name] = true
		}

		validTypes := map[string]bool{"slack": true, "discord": true, "email": true, "webhook": true}
		if !validTypes[channel.Type] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.type", fieldPrefix),
				Value:   channel.Type,
				Message: "invalid alert channel type (supported: slack, discord, email, webhook)",
			})
		}

		// Validate channel-specific settings
		if err := validateChannelSettings(channel.Type, channel.Settings, fieldPrefix); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				errors = append(errors, validationErrs...)
			}
		}
	}

	// Validate alert rules
	for i, rule := range alerting.Rules {
		fieldPrefix := fmt.Sprintf("alerting.rules[%d]", i)

		if strings.TrimSpace(rule.Name) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.name", fieldPrefix),
				Value:   rule.Name,
				Message: "alert rule name cannot be empty",
			})
		}

		validSeverities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
		for _, severity := range rule.Severity {
			if !validSeverities[severity] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.severity", fieldPrefix),
					Value:   severity,
					Message: "invalid severity level (supported: low, medium, high, critical)",
				})
			}
		}

		// Validate that referenced channels exist
		for _, channelName := range rule.Channels {
			if !channelNames[channelName] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.channels", fieldPrefix),
					Value:   channelName,
					Message: fmt.Sprintf("referenced channel '%s' does not exist", channelName),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateChannelSettings validates channel-specific settings
func validateChannelSettings(channelType string, settings map[string]interface{}, fieldPrefix string) error {
	var errors ValidationErrors

	switch channelType {
	case "slack":
		errors = append(errors, validateWebhookURL(settings, "webhook_url", fieldPrefix, "Slack")...)
	case "discord":
		errors = append(errors, validateWebhookURL(settings, "webhook_url", fieldPrefix, "Discord")...)
	case "email":
		errors = append(errors, validateEmailSettings(settings, fieldPrefix)...)
	case "webhook":
		errors = append(errors, validateWebhookURL(settings, "url", fieldPrefix, "webhook")...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateWebhookURL validates webhook URL settings for various channel types
func validateWebhookURL(settings map[string]interface{}, urlField, fieldPrefix, channelName string) ValidationErrors {
	var errors ValidationErrors
	fieldPath := fmt.Sprintf("%s.settings.%s", fieldPrefix, urlField)

	webhookURL, ok := settings[urlField].(string)
	if !ok {
		errors = append(errors, ValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("%s channel requires %s setting", channelName, urlField),
		})
		return errors
	}

	if strings.TrimSpace(webhookURL) == "" {
		errors = append(errors, ValidationError{
			Field:   fieldPath,
			Value:   webhookURL,
			Message: fmt.Sprintf("%s webhook URL cannot be empty", channelName),
		})
		return errors
	}

	// Skip validation if it's an environment variable placeholder
	if !strings.Contains(webhookURL, "${") {
		parsedURL, err := url.Parse(webhookURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fieldPath,
				Value:   webhookURL,
				Message: fmt.Sprintf("invalid %s webhook URL format", channelName),
			})
		}
	}

	return errors
}

// validateEmailSettings validates email channel settings
func validateEmailSettings(settings map[string]interface{}, fieldPrefix string) ValidationErrors {
	var errors ValidationErrors
	requiredFields := []string{"smtp_host", "smtp_port", "from", "to"}

	for _, field := range requiredFields {
		if _, ok := settings[field]; !ok {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.settings.%s", fieldPrefix, field),
				Message: fmt.Sprintf("email channel requires %s setting", field),
			})
		}
	}

	return errors
}

// validateReporting validates reporting configuration
func validateReporting(reporting *ReportingConfig) error {
	var errors ValidationErrors

	if reporting.RetentionDays <= 0 {
		errors = append(errors, ValidationError{
			Field:   "reporting.retention_days",
			Value:   reporting.RetentionDays,
			Message: "retention days must be positive",
		})
	}

	if reporting.RetentionDays > 365 {
		errors = append(errors, ValidationError{
			Field:   "reporting.retention_days",
			Value:   reporting.RetentionDays,
			Message: "retention days cannot exceed 365",
		})
	}

	validFormats := map[string]bool{"json": true, "csv": true, "yaml": true}
	if !validFormats[reporting.ExportFormat] {
		errors = append(errors, ValidationError{
			Field:   "reporting.export_format",
			Value:   reporting.ExportFormat,
			Message: "invalid export format (supported: json, csv, yaml)",
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
