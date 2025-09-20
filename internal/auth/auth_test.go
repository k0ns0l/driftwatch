package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_CreateAuthenticator(t *testing.T) {
	logger, err := logging.NewLogger(logging.LoggerConfig{
		Level:  logging.LogLevelDebug,
		Format: logging.LogFormatText,
	})
	require.NoError(t, err)
	manager := NewManager(logger)

	tests := []struct {
		name         string
		authConfig   *config.AuthConfig
		expectedType config.AuthType
		expectError  bool
	}{
		{
			name:         "nil config returns none auth",
			authConfig:   nil,
			expectedType: config.AuthTypeNone,
			expectError:  false,
		},
		{
			name: "none auth type",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeNone,
			},
			expectedType: config.AuthTypeNone,
			expectError:  false,
		},
		{
			name: "bearer auth with token",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeBearer,
				Bearer: &config.BearerAuth{
					Token: "test-token",
				},
			},
			expectedType: config.AuthTypeBearer,
			expectError:  false,
		},
		{
			name: "bearer auth without config",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeBearer,
			},
			expectError: true,
		},
		{
			name: "basic auth with credentials",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeBasic,
				Basic: &config.BasicAuth{
					Username: "user",
					Password: "pass",
				},
			},
			expectedType: config.AuthTypeBasic,
			expectError:  false,
		},
		{
			name: "api key auth with header",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeAPIKey,
				APIKey: &config.APIKeyAuth{
					Header: "X-API-Key",
					Value:  "secret-key",
				},
			},
			expectedType: config.AuthTypeAPIKey,
			expectError:  false,
		},
		{
			name: "oauth2 auth with config",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeOAuth2,
				OAuth2: &config.OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientID:     "client-id",
					ClientSecret: "client-secret",
				},
			},
			expectedType: config.AuthTypeOAuth2,
			expectError:  false,
		},
		{
			name: "unsupported auth type",
			authConfig: &config.AuthConfig{
				Type: "unsupported",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := manager.CreateAuthenticator(tt.authConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
				assert.Equal(t, tt.expectedType, auth.GetType())
			}
		})
	}
}

func TestNoneAuth(t *testing.T) {
	auth := &NoneAuth{}

	req, err := http.NewRequest("GET", "https://example.com", nil)
	require.NoError(t, err)

	// Should not modify the request
	originalHeaders := len(req.Header)
	err = auth.ApplyAuth(req)
	assert.NoError(t, err)
	assert.Equal(t, originalHeaders, len(req.Header))

	// Validation should pass
	assert.NoError(t, auth.Validate())
	assert.Equal(t, config.AuthTypeNone, auth.GetType())
}

func TestBearerAuth(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		auth := NewBearerAuth("test-token-123")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.NoError(t, err)
		assert.Equal(t, "Bearer test-token-123", req.Header.Get("Authorization"))

		assert.NoError(t, auth.Validate())
		assert.Equal(t, config.AuthTypeBearer, auth.GetType())
	})

	t.Run("empty token", func(t *testing.T) {
		auth := NewBearerAuth("")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.Error(t, err)

		err = auth.Validate()
		assert.Error(t, err)
	})
}

func TestBasicAuth(t *testing.T) {
	t.Run("valid credentials", func(t *testing.T) {
		auth := NewBasicAuth("testuser", "testpass")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.NoError(t, err)

		authHeader := req.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(authHeader, "Basic "))

		// Decode and verify credentials
		encoded := strings.TrimPrefix(authHeader, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		assert.Equal(t, "testuser:testpass", string(decoded))

		assert.NoError(t, auth.Validate())
		assert.Equal(t, config.AuthTypeBasic, auth.GetType())
	})

	t.Run("empty username", func(t *testing.T) {
		auth := NewBasicAuth("", "testpass")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.Error(t, err)

		err = auth.Validate()
		assert.Error(t, err)
	})
}

func TestAPIKeyAuth(t *testing.T) {
	t.Run("valid api key", func(t *testing.T) {
		auth := NewAPIKeyAuth("X-API-Key", "secret-key-123")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.NoError(t, err)
		assert.Equal(t, "secret-key-123", req.Header.Get("X-API-Key"))

		assert.NoError(t, auth.Validate())
		assert.Equal(t, config.AuthTypeAPIKey, auth.GetType())
	})

	t.Run("empty header name", func(t *testing.T) {
		auth := NewAPIKeyAuth("", "secret-key-123")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.Error(t, err)

		err = auth.Validate()
		assert.Error(t, err)
	})

	t.Run("empty api key value", func(t *testing.T) {
		auth := NewAPIKeyAuth("X-API-Key", "")

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.Error(t, err)

		err = auth.Validate()
		assert.Error(t, err)
	})
}

func TestOAuth2Auth(t *testing.T) {
	t.Run("successful token fetch and apply", func(t *testing.T) {
		// Create mock OAuth2 server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			// Parse form data
			err := r.ParseForm()
			require.NoError(t, err)
			assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			assert.Equal(t, "test-client", r.Form.Get("client_id"))
			assert.Equal(t, "test-secret", r.Form.Get("client_secret"))

			// Return token response
			response := OAuth2TokenResponse{
				AccessToken: "access-token-123",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		logger, err := logging.NewLogger(logging.LoggerConfig{
			Level:  logging.LogLevelDebug,
			Format: logging.LogFormatText,
		})
		require.NoError(t, err)

		oauthConfig := &config.OAuth2Auth{
			TokenURL:     server.URL,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}

		auth := NewOAuth2Auth(oauthConfig, nil, logger)

		// Validate configuration
		assert.NoError(t, auth.Validate())
		assert.Equal(t, config.AuthTypeOAuth2, auth.GetType())

		// Apply auth to request
		req, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.NoError(t, err)
		assert.Equal(t, "Bearer access-token-123", req.Header.Get("Authorization"))

		// Second request should use cached token
		req2, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req2)
		assert.NoError(t, err)
		assert.Equal(t, "Bearer access-token-123", req2.Header.Get("Authorization"))
	})

	t.Run("token fetch with scopes and extra params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := r.ParseForm()
			require.NoError(t, err)
			assert.Equal(t, "read write", r.Form.Get("scope"))
			assert.Equal(t, "extra-value", r.Form.Get("extra_param"))

			response := OAuth2TokenResponse{
				AccessToken: "scoped-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
				Scope:       "read write",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		oauthConfig := &config.OAuth2Auth{
			TokenURL:     server.URL,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			Scopes:       []string{"read", "write"},
			ExtraParams: map[string]string{
				"extra_param": "extra-value",
			},
		}

		auth := NewOAuth2Auth(oauthConfig, nil, nil)

		req, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.NoError(t, err)
		assert.Equal(t, "Bearer scoped-token", req.Header.Get("Authorization"))
	})

	t.Run("oauth2 error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OAuth2TokenResponse{
				Error:     "invalid_client",
				ErrorDesc: "Client authentication failed",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		oauthConfig := &config.OAuth2Auth{
			TokenURL:     server.URL,
			ClientID:     "invalid-client",
			ClientSecret: "invalid-secret",
		}

		auth := NewOAuth2Auth(oauthConfig, nil, nil)

		req, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)

		err = auth.ApplyAuth(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_client")
	})

	t.Run("validation errors", func(t *testing.T) {
		tests := []struct {
			name   string
			config *config.OAuth2Auth
		}{
			{
				name: "missing token URL",
				config: &config.OAuth2Auth{
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
			{
				name: "missing client ID",
				config: &config.OAuth2Auth{
					TokenURL:     "https://auth.example.com/token",
					ClientSecret: "secret",
				},
			},
			{
				name: "missing client secret",
				config: &config.OAuth2Auth{
					TokenURL: "https://auth.example.com/token",
					ClientID: "client",
				},
			},
			{
				name: "invalid token URL",
				config: &config.OAuth2Auth{
					TokenURL:     "not-a-url",
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				auth := NewOAuth2Auth(tt.config, nil, nil)
				err := auth.Validate()
				assert.Error(t, err)
			})
		}
	})

	t.Run("token expiration and refresh", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			response := OAuth2TokenResponse{
				AccessToken: "token-" + string(rune(callCount)),
				TokenType:   "Bearer",
				ExpiresIn:   1, // 1 second expiration
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		oauthConfig := &config.OAuth2Auth{
			TokenURL:     server.URL,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}

		auth := NewOAuth2Auth(oauthConfig, nil, nil)

		// First request
		req1, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)
		err = auth.ApplyAuth(req1)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Wait for token to expire
		time.Sleep(2 * time.Second)

		// Second request should fetch new token
		req2, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)
		err = auth.ApplyAuth(req2)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("clear cached token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OAuth2TokenResponse{
				AccessToken: "test-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		oauthConfig := &config.OAuth2Auth{
			TokenURL:     server.URL,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}

		auth := NewOAuth2Auth(oauthConfig, nil, nil)

		// Fetch token
		req, err := http.NewRequest("GET", "https://api.example.com", nil)
		require.NoError(t, err)
		err = auth.ApplyAuth(req)
		assert.NoError(t, err)

		// Verify token is cached
		assert.NotNil(t, auth.cachedToken)

		// Clear cache
		auth.ClearCachedToken()
		assert.Nil(t, auth.cachedToken)
	})
}
