package http_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/logging"
)

// Example demonstrates basic usage of the HTTP client
func ExampleHTTPClient_Do() {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"message": "Hello, World!"}`))
	}))
	defer server.Close()

	// Create HTTP client with custom logger
	logConfig := logging.LoggerConfig{
		Level:  logging.LogLevelWarn,
		Format: logging.LogFormatText,
		Output: "stdout",
	}
	logger, _ := logging.NewLogger(logConfig)
	client := httpClient.NewHTTPClient(logger)

	// Configure retry policy
	client.SetRetryPolicy(httpClient.RetryPolicy{
		MaxRetries: 3,
		Delay:      100 * time.Millisecond,
		Backoff:    httpClient.BackoffExponential,
		Jitter:     true,
	})

	// Create request
	req, err := httpClient.NewJSONRequest("GET", server.URL, nil, map[string]string{
		"Authorization": "Bearer token123",
	})
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	// Execute request
	response, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Response: %s\n", string(response.Body))
	fmt.Printf("Response Time: %v\n", response.ResponseTime)
	fmt.Printf("Attempt: %d\n", response.Attempt)

	// Get metrics
	metrics := client.GetMetrics()
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulReqs)
}

// Example demonstrates retry behavior with server errors
func ExampleHTTPClient_retry() {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(500) // Server error - will retry
			w.Write([]byte("Server Error"))
		} else {
			w.WriteHeader(200)
			w.Write([]byte(`{"message": "Success after retries"}`))
		}
	}))
	defer server.Close()

	client := httpClient.NewHTTPClient(nil)
	client.SetRetryPolicy(httpClient.RetryPolicy{
		MaxRetries: 3,
		Delay:      50 * time.Millisecond,
		Backoff:    httpClient.BackoffFixed,
		Jitter:     false,
	})

	req, _ := httpClient.NewRequest("GET", server.URL, nil, nil)
	response, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}

	fmt.Printf("Final Status: %d\n", response.StatusCode)
	fmt.Printf("Total Attempts: %d\n", response.Attempt)
	fmt.Printf("Server Hit Count: %d\n", attemptCount)
}

// Example demonstrates request with body and custom headers
func ExampleHTTPClient_postWithBody() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"id": 123, "status": "created"}`))
	}))
	defer server.Close()

	client := httpClient.NewHTTPClient(nil)

	// Create POST request with JSON body
	body := strings.NewReader(`{"name": "test", "value": 42}`)
	req, err := httpClient.NewJSONRequest("POST", server.URL, body, map[string]string{
		"Authorization": "Bearer secret-token",
		"X-Request-ID":  "req-123",
	})
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	response, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Response: %s\n", string(response.Body))
	fmt.Printf("Content-Type: %s\n", response.Headers.Get("Content-Type"))
}
