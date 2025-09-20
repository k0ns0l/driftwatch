package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/logging"
)

func TestNewHTTPClient(t *testing.T) {
	logger, _ := logging.NewLogger(logging.DefaultLoggerConfig())
	client := NewHTTPClient(logger)

	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}

	if client.client.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout of 30s, got %v", client.client.Timeout)
	}

	if client.retryPolicy.MaxRetries != 3 {
		t.Errorf("Expected default max retries of 3, got %d", client.retryPolicy.MaxRetries)
	}

	if client.retryPolicy.Backoff != BackoffExponential {
		t.Errorf("Expected default backoff strategy of exponential, got %s", client.retryPolicy.Backoff)
	}
}

func TestNewHTTPClientWithNilLogger(t *testing.T) {
	client := NewHTTPClient(nil)
	if client == nil {
		t.Fatal("NewHTTPClient returned nil with nil logger")
	}
	if client.logger == nil {
		t.Error("Expected default logger to be set when nil logger provided")
	}
}

func TestHTTPClient_SetTimeout(t *testing.T) {
	client := NewHTTPClient(nil)
	timeout := 10 * time.Second

	client.SetTimeout(timeout)

	if client.client.Timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, client.client.Timeout)
	}
}

func TestHTTPClient_SetRetryPolicy(t *testing.T) {
	client := NewHTTPClient(nil)
	policy := RetryPolicy{
		MaxRetries: 5,
		Delay:      2 * time.Second,
		Backoff:    BackoffLinear,
		Jitter:     false,
	}

	client.SetRetryPolicy(policy)

	if client.retryPolicy.MaxRetries != policy.MaxRetries {
		t.Errorf("Expected max retries %d, got %d", policy.MaxRetries, client.retryPolicy.MaxRetries)
	}
	if client.retryPolicy.Delay != policy.Delay {
		t.Errorf("Expected delay %v, got %v", policy.Delay, client.retryPolicy.Delay)
	}
	if client.retryPolicy.Backoff != policy.Backoff {
		t.Errorf("Expected backoff %s, got %s", policy.Backoff, client.retryPolicy.Backoff)
	}
	if client.retryPolicy.Jitter != policy.Jitter {
		t.Errorf("Expected jitter %v, got %v", policy.Jitter, client.retryPolicy.Jitter)
	}
}

func TestHTTPClient_DoSuccessful(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}

	if string(response.Body) != `{"message": "success"}` {
		t.Errorf("Expected body %s, got %s", `{"message": "success"}`, string(response.Body))
	}

	if response.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}

	if response.Attempt != 1 {
		t.Errorf("Expected attempt 1, got %d", response.Attempt)
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", metrics.TotalRequests)
	}
	if metrics.SuccessfulReqs != 1 {
		t.Errorf("Expected 1 successful request, got %d", metrics.SuccessfulReqs)
	}
	if metrics.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", metrics.FailedRequests)
	}
}

func TestHTTPClient_DoWithRequestBody(t *testing.T) {
	requestBody := `{"test": "data"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		if string(body) != requestBody {
			t.Errorf("Expected request body %s, got %s", requestBody, string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	req, err := http.NewRequest("POST", server.URL, strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}
}

func TestHTTPClient_DoRetryOnServerError(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		}
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	// Set faster retry for testing
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 3,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}

	if response.Attempt != 3 {
		t.Errorf("Expected attempt 3, got %d", response.Attempt)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 server attempts, got %d", attemptCount)
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.RetryAttempts != 2 {
		t.Errorf("Expected 2 retry attempts, got %d", metrics.RetryAttempts)
	}
}

func TestHTTPClient_DoRetryExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 2,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request should not fail, got: %v", err)
	}

	if response.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, response.StatusCode)
	}

	if response.Attempt != 3 {
		t.Errorf("Expected attempt 3, got %d", response.Attempt)
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
	if metrics.RetryAttempts != 2 {
		t.Errorf("Expected 2 retry attempts, got %d", metrics.RetryAttempts)
	}
}

func TestHTTPClient_DoNetworkError(t *testing.T) {
	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 2,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	// Use invalid URL to trigger network error
	req, err := http.NewRequest("GET", "http://invalid-host-that-does-not-exist.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}

	if response != nil {
		t.Error("Expected nil response on network error")
	}

	if !strings.Contains(err.Error(), "request failed after") {
		t.Errorf("Expected retry error message, got: %v", err)
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
	if metrics.RetryAttempts != 2 {
		t.Errorf("Expected 2 retry attempts, got %d", metrics.RetryAttempts)
	}
}

func TestHTTPClient_DoTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetTimeout(50 * time.Millisecond)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 1,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if response != nil {
		t.Error("Expected nil response on timeout")
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
}

func TestHTTPClient_CalculateDelayFixed(t *testing.T) {
	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		Delay:   1 * time.Second,
		Backoff: BackoffFixed,
		Jitter:  false,
	})

	delay := client.calculateDelay(0)
	if delay != 1*time.Second {
		t.Errorf("Expected delay of 1s, got %v", delay)
	}

	delay = client.calculateDelay(2)
	if delay != 1*time.Second {
		t.Errorf("Expected delay of 1s, got %v", delay)
	}
}

func TestHTTPClient_CalculateDelayLinear(t *testing.T) {
	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		Delay:   1 * time.Second,
		Backoff: BackoffLinear,
		Jitter:  false,
	})

	delay := client.calculateDelay(0)
	if delay != 1*time.Second {
		t.Errorf("Expected delay of 1s, got %v", delay)
	}

	delay = client.calculateDelay(1)
	if delay != 2*time.Second {
		t.Errorf("Expected delay of 2s, got %v", delay)
	}

	delay = client.calculateDelay(2)
	if delay != 3*time.Second {
		t.Errorf("Expected delay of 3s, got %v", delay)
	}
}

func TestHTTPClient_CalculateDelayExponential(t *testing.T) {
	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		Delay:   1 * time.Second,
		Backoff: BackoffExponential,
		Jitter:  false,
	})

	delay := client.calculateDelay(0)
	if delay != 1*time.Second {
		t.Errorf("Expected delay of 1s, got %v", delay)
	}

	delay = client.calculateDelay(1)
	if delay != 2*time.Second {
		t.Errorf("Expected delay of 2s, got %v", delay)
	}

	delay = client.calculateDelay(2)
	if delay != 4*time.Second {
		t.Errorf("Expected delay of 4s, got %v", delay)
	}
}

func TestHTTPClient_CalculateDelayWithJitter(t *testing.T) {
	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		Delay:   1 * time.Second,
		Backoff: BackoffFixed,
		Jitter:  true,
	})

	delay := client.calculateDelay(0)
	// With jitter, delay should be between 1s and 1.1s
	if delay < 1*time.Second || delay > 1100*time.Millisecond {
		t.Errorf("Expected delay between 1s and 1.1s with jitter, got %v", delay)
	}
}

func TestHTTPClient_ShouldRetry(t *testing.T) {
	client := NewHTTPClient(nil)

	retryableCodes := []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	for _, code := range retryableCodes {
		if !client.shouldRetry(code) {
			t.Errorf("Expected status code %d to be retryable", code)
		}
	}

	nonRetryableCodes := []int{
		http.StatusOK,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
	}

	for _, code := range nonRetryableCodes {
		if client.shouldRetry(code) {
			t.Errorf("Expected status code %d to not be retryable", code)
		}
	}
}

func TestHTTPClient_DoNonRetryableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 3,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, response.StatusCode)
	}

	if response.Attempt != 1 {
		t.Errorf("Expected attempt 1 (no retries), got %d", response.Attempt)
	}

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.RetryAttempts != 0 {
		t.Errorf("Expected 0 retry attempts, got %d", metrics.RetryAttempts)
	}
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
}

func TestHTTPClient_GetMetrics(t *testing.T) {
	client := NewHTTPClient(nil)

	metrics := client.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}

	if metrics.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests, got %d", metrics.TotalRequests)
	}
}

func TestHTTPClient_DoWithRetryAndRequestBody(t *testing.T) {
	requestBody := `{"test": "data"}`
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		if string(body) != requestBody {
			t.Errorf("Expected request body %s, got %s", requestBody, string(body))
		}

		if attemptCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"received": true}`))
		}
	}))
	defer server.Close()

	client := NewHTTPClient(nil)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: 2,
		Delay:      10 * time.Millisecond,
		Backoff:    BackoffFixed,
		Jitter:     false,
	})

	req, err := http.NewRequest("POST", server.URL, strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 server attempts, got %d", attemptCount)
	}
}

func TestNewRequest(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "custom-value",
	}

	req, err := NewRequest("GET", "https://api.example.com/test", nil, headers)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}

	if req.URL.String() != "https://api.example.com/test" {
		t.Errorf("Expected URL https://api.example.com/test, got %s", req.URL.String())
	}

	if req.Header.Get("User-Agent") != "driftwatch/1.0.0" {
		t.Errorf("Expected User-Agent driftwatch/1.0.0, got %s", req.Header.Get("User-Agent"))
	}

	if req.Header.Get("Accept") != "application/json" {
		t.Errorf("Expected Accept application/json, got %s", req.Header.Get("Accept"))
	}

	if req.Header.Get("Authorization") != "Bearer token123" {
		t.Errorf("Expected Authorization Bearer token123, got %s", req.Header.Get("Authorization"))
	}

	if req.Header.Get("X-Custom") != "custom-value" {
		t.Errorf("Expected X-Custom custom-value, got %s", req.Header.Get("X-Custom"))
	}
}

func TestNewJSONRequest(t *testing.T) {
	body := strings.NewReader(`{"test": "data"}`)
	headers := map[string]string{
		"Authorization": "Bearer token123",
	}

	req, err := NewJSONRequest("POST", "https://api.example.com/test", body, headers)
	if err != nil {
		t.Fatalf("NewJSONRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("Expected method POST, got %s", req.Method)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", req.Header.Get("Content-Type"))
	}

	if req.Header.Get("Accept") != "application/json" {
		t.Errorf("Expected Accept application/json, got %s", req.Header.Get("Accept"))
	}

	if req.Header.Get("Authorization") != "Bearer token123" {
		t.Errorf("Expected Authorization Bearer token123, got %s", req.Header.Get("Authorization"))
	}
}

func TestNewRequestInvalidURL(t *testing.T) {
	_, err := NewRequest("GET", "://invalid-url", nil, nil)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}
