package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHTTPClientIntegration tests the HTTP client with a more realistic scenario
func TestHTTPClientIntegration(t *testing.T) {
	// Create a test server that simulates different API behaviors
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		switch r.URL.Path {
		case "/healthy":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy", "timestamp": "2023-01-01T00:00:00Z"}`))

		case "/retry-then-success":
			if requestCount <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server temporarily unavailable"))
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"message": "success after retries"}`))
			}

		case "/rate-limited":
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limit exceeded"))

		case "/not-found":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not found"))

		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad request"))
		}
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetTimeout(5 * time.Second)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 3,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffExponential,
		Jitter:     false,
	})

	// Test 1: Successful request
	t.Run("HealthyEndpoint", func(t *testing.T) {
		req, err := NewJSONRequest("GET", server.URL+"/healthy", nil, map[string]string{
			"Authorization": "Bearer test-token",
		})
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", response.StatusCode)
		}

		if !strings.Contains(string(response.Body), "healthy") {
			t.Errorf("Expected response to contain 'healthy', got: %s", string(response.Body))
		}

		if response.Attempt != 1 {
			t.Errorf("Expected 1 attempt, got %d", response.Attempt)
		}
	})

	// Test 2: Retry scenario
	t.Run("RetryThenSuccess", func(t *testing.T) {
		requestCount = 0 // Reset counter

		req, err := NewRequest("GET", server.URL+"/retry-then-success", nil, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.StatusCode != http.StatusOK {
			t.Errorf("Expected final status 200, got %d", response.StatusCode)
		}

		if response.Attempt != 3 {
			t.Errorf("Expected 3 attempts, got %d", response.Attempt)
		}

		if !strings.Contains(string(response.Body), "success after retries") {
			t.Errorf("Expected success message, got: %s", string(response.Body))
		}
	})

	// Test 3: Non-retryable error
	t.Run("NonRetryableError", func(t *testing.T) {
		req, err := NewRequest("GET", server.URL+"/not-found", nil, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", response.StatusCode)
		}

		if response.Attempt != 1 {
			t.Errorf("Expected 1 attempt (no retries), got %d", response.Attempt)
		}
	})

	// Test 4: Retryable error that exhausts retries
	t.Run("RetryableErrorExhausted", func(t *testing.T) {
		req, err := NewRequest("GET", server.URL+"/rate-limited", nil, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", response.StatusCode)
		}

		if response.Attempt != 4 { // 1 initial + 3 retries
			t.Errorf("Expected 4 attempts, got %d", response.Attempt)
		}
	})

	// Test 5: Verify metrics
	t.Run("VerifyMetrics", func(t *testing.T) {
		metrics := client.GetMetrics()

		if metrics.TotalRequests < 4 {
			t.Errorf("Expected at least 4 total requests, got %d", metrics.TotalRequests)
		}

		if metrics.SuccessfulReqs < 2 {
			t.Errorf("Expected at least 2 successful requests, got %d", metrics.SuccessfulReqs)
		}

		if metrics.RetryAttempts < 5 { // 2 from retry test + 3 from rate limit test
			t.Errorf("Expected at least 5 retry attempts, got %d", metrics.RetryAttempts)
		}

		if metrics.AverageRespTime <= 0 {
			t.Error("Expected positive average response time")
		}
	})
}

// TestHTTPClientWithRealWorldScenario tests a more complex scenario
func TestHTTPClientWithRealWorldScenario(t *testing.T) {
	// Simulate an API that has intermittent issues
	validTokenCallCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Invalid token"}`))
			return
		}

		validTokenCallCount++

		// Simulate intermittent server issues - fail on first call, succeed on second
		if validTokenCallCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "Service temporarily unavailable"}`))
			return
		}

		// Success response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", fmt.Sprintf("req-%d", validTokenCallCount))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{
			"data": {
				"id": 123,
				"name": "Test API",
				"version": "1.0.0"
			},
			"meta": {
				"timestamp": "2023-01-01T00:00:00Z",
				"request_id": "req-%d"
			}
		}`, validTokenCallCount)))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetTimeout(2 * time.Second)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 2,
		Delay:      50 * time.Millisecond,
		Backoff:    BackoffExponential,
		Jitter:     true,
	})

	// Test authenticated request with retry
	req, err := NewJSONRequest("GET", server.URL+"/api/v1/data", nil, map[string]string{
		"Authorization": "Bearer valid-token",
		"Accept":        "application/json",
		"User-Agent":    "driftwatch/1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", response.StatusCode)
	}

	if !strings.Contains(string(response.Body), "Test API") {
		t.Errorf("Expected response to contain API data, got: %s", string(response.Body))
	}

	// Verify response headers
	if response.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", response.Headers.Get("Content-Type"))
	}

	// Verify timing
	if response.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}

	// Test unauthorized request (should not retry)
	req2, err := NewJSONRequest("GET", server.URL+"/api/v1/data", nil, map[string]string{
		"Authorization": "Bearer invalid-token",
	})
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", response2.StatusCode)
	}

	if response2.Attempt != 1 {
		t.Errorf("Expected 1 attempt for auth error, got %d", response2.Attempt)
	}
}
