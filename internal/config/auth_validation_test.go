package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAuth(t *testing.T) {
	tests := []struct {
		name        string
		auth        *AuthConfig
		expectError bool
		errorFields []string
	}{
		{
			name: "valid bearer auth",
			auth: &AuthConfig{
				Type: AuthTypeBearer,
				Bearer: &BearerAuth{
					Token: "test-token",
				},
			},
			expectError: false,
		},
		{
			name: "bearer auth without config",
			auth: &AuthConfig{
				Type: AuthTypeBearer,
			},
			expectError: true,
			errorFields: []string{"auth.bearer"},
		},
		{
			name: "bearer auth with empty token",
			auth: &AuthConfig{
				Type: AuthTypeBearer,
				Bearer: &BearerAuth{
					Token: "",
				},
			},
			expectError: true,
			errorFields: []string{"auth.bearer.token"},
		},
		{
			name: "valid basic auth",
			auth: &AuthConfig{
				Type: AuthTypeBasic,
				Basic: &BasicAuth{
					Username: "testuser",
					Password: "testpass",
				},
			},
			expectError: false,
		},
		{
			name: "basic auth without config",
			auth: &AuthConfig{
				Type: AuthTypeBasic,
			},
			expectError: true,
			errorFields: []string{"auth.basic"},
		},
		{
			name: "basic auth with empty username",
			auth: &AuthConfig{
				Type: AuthTypeBasic,
				Basic: &BasicAuth{
					Username: "",
					Password: "testpass",
				},
			},
			expectError: true,
			errorFields: []string{"auth.basic.username"},
		},
		{
			name: "basic auth with empty password (allowed)",
			auth: &AuthConfig{
				Type: AuthTypeBasic,
				Basic: &BasicAuth{
					Username: "testuser",
					Password: "",
				},
			},
			expectError: false,
		},
		{
			name: "valid api key auth",
			auth: &AuthConfig{
				Type: AuthTypeAPIKey,
				APIKey: &APIKeyAuth{
					Header: "X-API-Key",
					Value:  "secret-key",
				},
			},
			expectError: false,
		},
		{
			name: "api key auth without config",
			auth: &AuthConfig{
				Type: AuthTypeAPIKey,
			},
			expectError: true,
			errorFields: []string{"auth.api_key"},
		},
		{
			name: "api key auth with empty header",
			auth: &AuthConfig{
				Type: AuthTypeAPIKey,
				APIKey: &APIKeyAuth{
					Header: "",
					Value:  "secret-key",
				},
			},
			expectError: true,
			errorFields: []string{"auth.api_key.header"},
		},
		{
			name: "api key auth with empty value",
			auth: &AuthConfig{
				Type: AuthTypeAPIKey,
				APIKey: &APIKeyAuth{
					Header: "X-API-Key",
					Value:  "",
				},
			},
			expectError: true,
			errorFields: []string{"auth.api_key.value"},
		},
		{
			name: "valid oauth2 auth",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				},
			},
			expectError: false,
		},
		{
			name: "oauth2 auth without config",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
			},
			expectError: true,
			errorFields: []string{"auth.oauth2"},
		},
		{
			name: "oauth2 auth with empty token URL",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				},
			},
			expectError: true,
			errorFields: []string{"auth.oauth2.token_url"},
		},
		{
			name: "oauth2 auth with invalid token URL",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "not-a-url",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				},
			},
			expectError: true,
			errorFields: []string{"auth.oauth2.token_url"},
		},
		{
			name: "oauth2 auth with empty client ID",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientID:     "",
					ClientSecret: "client-secret",
				},
			},
			expectError: true,
			errorFields: []string{"auth.oauth2.client_id"},
		},
		{
			name: "oauth2 auth with empty client secret",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientID:     "client-id",
					ClientSecret: "",
				},
			},
			expectError: true,
			errorFields: []string{"auth.oauth2.client_secret"},
		},
		{
			name: "oauth2 auth with scopes and extra params",
			auth: &AuthConfig{
				Type: AuthTypeOAuth2,
				OAuth2: &OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{"read", "write"},
					ExtraParams: map[string]string{
						"audience": "api.example.com",
					},
				},
			},
			expectError: false,
		},
		{
			name: "none auth type",
			auth: &AuthConfig{
				Type: AuthTypeNone,
			},
			expectError: false,
		},
		{
			name: "invalid auth type",
			auth: &AuthConfig{
				Type: "invalid",
			},
			expectError: true,
			errorFields: []string{"auth.type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuth(tt.auth, "auth")

			if tt.expectError {
				assert.Error(t, err)
				if len(tt.errorFields) > 0 {
					validationErrs, ok := err.(ValidationErrors)
					assert.True(t, ok, "Expected ValidationErrors type")

					// Check that all expected error fields are present
					errorFieldsMap := make(map[string]bool)
					for _, validationErr := range validationErrs {
						errorFieldsMap[validationErr.Field] = true
					}

					for _, expectedField := range tt.errorFields {
						assert.True(t, errorFieldsMap[expectedField],
							"Expected error field %s not found in validation errors", expectedField)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEndpointWithAuth(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    *EndpointConfig
		expectError bool
	}{
		{
			name: "endpoint with valid bearer auth",
			endpoint: &EndpointConfig{
				ID:       "test-endpoint",
				URL:      "https://api.example.com/test",
				Method:   "GET",
				Interval: 5 * 60 * 1000000000, // 5 minutes in nanoseconds
				Auth: &AuthConfig{
					Type: AuthTypeBearer,
					Bearer: &BearerAuth{
						Token: "test-token",
					},
				},
			},
			expectError: false,
		},
		{
			name: "endpoint with invalid auth",
			endpoint: &EndpointConfig{
				ID:       "test-endpoint",
				URL:      "https://api.example.com/test",
				Method:   "GET",
				Interval: 5 * 60 * 1000000000, // 5 minutes in nanoseconds
				Auth: &AuthConfig{
					Type: AuthTypeBearer,
					// Missing Bearer config
				},
			},
			expectError: true,
		},
		{
			name: "endpoint without auth",
			endpoint: &EndpointConfig{
				ID:       "test-endpoint",
				URL:      "https://api.example.com/test",
				Method:   "GET",
				Interval: 5 * 60 * 1000000000, // 5 minutes in nanoseconds
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoint(tt.endpoint, "endpoints[0]")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
