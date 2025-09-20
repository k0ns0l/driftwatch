package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationIntegration(t *testing.T) {
	t.Run("Bearer token authentication", func(t *testing.T) {
		// Create a test server that checks for Bearer token
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer test-token-123" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		}))
		defer server.Close()

		// Create auth config
		authConfig := &config.AuthConfig{
			Type: config.AuthTypeBearer,
			Bearer: &config.BearerAuth{
				Token: "test-token-123",
			},
		}

		// Create authenticator
		manager := NewManager(nil)
		authenticator, err := manager.CreateAuthenticator(authConfig)
		require.NoError(t, err)

		// Create request and apply auth
		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		err = authenticator.ApplyAuth(req)
		require.NoError(t, err)

		// Make the request
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify success
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Basic authentication", func(t *testing.T) {
		// Create a test server that checks for Basic auth
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok || username != "testuser" || password != "testpass" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		}))
		defer server.Close()

		// Create auth config
		authConfig := &config.AuthConfig{
			Type: config.AuthTypeBasic,
			Basic: &config.BasicAuth{
				Username: "testuser",
				Password: "testpass",
			},
		}

		// Create authenticator
		manager := NewManager(nil)
		authenticator, err := manager.CreateAuthenticator(authConfig)
		require.NoError(t, err)

		// Create request and apply auth
		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		err = authenticator.ApplyAuth(req)
		require.NoError(t, err)

		// Make the request
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify success
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("API Key authentication", func(t *testing.T) {
		// Create a test server that checks for API key header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "secret-api-key-123" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		}))
		defer server.Close()

		// Create auth config
		authConfig := &config.AuthConfig{
			Type: config.AuthTypeAPIKey,
			APIKey: &config.APIKeyAuth{
				Header: "X-API-Key",
				Value:  "secret-api-key-123",
			},
		}

		// Create authenticator
		manager := NewManager(nil)
		authenticator, err := manager.CreateAuthenticator(authConfig)
		require.NoError(t, err)

		// Create request and apply auth
		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		err = authenticator.ApplyAuth(req)
		require.NoError(t, err)

		// Make the request
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify success
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
