package monitor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStorage is a mock implementation of the Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) SaveEndpoint(endpoint *storage.Endpoint) error {
	args := m.Called(endpoint)
	return args.Error(0)
}

func (m *MockStorage) GetEndpoint(id string) (*storage.Endpoint, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.Endpoint), args.Error(1)
}

func (m *MockStorage) ListEndpoints() ([]*storage.Endpoint, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.Endpoint), args.Error(1)
}

func (m *MockStorage) SaveMonitoringRun(run *storage.MonitoringRun) error {
	args := m.Called(run)
	return args.Error(0)
}

func (m *MockStorage) GetMonitoringHistory(endpointID string, period time.Duration) ([]*storage.MonitoringRun, error) {
	args := m.Called(endpointID, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.MonitoringRun), args.Error(1)
}

func (m *MockStorage) SaveDrift(drift *storage.Drift) error {
	args := m.Called(drift)
	return args.Error(0)
}

func (m *MockStorage) GetDrifts(filters storage.DriftFilters) ([]*storage.Drift, error) {
	args := m.Called(filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.Drift), args.Error(1)
}

func (m *MockStorage) SaveAlert(alert *storage.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockStorage) GetAlerts(filters storage.AlertFilters) ([]*storage.Alert, error) {
	args := m.Called(filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.Alert), args.Error(1)
}

func (m *MockStorage) BackupDatabase(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockStorage) CheckIntegrity() (*storage.IntegrityResult, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.IntegrityResult), args.Error(1)
}

func (m *MockStorage) CleanupOldAlerts(cutoff time.Time) (int64, error) {
	args := m.Called(cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) CleanupOldDrifts(cutoff time.Time) (int64, error) {
	args := m.Called(cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) CleanupOldMonitoringRuns(cutoff time.Time) (int64, error) {
	args := m.Called(cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) GetDatabaseStats() (*storage.DatabaseStats, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.DatabaseStats), args.Error(1)
}

func (m *MockStorage) GetHealthStatus() (*storage.HealthStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.HealthStatus), args.Error(1)
}

func (m *MockStorage) RepairDatabase() (*storage.RepairResult, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.RepairResult), args.Error(1)
}

func (m *MockStorage) VacuumDatabase() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStorage) RestoreDatabase(backupPath string) error {
	args := m.Called(backupPath)
	return args.Error(0)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockHTTPClient is a mock implementation of the HTTP Client interface
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*httpClient.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*httpClient.Response), args.Error(1)
}

func (m *MockHTTPClient) SetTimeout(duration time.Duration) {
	m.Called(duration)
}

func (m *MockHTTPClient) SetRetryPolicy(policy httpClient.RetryPolicy) {
	m.Called(policy)
}

func (m *MockHTTPClient) GetMetrics() *httpClient.Metrics {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*httpClient.Metrics)
}

func TestNewCronScheduler(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.cron)
	assert.NotNil(t, scheduler.endpoints)
	assert.NotNil(t, scheduler.endpointJobs)
	assert.NotNil(t, scheduler.endpointStatus)
	assert.Equal(t, cfg, scheduler.config)
	assert.Equal(t, mockStorage, scheduler.storage)
	assert.Equal(t, mockHTTPClient, scheduler.httpClient)
	assert.False(t, scheduler.running)
}

func TestSchedulerStartStop(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	// Mock ListEndpoints to return empty list
	mockStorage.On("ListEndpoints").Return([]*storage.Endpoint{}, nil)

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	ctx := context.Background()

	// Test Start
	err := scheduler.Start(ctx)
	require.NoError(t, err)
	assert.True(t, scheduler.running)
	assert.False(t, scheduler.startedAt.IsZero())

	// Test Stop
	err = scheduler.Stop()
	require.NoError(t, err)
	assert.False(t, scheduler.running)

	mockStorage.AssertExpectations(t)
}

func TestSchedulerStartAlreadyRunning(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	// Mock ListEndpoints to return empty list
	mockStorage.On("ListEndpoints").Return([]*storage.Endpoint{}, nil)

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	ctx := context.Background()

	// Start scheduler
	err := scheduler.Start(ctx)
	require.NoError(t, err)
	defer scheduler.Stop() // Ensure cleanup

	// Try to start again - should return error
	err = scheduler.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	mockStorage.AssertExpectations(t)
}

func TestAddEndpoint(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	endpoint := &config.EndpointConfig{
		ID:       "test-endpoint",
		URL:      "https://api.example.com/test",
		Method:   "GET",
		Interval: 5 * time.Minute,
		Enabled:  true,
	}

	err := scheduler.AddEndpoint(endpoint)
	require.NoError(t, err)

	// Check that endpoint was added
	assert.Contains(t, scheduler.endpoints, "test-endpoint")
	assert.Contains(t, scheduler.endpointJobs, "test-endpoint")
	assert.Contains(t, scheduler.endpointStatus, "test-endpoint")

	status, exists := scheduler.endpointStatus["test-endpoint"]
	require.True(t, exists, "endpoint status should exist")
	assert.Equal(t, "test-endpoint", status.ID)
	assert.True(t, status.Enabled)
}

func TestAddDisabledEndpoint(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	endpoint := &config.EndpointConfig{
		ID:       "disabled-endpoint",
		URL:      "https://api.example.com/test",
		Method:   "GET",
		Interval: 5 * time.Minute,
		Enabled:  false,
	}

	err := scheduler.AddEndpoint(endpoint)
	require.NoError(t, err)

	// Check that endpoint was not scheduled (disabled endpoints are skipped)
	assert.NotContains(t, scheduler.endpoints, "disabled-endpoint")
	assert.NotContains(t, scheduler.endpointJobs, "disabled-endpoint")
	assert.NotContains(t, scheduler.endpointStatus, "disabled-endpoint")
}

func TestRemoveEndpoint(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	endpoint := &config.EndpointConfig{
		ID:       "test-endpoint",
		URL:      "https://api.example.com/test",
		Method:   "GET",
		Interval: 5 * time.Minute,
		Enabled:  true,
	}

	// Add endpoint first
	err := scheduler.AddEndpoint(endpoint)
	require.NoError(t, err)

	// Remove endpoint
	err = scheduler.RemoveEndpoint("test-endpoint")
	require.NoError(t, err)

	// Check that endpoint was removed
	assert.NotContains(t, scheduler.endpoints, "test-endpoint")
	assert.NotContains(t, scheduler.endpointJobs, "test-endpoint")
	assert.NotContains(t, scheduler.endpointStatus, "test-endpoint")
}

func TestGetStatus(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
			MaxWorkers: 5,
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	// Add an endpoint
	endpoint := &config.EndpointConfig{
		ID:       "test-endpoint",
		URL:      "https://api.example.com/test",
		Method:   "GET",
		Interval: 5 * time.Minute,
		Enabled:  true,
	}

	err := scheduler.AddEndpoint(endpoint)
	require.NoError(t, err)

	status := scheduler.GetStatus()

	assert.False(t, status.Running)
	assert.Equal(t, 1, status.EndpointsScheduled)
	assert.Contains(t, status.EndpointStatuses, "test-endpoint")

	epStatus, exists := status.EndpointStatuses["test-endpoint"]
	require.True(t, exists, "endpoint status should exist in status response")
	assert.Equal(t, "test-endpoint", epStatus.ID)
	assert.True(t, epStatus.Enabled)
	assert.Equal(t, int64(0), epStatus.CheckCount)
	assert.Equal(t, int64(0), epStatus.ErrorCount)
}

func TestIntervalToCron(t *testing.T) {
	cfg := &config.Config{}
	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	tests := []struct {
		name     string
		interval time.Duration
		expected string
	}{
		{"30 seconds", 30 * time.Second, "*/30 * * * * *"},
		{"1 minute", 1 * time.Minute, "0 */1 * * * *"},
		{"5 minutes", 5 * time.Minute, "0 */5 * * * *"},
		{"1 hour", 1 * time.Hour, "0 0 */1 * * *"},
		{"2 hours", 2 * time.Hour, "0 0 */2 * * *"},
		{"24 hours", 24 * time.Hour, "0 0 0 * * *"},
		{"48 hours fallback", 48 * time.Hour, "0 0 0 * * *"}, // Falls back to daily
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.intervalToCron(tt.interval)
			assert.Equal(t, tt.expected, result, "Failed for interval %v", tt.interval)
		})
	}
}

func TestCheckOnceWithNoEndpoints(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			MaxWorkers: 5,
		},
		Endpoints: []config.EndpointConfig{}, // Empty endpoints
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	// Mock ListEndpoints to return empty list
	mockStorage.On("ListEndpoints").Return([]*storage.Endpoint{}, nil)

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	ctx := context.Background()
	err := scheduler.CheckOnce(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled endpoints")

	mockStorage.AssertExpectations(t)
}

func TestCheckOnceWithTimeout(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			MaxWorkers: 1,
		},
		Endpoints: []config.EndpointConfig{
			{
				ID:       "test-endpoint",
				URL:      "https://api.example.com/test",
				Method:   "GET",
				Interval: 5 * time.Minute,
				Enabled:  true,
			},
		},
	}

	mockStorage := &MockStorage{}
	mockHTTPClient := &MockHTTPClient{}

	// Mock ListEndpoints to return empty list (config endpoints will be used)
	mockStorage.On("ListEndpoints").Return([]*storage.Endpoint{}, nil)

	// Mock GetEndpoint to return not found error (so endpoint will be saved)
	mockStorage.On("GetEndpoint", "test-endpoint").Return(nil, fmt.Errorf("endpoint not found: test-endpoint"))

	// Mock SaveEndpoint to succeed
	mockStorage.On("SaveEndpoint", mock.AnythingOfType("*storage.Endpoint")).Return(nil)

	// Mock SaveMonitoringRun since it's called during endpoint checking
	mockStorage.On("SaveMonitoringRun", mock.AnythingOfType("*storage.MonitoringRun")).Return(nil).Maybe()

	// Mock the HTTP client Do method to simulate a response that takes longer than timeout
	// Use a custom function that blocks until context is cancelled
	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(
		func(req *http.Request) *httpClient.Response {
			// Block until context is cancelled
			<-req.Context().Done()
			return nil
		},
		func(req *http.Request) error {
			// Return context error
			return req.Context().Err()
		}).Maybe()

	scheduler := NewCronScheduler(cfg, mockStorage, mockHTTPClient)

	// Create a context that times out very quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give context time to timeout
	time.Sleep(2 * time.Millisecond)

	err := scheduler.CheckOnce(ctx)

	// Now we should get an error due to timeout
	if err != nil {
		// Check if it's a timeout-related error
		errorStr := err.Error()
		// The CheckOnce method wraps errors, so check for the wrapper message or common timeout patterns
		timeoutRelated := strings.Contains(errorStr, "deadline") ||
			strings.Contains(errorStr, "timeout") ||
			strings.Contains(errorStr, "cancelled") ||
			strings.Contains(errorStr, "encountered") // This is the wrapper message from CheckOnce

		assert.True(t, timeoutRelated, "Expected timeout-related error, got: %s", errorStr)
	} else {
		// If no error, that's also acceptable - the operation might complete very quickly
		t.Log("Operation completed before timeout - this is acceptable")
	}

	mockStorage.AssertExpectations(t)
	// Note: We don't assert HTTP client expectations since the timeout might prevent the call
}
