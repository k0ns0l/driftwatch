// Package auth provides authentication functionality for HTTP requests
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
)

// Authenticator defines the interface for authentication providers
type Authenticator interface {
	// ApplyAuth applies authentication to an HTTP request
	ApplyAuth(req *http.Request) error
	// Validate validates the authentication configuration
	Validate() error
	// GetType returns the authentication type
	GetType() config.AuthType
}

// Manager manages authentication for HTTP requests
type Manager struct {
	logger *logging.Logger
	client *http.Client
}

// NewManager creates a new authentication manager
func NewManager(logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}

	return &Manager{
		logger: logger.WithComponent("auth_manager"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateAuthenticator creates an authenticator based on the auth configuration
func (m *Manager) CreateAuthenticator(authConfig *config.AuthConfig) (Authenticator, error) {
	if authConfig == nil {
		return &NoneAuth{}, nil
	}

	switch authConfig.Type {
	case config.AuthTypeNone:
		return &NoneAuth{}, nil
	case config.AuthTypeBearer:
		if authConfig.Bearer == nil {
			return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_CONFIG_MISSING", "bearer auth configuration is missing").
				WithSeverity(errors.SeverityHigh).
				WithGuidance("Provide bearer token configuration")
		}
		return NewBearerAuth(authConfig.Bearer.Token), nil
	case config.AuthTypeBasic:
		if authConfig.Basic == nil {
			return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_CONFIG_MISSING", "basic auth configuration is missing").
				WithSeverity(errors.SeverityHigh).
				WithGuidance("Provide username and password for basic authentication")
		}
		return NewBasicAuth(authConfig.Basic.Username, authConfig.Basic.Password), nil
	case config.AuthTypeAPIKey:
		if authConfig.APIKey == nil {
			return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_CONFIG_MISSING", "API key auth configuration is missing").
				WithSeverity(errors.SeverityHigh).
				WithGuidance("Provide header name and API key value")
		}
		return NewAPIKeyAuth(authConfig.APIKey.Header, authConfig.APIKey.Value), nil
	case config.AuthTypeOAuth2:
		if authConfig.OAuth2 == nil {
			return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_CONFIG_MISSING", "OAuth2 auth configuration is missing").
				WithSeverity(errors.SeverityHigh).
				WithGuidance("Provide OAuth2 client credentials configuration")
		}
		return NewOAuth2Auth(authConfig.OAuth2, m.client, m.logger), nil
	default:
		return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_TYPE_UNSUPPORTED", fmt.Sprintf("unsupported auth type: %s", authConfig.Type)).
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Use one of: none, bearer, basic, api_key, oauth2")
	}
}

// NoneAuth represents no authentication
type NoneAuth struct{}

func (a *NoneAuth) ApplyAuth(req *http.Request) error {
	// No authentication to apply
	return nil
}

func (a *NoneAuth) Validate() error {
	// No validation needed for no auth
	return nil
}

func (a *NoneAuth) GetType() config.AuthType {
	return config.AuthTypeNone
}

// BearerAuth represents Bearer token authentication
type BearerAuth struct {
	token string
}

// NewBearerAuth creates a new Bearer token authenticator
func NewBearerAuth(token string) *BearerAuth {
	return &BearerAuth{token: token}
}

func (a *BearerAuth) ApplyAuth(req *http.Request) error {
	if a.token == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_EMPTY", "bearer token is empty").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid bearer token")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
	return nil
}

func (a *BearerAuth) Validate() error {
	if a.token == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_EMPTY", "bearer token is required").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid bearer token")
	}
	return nil
}

func (a *BearerAuth) GetType() config.AuthType {
	return config.AuthTypeBearer
}

// BasicAuth represents HTTP Basic authentication
type BasicAuth struct {
	username string
	password string
}

// NewBasicAuth creates a new Basic authentication authenticator
func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{
		username: username,
		password: password,
	}
}

func (a *BasicAuth) ApplyAuth(req *http.Request) error {
	if a.username == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_USERNAME_EMPTY", "username is empty").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid username for basic authentication")
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.username, a.password)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", credentials))
	return nil
}

func (a *BasicAuth) Validate() error {
	if a.username == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_USERNAME_EMPTY", "username is required for basic authentication").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid username")
	}
	return nil
}

func (a *BasicAuth) GetType() config.AuthType {
	return config.AuthTypeBasic
}

// APIKeyAuth represents API key authentication via custom headers
type APIKeyAuth struct {
	header string
	value  string
}

// NewAPIKeyAuth creates a new API key authenticator
func NewAPIKeyAuth(header, value string) *APIKeyAuth {
	return &APIKeyAuth{
		header: header,
		value:  value,
	}
}

func (a *APIKeyAuth) ApplyAuth(req *http.Request) error {
	if a.header == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_HEADER_EMPTY", "API key header name is empty").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid header name for API key authentication")
	}

	if a.value == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_VALUE_EMPTY", "API key value is empty").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid API key value")
	}

	req.Header.Set(a.header, a.value)
	return nil
}

func (a *APIKeyAuth) Validate() error {
	if a.header == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_HEADER_EMPTY", "header name is required for API key authentication").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid header name (e.g., 'X-API-Key')")
	}

	if a.value == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_VALUE_EMPTY", "API key value is required").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid API key value")
	}

	return nil
}

func (a *APIKeyAuth) GetType() config.AuthType {
	return config.AuthTypeAPIKey
}

// OAuth2Auth represents OAuth 2.0 client credentials flow authentication
type OAuth2Auth struct {
	config      *config.OAuth2Auth
	client      *http.Client
	logger      *logging.Logger
	cachedToken *OAuth2Token
}

// OAuth2Token represents an OAuth 2.0 access token
type OAuth2Token struct {
	ExpiresAt   time.Time `json:"-"`
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope,omitempty"`
	ExpiresIn   int       `json:"expires_in"`
}

// OAuth2TokenResponse represents the response from an OAuth 2.0 token endpoint
type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewOAuth2Auth creates a new OAuth 2.0 authenticator
func NewOAuth2Auth(config *config.OAuth2Auth, client *http.Client, logger *logging.Logger) *OAuth2Auth {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}

	return &OAuth2Auth{
		config: config,
		client: client,
		logger: logger.WithComponent("oauth2_auth"),
	}
}

func (a *OAuth2Auth) ApplyAuth(req *http.Request) error {
	token, err := a.getValidToken()
	if err != nil {
		return errors.WrapError(err, errors.ErrorTypeAuth, "AUTH_TOKEN_FETCH", "failed to get OAuth2 token").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 configuration and credentials")
	}

	// Apply the token to the request
	authHeader := fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)
	if token.TokenType == "" {
		authHeader = fmt.Sprintf("Bearer %s", token.AccessToken)
	}
	req.Header.Set("Authorization", authHeader)

	return nil
}

func (a *OAuth2Auth) Validate() error {
	if a.config.TokenURL == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_URL_EMPTY", "OAuth2 token URL is required").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid OAuth2 token endpoint URL")
	}

	if a.config.ClientID == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_CLIENT_ID_EMPTY", "OAuth2 client ID is required").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid OAuth2 client ID")
	}

	if a.config.ClientSecret == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_CLIENT_SECRET_EMPTY", "OAuth2 client secret is required").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid OAuth2 client secret")
	}

	// Validate token URL format
	if parsedURL, err := url.Parse(a.config.TokenURL); err != nil {
		return errors.WrapError(err, errors.ErrorTypeAuth, "AUTH_TOKEN_URL_INVALID", "OAuth2 token URL is invalid").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid URL for the OAuth2 token endpoint")
	} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_URL_INVALID", "OAuth2 token URL must be absolute with scheme and host").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Provide a valid absolute URL (e.g., https://auth.example.com/token)")
	} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_URL_INVALID", "OAuth2 token URL scheme must be http or https").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Use http:// or https:// scheme for the token URL")
	}

	return nil
}

func (a *OAuth2Auth) GetType() config.AuthType {
	return config.AuthTypeOAuth2
}

// getValidToken returns a valid OAuth2 token, fetching a new one if necessary
func (a *OAuth2Auth) getValidToken() (*OAuth2Token, error) {
	// Check if we have a cached token that's still valid
	if a.cachedToken != nil && time.Now().Before(a.cachedToken.ExpiresAt.Add(-30*time.Second)) {
		a.logger.Debug("Using cached OAuth2 token")
		return a.cachedToken, nil
	}

	a.logger.Debug("Fetching new OAuth2 token", "token_url", a.config.TokenURL)

	// Fetch a new token
	token, err := a.fetchToken()
	if err != nil {
		return nil, err
	}

	// Cache the token
	a.cachedToken = token
	a.logger.Debug("OAuth2 token fetched and cached", "expires_at", token.ExpiresAt)

	return token, nil
}

// fetchToken fetches a new OAuth2 token using client credentials flow
func (a *OAuth2Auth) fetchToken() (*OAuth2Token, error) {
	// Prepare the request body
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)

	if len(a.config.Scopes) > 0 {
		data.Set("scope", strings.Join(a.config.Scopes, " "))
	}

	// Add extra parameters
	for key, value := range a.config.ExtraParams {
		data.Set(key, value)
	}

	// Create the request
	req, err := http.NewRequestWithContext(context.Background(), "POST", a.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.WrapError(err, errors.ErrorTypeAuth, "AUTH_REQUEST_CREATE", "failed to create OAuth2 token request").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 token URL and parameters")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Make the request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, errors.WrapError(err, errors.ErrorTypeAuth, "AUTH_REQUEST_FAILED", "OAuth2 token request failed").
			WithSeverity(errors.SeverityHigh).
			WithRecoverable(true).
			WithGuidance("Check network connectivity and OAuth2 server availability")
	}
	defer resp.Body.Close()

	// Parse the response
	var tokenResp OAuth2TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, errors.WrapError(err, errors.ErrorTypeAuth, "AUTH_RESPONSE_PARSE", "failed to parse OAuth2 token response").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 server response format")
	}

	// Check for errors in the response
	if tokenResp.Error != "" {
		errorMsg := fmt.Sprintf("OAuth2 error: %s", tokenResp.Error)
		if tokenResp.ErrorDesc != "" {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, tokenResp.ErrorDesc)
		}
		return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_OAUTH2_ERROR", errorMsg).
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 client credentials and configuration")
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_HTTP_ERROR", fmt.Sprintf("OAuth2 token request failed with status %d", resp.StatusCode)).
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 endpoint URL and client credentials")
	}

	// Validate required fields
	if tokenResp.AccessToken == "" {
		return nil, errors.NewError(errors.ErrorTypeAuth, "AUTH_TOKEN_EMPTY", "OAuth2 response missing access token").
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check OAuth2 server configuration and response format")
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		// Default to 1 hour if no expiration provided
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	token := &OAuth2Token{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		ExpiresIn:   tokenResp.ExpiresIn,
		Scope:       tokenResp.Scope,
		ExpiresAt:   expiresAt,
	}

	return token, nil
}

// ClearCachedToken clears the cached OAuth2 token, forcing a refresh on next use
func (a *OAuth2Auth) ClearCachedToken() {
	a.cachedToken = nil
	a.logger.Debug("OAuth2 cached token cleared")
}
