// Package http provides HTTP client functionality with retry logic
package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/errors"
	"github.com/k0ns0l/driftwatch/internal/logging"
)

// Client defines the interface for HTTP operations
type Client interface {
	Do(req *http.Request) (*Response, error)
	SetTimeout(duration time.Duration)
	SetRetryPolicy(policy RetryPolicy)
	GetMetrics() *Metrics
}

// Response represents an HTTP response with additional metadata
type Response struct {
	StatusCode   int           `json:"status_code"`
	Headers      http.Header   `json:"headers"`
	Body         []byte        `json:"body"`
	ResponseTime time.Duration `json:"response_time"`
	Timestamp    time.Time     `json:"timestamp"`
	Attempt      int           `json:"attempt"`
}

// RetryPolicy defines retry behavior for HTTP requests
type RetryPolicy struct {
	MaxRetries int             `json:"max_retries"`
	Delay      time.Duration   `json:"delay"`
	Backoff    BackoffStrategy `json:"backoff"`
	Jitter     bool            `json:"jitter"`
}

// BackoffStrategy defines the backoff strategy for retries
type BackoffStrategy string

const (
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffExponential BackoffStrategy = "exponential"
	BackoffLinear      BackoffStrategy = "linear"
)

// Metrics holds HTTP client metrics
type Metrics struct {
	TotalRequests   int64         `json:"total_requests"`
	SuccessfulReqs  int64         `json:"successful_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	RetryAttempts   int64         `json:"retry_attempts"`
	AverageRespTime time.Duration `json:"average_response_time"`
	TotalRespTime   time.Duration `json:"total_response_time"`
}

// HTTPClient implements the Client interface with retry logic and metrics
type HTTPClient struct {
	client      *http.Client
	retryPolicy RetryPolicy
	logger      *logging.Logger
	metrics     *Metrics
}

// NewHTTPClient creates a new HTTP client with default settings
func NewHTTPClient(logger *logging.Logger) *HTTPClient {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryPolicy: RetryPolicy{
			MaxRetries: 3,
			Delay:      1 * time.Second,
			Backoff:    BackoffExponential,
			Jitter:     true,
		},
		logger:  logger.WithComponent("http_client"),
		metrics: &Metrics{},
	}
}

// Do executes an HTTP request with retry logic and metrics collection
func (c *HTTPClient) Do(req *http.Request) (*Response, error) {
	c.metrics.TotalRequests++

	bodyBytes, err := c.prepareRequestBody(req)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		response, err := c.executeAttempt(req, bodyBytes, attempt)
		if err != nil {
			lastErr = err
			if attempt < c.retryPolicy.MaxRetries {
				c.retryAfterDelay(attempt)
				continue
			}
			break
		}

		// Check if we should retry based on status code
		if c.shouldRetry(response.StatusCode) && attempt < c.retryPolicy.MaxRetries {
			c.logRetryableStatus(req, response, attempt)
			c.retryAfterDelay(attempt)
			continue
		}

		c.logFinalResult(req, response, attempt)
		return response, nil
	}

	return c.handleExhaustedRetries(req, lastErr)
}

// prepareRequestBody reads and stores the request body for potential retries
func (c *HTTPClient) prepareRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		c.metrics.FailedRequests++
		httpErr := errors.WrapError(err, errors.ErrorTypeNetwork, "HTTP_BODY_READ", "failed to read request body").
			WithSeverity(errors.SeverityMedium).
			WithGuidance("Check request body content and size")
		c.logger.LogError(context.Background(), httpErr, "Failed to read request body")
		return nil, httpErr
	}

	if closeErr := req.Body.Close(); closeErr != nil {
		c.logger.Warn("Failed to close request body", "error", closeErr)
	}

	return bodyBytes, nil
}

// executeAttempt performs a single HTTP request attempt
func (c *HTTPClient) executeAttempt(req *http.Request, bodyBytes []byte, attempt int) (*Response, error) {
	// Restore body for retry attempts
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	startTime := time.Now()
	c.logger.Debug("Making HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
		"attempt", attempt+1,
		"max_attempts", c.retryPolicy.MaxRetries+1)

	resp, err := c.client.Do(req)
	responseTime := time.Since(startTime)

	if attempt > 0 {
		c.metrics.RetryAttempts++
	}

	if err != nil {
		return nil, c.handleRequestError(err, req, attempt, responseTime)
	}

	return c.processResponse(resp, responseTime, startTime, attempt)
}

// handleRequestError handles network-level request errors
func (c *HTTPClient) handleRequestError(err error, req *http.Request, attempt int, responseTime time.Duration) error {
	wrappedErr := c.wrapNetworkError(err, req, attempt+1, responseTime)
	c.logger.LogError(context.Background(), wrappedErr, "HTTP request failed",
		"method", req.Method,
		"url", req.URL.String(),
		"attempt", attempt+1,
		"response_time", responseTime)
	return wrappedErr
}

// processResponse reads and processes the HTTP response
func (c *HTTPClient) processResponse(resp *http.Response, responseTime time.Duration, startTime time.Time, attempt int) (*Response, error) {
	body, err := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		c.logger.Warn("Failed to close response body", "error", closeErr)
	}

	if err != nil {
		return nil, errors.WrapError(err, errors.ErrorTypeNetwork, "HTTP_RESPONSE_READ", "failed to read response body").
			WithSeverity(errors.SeverityMedium).
			WithRecoverable(true).
			WithGuidance("Check network connectivity and response size").
			WithContext("method", resp.Request.Method).
			WithContext("url", resp.Request.URL.String()).
			WithContext("attempt", attempt+1)
	}

	response := &Response{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header,
		Body:         body,
		ResponseTime: responseTime,
		Timestamp:    startTime,
		Attempt:      attempt + 1,
	}

	// Update metrics
	c.metrics.TotalRespTime += responseTime
	c.metrics.AverageRespTime = c.metrics.TotalRespTime / time.Duration(c.metrics.TotalRequests)

	return response, nil
}

// retryAfterDelay waits for the calculated delay before retrying
func (c *HTTPClient) retryAfterDelay(attempt int) {
	delay := c.calculateDelay(attempt)
	c.logger.Debug("Retrying request after delay",
		"delay", delay,
		"next_attempt", attempt+2)
	time.Sleep(delay)
}

// logRetryableStatus logs when a request returns a retryable status code
func (c *HTTPClient) logRetryableStatus(req *http.Request, response *Response, attempt int) {
	c.logger.Warn("HTTP request returned retryable status code",
		"method", req.Method,
		"url", req.URL.String(),
		"status_code", response.StatusCode,
		"attempt", attempt+1,
		"response_time", response.ResponseTime)
}

// logFinalResult logs the final result of the request
func (c *HTTPClient) logFinalResult(req *http.Request, response *Response, attempt int) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		c.metrics.SuccessfulReqs++
		c.logger.Debug("HTTP request successful",
			"method", req.Method,
			"url", req.URL.String(),
			"status_code", response.StatusCode,
			"response_time", response.ResponseTime,
			"attempt", attempt+1)
	} else {
		c.metrics.FailedRequests++
		c.logger.Warn("HTTP request failed with non-retryable status",
			"method", req.Method,
			"url", req.URL.String(),
			"status_code", response.StatusCode,
			"response_time", response.ResponseTime,
			"attempt", attempt+1)
	}
}

// handleExhaustedRetries handles the case when all retries are exhausted
func (c *HTTPClient) handleExhaustedRetries(req *http.Request, lastErr error) (*Response, error) {
	c.metrics.FailedRequests++
	if lastErr != nil {
		finalErr := errors.WrapError(lastErr, errors.ErrorTypeNetwork, "HTTP_REQUEST_EXHAUSTED",
			fmt.Sprintf("request failed after %d attempts", c.retryPolicy.MaxRetries+1)).
			WithSeverity(errors.SeverityHigh).
			WithGuidance("Check endpoint availability and network connectivity").
			WithContext("method", req.Method).
			WithContext("url", req.URL.String()).
			WithContext("max_attempts", c.retryPolicy.MaxRetries+1)

		c.logger.LogError(context.Background(), finalErr, "Request failed after all retries")
		return nil, finalErr
	}
	return nil, nil
}

// SetTimeout sets the HTTP client timeout
func (c *HTTPClient) SetTimeout(duration time.Duration) {
	c.client.Timeout = duration
	c.logger.Debug("HTTP client timeout updated", "timeout", duration)
}

// SetRetryPolicy sets the retry policy for the HTTP client
func (c *HTTPClient) SetRetryPolicy(policy RetryPolicy) {
	c.retryPolicy = policy
	c.logger.Debug("HTTP client retry policy updated",
		"max_retries", policy.MaxRetries,
		"delay", policy.Delay,
		"backoff", policy.Backoff,
		"jitter", policy.Jitter)
}

// GetMetrics returns the current metrics
func (c *HTTPClient) GetMetrics() *Metrics {
	return c.metrics
}

// calculateDelay calculates the delay for the next retry attempt
func (c *HTTPClient) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch c.retryPolicy.Backoff {
	case BackoffFixed:
		delay = c.retryPolicy.Delay
	case BackoffLinear:
		delay = c.retryPolicy.Delay * time.Duration(attempt+1)
	case BackoffExponential:
		delay = c.retryPolicy.Delay * time.Duration(math.Pow(2, float64(attempt)))
	default:
		delay = c.retryPolicy.Delay
	}

	// Add jitter if enabled
	if c.retryPolicy.Jitter {
		// Use crypto/rand for secure random number generation
		maxJitter := int64(float64(delay) * 0.1) // 10% jitter
		if maxJitter > 0 {
			jitterBig, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
			if err == nil {
				jitter := time.Duration(jitterBig.Int64())
				delay += jitter
			}
			// If crypto/rand fails, continue without jitter (safer than using weak rand)
		}
	}

	return delay
}

// shouldRetry determines if a request should be retried based on status code
func (c *HTTPClient) shouldRetry(statusCode int) bool {
	// Retry on server errors (5xx) and some client errors
	switch statusCode {
	case http.StatusRequestTimeout, // 408
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// NewRequest creates a new HTTP request with common headers
func NewRequest(method, url string, body io.Reader, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", "driftwatch/1.0.0")
	req.Header.Set("Accept", "application/json")

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// NewJSONRequest creates a new HTTP request with JSON content type
func NewJSONRequest(method, url string, body io.Reader, headers map[string]string) (*http.Request, error) {
	req, err := NewRequest(method, url, body, headers)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// ClientConfig holds configuration for creating HTTP clients
type ClientConfig struct {
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
	UserAgent  string
}

// NewClient is a variable that holds the function to create a new HTTP client
// This allows for easy mocking in tests
var NewClient = func(config ClientConfig) Client {
	client := NewHTTPClient(nil)

	client.SetTimeout(config.Timeout)
	client.SetRetryPolicy(RetryPolicy{
		MaxRetries: config.RetryCount,
		Delay:      config.RetryDelay,
		Backoff:    BackoffExponential,
		Jitter:     true,
	})

	return client
}

// CreateRequest creates an HTTP request with the given parameters
func (c *HTTPClient) CreateRequest(method, url string, headers map[string]string, body io.Reader) (*http.Request, error) {
	return NewRequest(method, url, body, headers)
}

// CreateAuthenticatedRequest creates an HTTP request with authentication applied
func (c *HTTPClient) CreateAuthenticatedRequest(method, url string, headers map[string]string, body io.Reader, authenticator interface{}) (*http.Request, error) {
	req, err := NewRequest(method, url, body, headers)
	if err != nil {
		return nil, err
	}

	// Apply authentication if provided
	if auth, ok := authenticator.(interface{ ApplyAuth(*http.Request) error }); ok && auth != nil {
		if err := auth.ApplyAuth(req); err != nil {
			return nil, fmt.Errorf("failed to apply authentication: %w", err)
		}
	}

	return req, nil
}

// wrapNetworkError wraps a network error with appropriate DriftWatch error context
func (c *HTTPClient) wrapNetworkError(err error, req *http.Request, attempt int, responseTime time.Duration) *errors.DriftWatchError {
	var code string
	var message string
	var guidance string

	// Categorize the error based on its type or message
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		code = "HTTP_TIMEOUT"
		message = "HTTP request timed out"
		guidance = "Increase timeout value or check endpoint performance"
	case strings.Contains(errStr, "connection refused"):
		code = "HTTP_CONNECTION_REFUSED"
		message = "Connection refused by server"
		guidance = "Check if the service is running and accessible"
	case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "dns"):
		code = "HTTP_DNS_ERROR"
		message = "DNS resolution failed"
		guidance = "Check hostname spelling and DNS configuration"
	case strings.Contains(errStr, "certificate") || strings.Contains(errStr, "tls") || strings.Contains(errStr, "ssl"):
		code = "HTTP_TLS_ERROR"
		message = "TLS/SSL connection failed"
		guidance = "Check certificate validity or use --insecure flag for testing"
	default:
		code = "HTTP_NETWORK_ERROR"
		message = "Network request failed"
		guidance = "Check network connectivity and endpoint availability"
	}

	return errors.WrapError(err, errors.ErrorTypeNetwork, code, message).
		WithSeverity(errors.SeverityHigh).
		WithRecoverable(true).
		WithGuidance(guidance).
		WithContext("method", req.Method).
		WithContext("url", req.URL.String()).
		WithContext("attempt", attempt).
		WithContext("response_time", responseTime)
}
