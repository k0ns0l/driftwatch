package alerting

import (
	"context"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/config"
	"github.com/k0ns0l/driftwatch/internal/drift"
	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStorage implements the storage.Storage interface for testing
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) SaveEndpoint(endpoint *storage.Endpoint) error {
	args := m.Called(endpoint)
	return args.Error(0)
}

func (m *MockStorage) GetEndpoint(id string) (*storage.Endpoint, error) {
	args := m.Called(id)
	return args.Get(0).(*storage.Endpoint), args.Error(1)
}

func (m *MockStorage) ListEndpoints() ([]*storage.Endpoint, error) {
	args := m.Called()
	return args.Get(0).([]*storage.Endpoint), args.Error(1)
}

func (m *MockStorage) SaveMonitoringRun(run *storage.MonitoringRun) error {
	args := m.Called(run)
	return args.Error(0)
}

func (m *MockStorage) GetMonitoringHistory(endpointID string, period time.Duration) ([]*storage.MonitoringRun, error) {
	args := m.Called(endpointID, period)
	return args.Get(0).([]*storage.MonitoringRun), args.Error(1)
}

func (m *MockStorage) SaveDrift(drift *storage.Drift) error {
	args := m.Called(drift)
	if args.Get(0) != nil {
		// Set ID to simulate database behavior
		drift.ID = args.Get(0).(int64)
	}
	return args.Error(1)
}

func (m *MockStorage) GetDrifts(filters storage.DriftFilters) ([]*storage.Drift, error) {
	args := m.Called(filters)
	return args.Get(0).([]*storage.Drift), args.Error(1)
}

func (m *MockStorage) SaveAlert(alert *storage.Alert) error {
	args := m.Called(alert)
	if args.Get(0) != nil {
		// Set ID to simulate database behavior
		alert.ID = args.Get(0).(int64)
	}
	return args.Error(1)
}

func (m *MockStorage) GetAlerts(filters storage.AlertFilters) ([]*storage.Alert, error) {
	args := m.Called(filters)
	return args.Get(0).([]*storage.Alert), args.Error(1)
}

// Data retention and cleanup methods
func (m *MockStorage) CleanupOldMonitoringRuns(olderThan time.Time) (int64, error) {
	args := m.Called(olderThan)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) CleanupOldDrifts(olderThan time.Time) (int64, error) {
	args := m.Called(olderThan)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) CleanupOldAlerts(olderThan time.Time) (int64, error) {
	args := m.Called(olderThan)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) GetDatabaseStats() (*storage.DatabaseStats, error) {
	args := m.Called()
	return args.Get(0).(*storage.DatabaseStats), args.Error(1)
}

func (m *MockStorage) VacuumDatabase() error {
	args := m.Called()
	return args.Error(0)
}

// Database integrity and recovery methods
func (m *MockStorage) CheckIntegrity() (*storage.IntegrityResult, error) {
	args := m.Called()
	return args.Get(0).(*storage.IntegrityResult), args.Error(1)
}

func (m *MockStorage) RepairDatabase() (*storage.RepairResult, error) {
	args := m.Called()
	return args.Get(0).(*storage.RepairResult), args.Error(1)
}

func (m *MockStorage) BackupDatabase(backupPath string) error {
	args := m.Called(backupPath)
	return args.Error(0)
}

func (m *MockStorage) RestoreDatabase(backupPath string) error {
	args := m.Called(backupPath)
	return args.Error(0)
}

func (m *MockStorage) GetHealthStatus() (*storage.HealthStatus, error) {
	args := m.Called()
	return args.Get(0).(*storage.HealthStatus), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockAlertChannel implements the AlertChannel interface for testing
type MockAlertChannel struct {
	mock.Mock
	name     string
	chanType string
	enabled  bool
}

func (m *MockAlertChannel) Send(ctx context.Context, message *AlertMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockAlertChannel) Test(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAlertChannel) GetType() string {
	return m.chanType
}

func (m *MockAlertChannel) GetName() string {
	return m.name
}

func (m *MockAlertChannel) IsEnabled() bool {
	return m.enabled
}

func TestNewAlertManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "valid configuration",
			config: &config.Config{
				Alerting: config.AlertingConfig{
					Enabled:  true,
					Channels: []config.AlertChannelConfig{},
					Rules:    []config.AlertRuleConfig{},
				},
			},
			expectError: false,
		},
		{
			name: "disabled alerting",
			config: &config.Config{
				Alerting: config.AlertingConfig{
					Enabled:  false,
					Channels: []config.AlertChannelConfig{},
					Rules:    []config.AlertRuleConfig{},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}

			manager, err := NewAlertManager(tt.config, mockStorage)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
			}
		})
	}
}

func TestProcessDrift(t *testing.T) {
	mockStorage := &MockStorage{}

	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled:  false, // Disable alerting to avoid channel creation issues
			Channels: []config.AlertChannelConfig{},
			Rules:    []config.AlertRuleConfig{},
		},
	}

	manager, err := NewAlertManager(cfg, mockStorage)
	require.NoError(t, err)

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
	}

	driftResult := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.user.email",
				Description: "Field 'email' was removed",
				Severity:    drift.SeverityCritical,
				Breaking:    true,
			},
		},
	}

	// Mock storage calls
	mockStorage.On("SaveDrift", mock.AnythingOfType("*storage.Drift")).Return(int64(1), nil)

	ctx := context.Background()
	err = manager.ProcessDrift(ctx, driftResult, endpoint)

	// Since we can't easily mock the Slack channel creation, we expect an error
	// but we can verify that the drift processing logic works
	assert.NoError(t, err) // Should not error if no rules match or alerting disabled

	mockStorage.AssertExpectations(t)
}

func TestSendAlert(t *testing.T) {
	mockStorage := &MockStorage{}
	mockChannel := &MockAlertChannel{
		name:     "test-channel",
		chanType: "test",
		enabled:  true,
	}

	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Enabled: true,
			Rules: []config.AlertRuleConfig{
				{
					Name:     "test-rule",
					Severity: []string{"high"},
					Channels: []string{"test-channel"},
				},
			},
		},
	}

	manager := &DefaultAlertManager{
		config:  cfg,
		storage: mockStorage,
		channels: map[string]AlertChannel{
			"test-channel": mockChannel,
		},
	}

	drift := &storage.Drift{
		ID:          1,
		EndpointID:  "test-endpoint",
		Severity:    "high",
		Description: "Test drift",
		DetectedAt:  time.Now(),
	}

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/test",
		Method: "GET",
	}

	// Mock successful alert send
	mockChannel.On("Send", mock.Anything, mock.AnythingOfType("*alerting.AlertMessage")).Return(nil)
	mockStorage.On("SaveAlert", mock.AnythingOfType("*storage.Alert")).Return(int64(1), nil)

	ctx := context.Background()
	err := manager.SendAlert(ctx, drift, endpoint)

	assert.NoError(t, err)
	mockChannel.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestTestConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		alertingEnabled bool
		channels        map[string]AlertChannel
		expectError     bool
	}{
		{
			name:            "alerting disabled",
			alertingEnabled: false,
			channels:        map[string]AlertChannel{},
			expectError:     true,
		},
		{
			name:            "no channels configured",
			alertingEnabled: true,
			channels:        map[string]AlertChannel{},
			expectError:     true,
		},
		{
			name:            "successful test",
			alertingEnabled: true,
			channels: map[string]AlertChannel{
				"test-channel": &MockAlertChannel{
					name:     "test-channel",
					chanType: "test",
					enabled:  true,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}

			cfg := &config.Config{
				Alerting: config.AlertingConfig{
					Enabled: tt.alertingEnabled,
				},
			}

			manager := &DefaultAlertManager{
				config:   cfg,
				storage:  mockStorage,
				channels: tt.channels,
			}

			// Mock channel test calls
			for _, channel := range tt.channels {
				if mockChannel, ok := channel.(*MockAlertChannel); ok {
					mockChannel.On("Test", mock.Anything).Return(nil)
				}
			}

			ctx := context.Background()
			err := manager.TestConfiguration(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			for _, channel := range tt.channels {
				if mockChannel, ok := channel.(*MockAlertChannel); ok {
					mockChannel.AssertExpectations(t)
				}
			}
		})
	}
}

func TestFindApplicableRules(t *testing.T) {
	cfg := &config.Config{
		Alerting: config.AlertingConfig{
			Rules: []config.AlertRuleConfig{
				{
					Name:      "critical-rule",
					Severity:  []string{"critical"},
					Endpoints: []string{"endpoint-1"},
					Channels:  []string{"slack"},
				},
				{
					Name:     "high-rule",
					Severity: []string{"high", "medium"},
					Channels: []string{"email"},
				},
				{
					Name:      "specific-endpoint",
					Severity:  []string{"low"},
					Endpoints: []string{"endpoint-2"},
					Channels:  []string{"webhook"},
				},
			},
		},
	}

	manager := &DefaultAlertManager{
		config: cfg,
	}

	tests := []struct {
		name          string
		drift         *storage.Drift
		endpoint      *storage.Endpoint
		expectedRules int
	}{
		{
			name: "critical drift on endpoint-1",
			drift: &storage.Drift{
				Severity: "critical",
			},
			endpoint: &storage.Endpoint{
				ID: "endpoint-1",
			},
			expectedRules: 1, // Only critical-rule matches
		},
		{
			name: "high drift on any endpoint",
			drift: &storage.Drift{
				Severity: "high",
			},
			endpoint: &storage.Endpoint{
				ID: "any-endpoint",
			},
			expectedRules: 1, // Only high-rule matches (no endpoint restriction)
		},
		{
			name: "low drift on endpoint-2",
			drift: &storage.Drift{
				Severity: "low",
			},
			endpoint: &storage.Endpoint{
				ID: "endpoint-2",
			},
			expectedRules: 1, // Only specific-endpoint rule matches
		},
		{
			name: "no matching rules",
			drift: &storage.Drift{
				Severity: "info",
			},
			endpoint: &storage.Endpoint{
				ID: "any-endpoint",
			},
			expectedRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := manager.findApplicableRules(tt.drift, tt.endpoint)
			assert.Len(t, rules, tt.expectedRules)
		})
	}
}

func TestCreateAlertMessage(t *testing.T) {
	manager := &DefaultAlertManager{}

	drift := &storage.Drift{
		ID:          1,
		Severity:    "high",
		Description: "Field removed",
		FieldPath:   "$.user.email",
		BeforeValue: "test@example.com",
		AfterValue:  "",
		DetectedAt:  time.Now(),
	}

	endpoint := &storage.Endpoint{
		ID:     "test-endpoint",
		URL:    "https://api.example.com/users",
		Method: "GET",
	}

	message := manager.createAlertMessage(drift, endpoint)

	assert.Equal(t, "API Drift Detected: https://api.example.com/users", message.Title)
	assert.Equal(t, "Field removed", message.Summary)
	assert.Equal(t, "high", message.Severity)
	assert.Equal(t, "test-endpoint", message.EndpointID)
	assert.Equal(t, "https://api.example.com/users", message.EndpointURL)
	assert.Len(t, message.Changes, 1)
	assert.Equal(t, "$.user.email", message.Changes[0].Path)
	assert.True(t, message.Changes[0].Breaking)
}

func TestConvertDriftResult(t *testing.T) {
	manager := &DefaultAlertManager{}

	endpoint := &storage.Endpoint{
		ID: "test-endpoint",
	}

	driftResult := &drift.DiffResult{
		HasChanges: true,
		StructuralChanges: []drift.StructuralChange{
			{
				Type:        drift.ChangeTypeFieldRemoved,
				Path:        "$.user.email",
				Description: "Field removed",
				Severity:    drift.SeverityHigh,
				OldValue:    "test@example.com",
			},
		},
		DataChanges: []drift.DataChange{
			{
				Path:        "$.user.name",
				OldValue:    "John",
				NewValue:    "Jane",
				ChangeType:  drift.ChangeTypeFieldModified,
				Severity:    drift.SeverityMedium,
				Description: "Name changed",
			},
		},
		PerformanceChanges: &drift.PerformanceChange{
			ResponseTimeDelta: 500 * time.Millisecond,
			Severity:          drift.SeverityLow,
			Description:       "Response time increased",
		},
	}

	drifts := manager.convertDriftResult(driftResult, endpoint)
	assert.Len(t, drifts, 3) // 1 structural + 1 data + 1 performance

	// Check structural change
	assert.Equal(t, "field_removed", drifts[0].DriftType)
	assert.Equal(t, "high", drifts[0].Severity)
	assert.Equal(t, "$.user.email", drifts[0].FieldPath)

	// Check data change
	assert.Equal(t, "field_modified", drifts[1].DriftType)
	assert.Equal(t, "medium", drifts[1].Severity)
	assert.Equal(t, "$.user.name", drifts[1].FieldPath)

	// Check performance change
	assert.Equal(t, "performance_change", drifts[2].DriftType)
	assert.Equal(t, "low", drifts[2].Severity)
	assert.Equal(t, "$.response_time", drifts[2].FieldPath)
}
