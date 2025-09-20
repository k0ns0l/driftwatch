// Package monitor provides API endpoint monitoring functionality
package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/k0ns0l/driftwatch/internal/auth"
	"github.com/k0ns0l/driftwatch/internal/config"
	httpClient "github.com/k0ns0l/driftwatch/internal/http"
	"github.com/k0ns0l/driftwatch/internal/logging"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/robfig/cron/v3"
)

// Monitor defines the interface for endpoint monitoring
type Monitor interface {
	CheckEndpoint(ctx context.Context, endpointID string) (*CheckResult, error)
	StartMonitoring(ctx context.Context) error
	StopMonitoring() error
	GetStatus() MonitorStatus
}

// Scheduler defines the interface for monitoring scheduler
type Scheduler interface {
	Start(ctx context.Context) error
	Stop() error
	AddEndpoint(endpoint *config.EndpointConfig) error
	RemoveEndpoint(id string) error
	GetStatus() SchedulerStatus
	CheckOnce(ctx context.Context) error
}

// CheckResult represents the result of a single endpoint check
type CheckResult struct {
	ResponseBody     []byte
	ResponseHeaders  map[string]string
	ValidationErrors []ValidationError
	Error            error
	EndpointID       string
	Timestamp        time.Time
	ResponseTime     time.Duration
	ResponseStatus   int
	Success          bool
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// MonitorStatus represents the current status of the monitoring system
type MonitorStatus struct {
	StartedAt          time.Time `json:"started_at,omitempty"`
	LastCheckAt        time.Time `json:"last_check_at,omitempty"`
	EndpointsMonitored int       `json:"endpoints_monitored"`
	Running            bool      `json:"running"`
}

// SchedulerStatus represents the current status of the scheduler
type SchedulerStatus struct {
	StartedAt          time.Time                 `json:"started_at,omitempty"`
	LastCheckAt        time.Time                 `json:"last_check_at,omitempty"`
	EndpointStatuses   map[string]EndpointStatus `json:"endpoint_statuses"`
	EndpointsScheduled int                       `json:"endpoints_scheduled"`
	Running            bool                      `json:"running"`
}

// EndpointStatus represents the status of a single endpoint
type EndpointStatus struct {
	ID         string    `json:"id"`
	LastError  string    `json:"last_error,omitempty"`
	LastCheck  time.Time `json:"last_check,omitempty"`
	CheckCount int64     `json:"check_count"`
	ErrorCount int64     `json:"error_count"`
	LastStatus int       `json:"last_status,omitempty"`
	Enabled    bool      `json:"enabled"`
}

// CronScheduler implements the Scheduler interface using cron for scheduling
type CronScheduler struct {
	cron           *cron.Cron
	endpoints      map[string]*config.EndpointConfig
	endpointJobs   map[string]cron.EntryID
	endpointStatus map[string]*EndpointStatus
	httpClient     httpClient.Client
	storage        storage.Storage
	config         *config.Config
	authManager    *auth.Manager
	logger         *log.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	startedAt      time.Time
	lastCheckAt    time.Time
	mu             sync.RWMutex
	running        bool
}

// NewCronScheduler creates a new cron-based scheduler
func NewCronScheduler(cfg *config.Config, storage storage.Storage, httpClient httpClient.Client) *CronScheduler {
	logger := log.New(os.Stdout, "[SCHEDULER] ", log.LstdFlags)

	// Create logging.Logger for auth manager
	loggingLogger, err := logging.NewLogger(logging.LoggerConfig{
		Level:  logging.LogLevelInfo,
		Format: logging.LogFormatText,
	})
	if err != nil {
		// Fallback to a basic logger if creation fails
		loggingLogger = logging.GetGlobalLogger()
	}

	return &CronScheduler{
		cron:           cron.New(cron.WithSeconds()),
		endpoints:      make(map[string]*config.EndpointConfig),
		endpointJobs:   make(map[string]cron.EntryID),
		endpointStatus: make(map[string]*EndpointStatus),
		httpClient:     httpClient,
		storage:        storage,
		config:         cfg,
		authManager:    auth.NewManager(loggingLogger),
		logger:         logger,
	}
}

// Start begins the monitoring scheduler
func (s *CronScheduler) Start(ctx context.Context) error {
	s.mu.Lock()

	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.startedAt = time.Now()
	s.mu.Unlock()

	// Load endpoints from storage
	if err := s.loadEndpoints(); err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("failed to load endpoints: %w", err)
	}

	// Start the cron scheduler
	s.cron.Start()
	s.logger.Printf("Scheduler started with %d endpoints", len(s.endpoints))

	return nil
}

// Stop stops the monitoring scheduler
func (s *CronScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Println("Stopping scheduler...")

	// Stop the cron scheduler
	cronCtx := s.cron.Stop()

	// Wait for running jobs to complete with timeout
	select {
	case <-cronCtx.Done():
		s.logger.Println("All scheduled jobs completed")
	case <-time.After(30 * time.Second):
		s.logger.Println("Timeout waiting for jobs to complete, forcing shutdown")
	}

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Println("Scheduler stopped")

	return nil
}

// AddEndpoint adds an endpoint to the monitoring schedule
func (s *CronScheduler) AddEndpoint(endpoint *config.EndpointConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !endpoint.Enabled {
		s.logger.Printf("Endpoint %s is disabled, skipping", endpoint.ID)
		return nil
	}

	// Remove existing job if it exists
	if jobID, exists := s.endpointJobs[endpoint.ID]; exists {
		s.cron.Remove(jobID)
		delete(s.endpointJobs, endpoint.ID)
	}

	// Convert interval to cron expression
	cronExpr := s.intervalToCron(endpoint.Interval)

	// Create monitoring job
	job := func() {
		s.checkEndpoint(endpoint)
	}

	// Schedule the job
	jobID, err := s.cron.AddFunc(cronExpr, job)
	if err != nil {
		return fmt.Errorf("failed to schedule endpoint %s: %w", endpoint.ID, err)
	}

	// Store endpoint and job info
	s.endpoints[endpoint.ID] = endpoint
	s.endpointJobs[endpoint.ID] = jobID
	s.endpointStatus[endpoint.ID] = &EndpointStatus{
		ID:      endpoint.ID,
		Enabled: endpoint.Enabled,
	}

	s.logger.Printf("Added endpoint %s with interval %s (cron: %s)",
		endpoint.ID, endpoint.Interval, cronExpr)

	return nil
}

// RemoveEndpoint removes an endpoint from the monitoring schedule
func (s *CronScheduler) RemoveEndpoint(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove cron job
	if jobID, exists := s.endpointJobs[id]; exists {
		s.cron.Remove(jobID)
		delete(s.endpointJobs, id)
	}

	// Remove from maps
	delete(s.endpoints, id)
	delete(s.endpointStatus, id)

	s.logger.Printf("Removed endpoint %s from schedule", id)

	return nil
}

// GetStatus returns the current scheduler status
func (s *CronScheduler) GetStatus() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy endpoint statuses
	statuses := make(map[string]EndpointStatus)
	for id, status := range s.endpointStatus {
		statuses[id] = *status
	}

	return SchedulerStatus{
		Running:            s.running,
		StartedAt:          s.startedAt,
		EndpointsScheduled: len(s.endpoints),
		LastCheckAt:        s.lastCheckAt,
		EndpointStatuses:   statuses,
	}
}

// CheckOnce performs a one-time check of all endpoints
func (s *CronScheduler) CheckOnce(ctx context.Context) error {
	// First load endpoints if not already loaded
	if len(s.endpoints) == 0 {
		if err := s.loadEndpoints(); err != nil {
			return fmt.Errorf("failed to load endpoints: %w", err)
		}
	}

	s.mu.RLock()
	endpoints := make([]*config.EndpointConfig, 0, len(s.endpoints))
	for _, ep := range s.endpoints {
		if ep.Enabled {
			endpoints = append(endpoints, ep)
		}
	}
	s.mu.RUnlock()

	if len(endpoints) == 0 {
		return fmt.Errorf("no enabled endpoints to check")
	}

	s.logger.Printf("Performing one-time check of %d endpoints", len(endpoints))

	// Use worker pool for concurrent checks
	maxWorkers := s.config.Global.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	jobs := make(chan *config.EndpointConfig, len(endpoints))
	results := make(chan error, len(endpoints))

	// Start workers
	for w := 0; w < maxWorkers; w++ {
		go func() {
			for endpoint := range jobs {
				select {
				case <-ctx.Done():
					results <- ctx.Err()
					return
				default:
					s.checkEndpoint(endpoint)
					results <- nil
				}
			}
		}()
	}

	// Send jobs
	for _, endpoint := range endpoints {
		jobs <- endpoint
	}
	close(jobs)

	// Wait for results
	var errors []error
	for i := 0; i < len(endpoints); i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during one-time check", len(errors))
	}

	s.logger.Printf("One-time check completed successfully")
	return nil
}

// loadEndpoints loads endpoints from storage and configuration
func (s *CronScheduler) loadEndpoints() error {
	s.logger.Printf("Loading %d endpoints from configuration", len(s.config.Endpoints))
	var errors []error

	// First, load from configuration and save to database if not already present
	for _, endpointConfig := range s.config.Endpoints {
		// Check if endpoint IF already exists in database
		_, err := s.storage.GetEndpoint(endpointConfig.ID)
		if err != nil {
			// Endpoint doesn't exist in database, save it
			s.logger.Printf("Endpoint %s not found in database, saving it", endpointConfig.ID)
			configJSON, marshalErr := json.Marshal(endpointConfig)
			if marshalErr != nil {
				s.logger.Printf("Failed to marshal config for endpoint %s: %v", endpointConfig.ID, marshalErr)
				errors = append(errors, fmt.Errorf("failed to marshal config for endpoint %s: %w", endpointConfig.ID, marshalErr))
				continue
			}

			endpoint := &storage.Endpoint{
				ID:        endpointConfig.ID,
				URL:       endpointConfig.URL,
				Method:    endpointConfig.Method,
				SpecFile:  endpointConfig.SpecFile,
				Config:    string(configJSON),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if saveErr := s.storage.SaveEndpoint(endpoint); saveErr != nil {
				s.logger.Printf("Failed to save endpoint %s to database: %v", endpointConfig.ID, saveErr)
				errors = append(errors, fmt.Errorf("failed to save endpoint %s to database: %w", endpointConfig.ID, saveErr))
				// Skip adding to scheduler if we can't save to database to avoid foreign key constraint errors
				continue
			} else {
				s.logger.Printf("Successfully saved endpoint %s to database", endpointConfig.ID)
			}
		} else {
			s.logger.Printf("Endpoint %s already exists in database", endpointConfig.ID)
		}

		if err := s.AddEndpoint(&endpointConfig); err != nil {
			s.logger.Printf("Failed to add endpoint %s from config: %v", endpointConfig.ID, err)
			errors = append(errors, fmt.Errorf("failed to add endpoint %s from config: %w", endpointConfig.ID, err))
		}
	}

	// Then, load from storage (this might override config endpoints with database versions)
	endpoints, err := s.storage.ListEndpoints()
	if err != nil {
		s.logger.Printf("Failed to list endpoints from storage: %v", err)
		// Don't return error, just log it - we can still use config endpoints
		if len(errors) > 0 {
			return fmt.Errorf("encountered %d errors loading endpoints", len(errors))
		}
		return nil
	}

	for _, dbEndpoint := range endpoints {
		// Parse endpoint config from JSON
		var endpointConfig config.EndpointConfig
		if err := parseEndpointConfig(dbEndpoint.Config, &endpointConfig); err != nil {
			s.logger.Printf("Failed to parse config for endpoint %s: %v", dbEndpoint.ID, err)
			errors = append(errors, fmt.Errorf("failed to parse config for endpoint %s: %w", dbEndpoint.ID, err))
			continue
		}

		// Add to scheduler (this will replace any config endpoint with same ID)
		if err := s.AddEndpoint(&endpointConfig); err != nil {
			s.logger.Printf("Failed to add endpoint %s to scheduler: %v", endpointConfig.ID, err)
			errors = append(errors, fmt.Errorf("failed to add endpoint %s to scheduler: %w", endpointConfig.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors loading endpoints", len(errors))
	}
	return nil
}

// checkEndpoint performs a single endpoint check
func (s *CronScheduler) checkEndpoint(endpoint *config.EndpointConfig) {
	start := time.Now()

	s.mu.Lock()
	status := s.endpointStatus[endpoint.ID]
	if status == nil {
		status = &EndpointStatus{
			ID:      endpoint.ID,
			Enabled: endpoint.Enabled,
		}
		s.endpointStatus[endpoint.ID] = status
	}
	s.lastCheckAt = start
	s.mu.Unlock()

	// Update status
	status.LastCheck = start
	status.CheckCount++

	// Create authenticator if auth config is provided
	var authenticator auth.Authenticator
	if endpoint.Auth != nil {
		var err error
		authenticator, err = s.authManager.CreateAuthenticator(endpoint.Auth)
		if err != nil {
			s.handleCheckError(status, fmt.Errorf("failed to create authenticator: %w", err))
			return
		}
	}

	// Create HTTP request
	req, err := httpClient.NewRequest(endpoint.Method, endpoint.URL, nil, endpoint.Headers)
	if err != nil {
		s.handleCheckError(status, fmt.Errorf("failed to create request: %w", err))
		return
	}

	// Apply authentication if configured
	if authenticator != nil {
		if err := authenticator.ApplyAuth(req); err != nil {
			s.handleCheckError(status, fmt.Errorf("failed to apply authentication: %w", err))
			return
		}
	}

	// Set timeout
	timeout := endpoint.Timeout
	if timeout == 0 {
		timeout = s.config.Global.Timeout
	}

	// Use background context if scheduler context is not available
	parentCtx := s.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	// Perform request
	resp, err := s.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		s.handleCheckError(status, fmt.Errorf("request failed: %w", err))
		return
	}

	// Update status with success
	status.LastStatus = resp.StatusCode
	status.LastError = ""

	// Verify endpoint exists in database before saving monitoring run
	_, err = s.storage.GetEndpoint(endpoint.ID)
	if err != nil {
		s.logger.Printf("Endpoint %s not found in database, attempting to save it before monitoring run", endpoint.ID)

		// Try to save the endpoint to database
		configJSON, marshalErr := json.Marshal(endpoint)
		if marshalErr != nil {
			s.logger.Printf("Failed to marshal config for endpoint %s: %v", endpoint.ID, marshalErr)
			return
		}

		dbEndpoint := &storage.Endpoint{
			ID:        endpoint.ID,
			URL:       endpoint.URL,
			Method:    endpoint.Method,
			SpecFile:  endpoint.SpecFile,
			Config:    string(configJSON),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if saveErr := s.storage.SaveEndpoint(dbEndpoint); saveErr != nil {
			s.logger.Printf("Failed to save endpoint %s to database: %v", endpoint.ID, saveErr)
			s.logger.Printf("Skipping monitoring run save for %s due to database constraint", endpoint.ID)
			return
		} else {
			s.logger.Printf("Successfully saved endpoint %s to database", endpoint.ID)
		}
	}

	// Save monitoring run to storage
	run := &storage.MonitoringRun{
		EndpointID:      endpoint.ID,
		Timestamp:       start,
		ResponseStatus:  resp.StatusCode,
		ResponseTimeMs:  resp.ResponseTime.Milliseconds(),
		ResponseBody:    string(resp.Body),
		ResponseHeaders: s.convertHeaders(resp.Headers),
	}

	if err := s.storage.SaveMonitoringRun(run); err != nil {
		s.logger.Printf("Failed to save monitoring run for %s: %v", endpoint.ID, err)
	}

	s.logger.Printf("Checked endpoint %s: %d (%s)",
		endpoint.ID, resp.StatusCode, time.Since(start))
}

// handleCheckError handles errors during endpoint checks
func (s *CronScheduler) handleCheckError(status *EndpointStatus, err error) {
	status.ErrorCount++
	status.LastError = err.Error()
	s.logger.Printf("Error checking endpoint %s: %v", status.ID, err)
}

// convertHeaders converts http.Header to map[string]string
func (s *CronScheduler) convertHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// intervalToCron converts a time.Duration to a cron expression
func (s *CronScheduler) intervalToCron(interval time.Duration) string {
	seconds := int(interval.Seconds())

	if seconds < 60 {
		// Every N seconds
		return fmt.Sprintf("*/%d * * * * *", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		// Every N minutes
		return fmt.Sprintf("0 */%d * * * *", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		// Every N hours
		return fmt.Sprintf("0 0 */%d * * *", hours)
	}

	// Daily (fallback for very long intervals)
	return "0 0 0 * * *"
}

// parseEndpointConfig parses JSON config string into EndpointConfig
func parseEndpointConfig(configJSON string, endpointConfig *config.EndpointConfig) error {
	if configJSON == "" {
		return fmt.Errorf("empty config JSON")
	}

	return json.Unmarshal([]byte(configJSON), endpointConfig)
}
