package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		name     string
		period   string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "24 hours",
			period:   "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "1 day",
			period:   "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "7 days",
			period:   "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "week",
			period:   "week",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "30 days",
			period:   "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "month",
			period:   "month",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "custom duration",
			period:   "48h",
			expected: 48 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "invalid period",
			period:   "invalid",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePeriod(tt.period)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGenerateDriftSummary(t *testing.T) {
	now := time.Now()
	drifts := []*storage.Drift{
		{
			ID:           1,
			EndpointID:   "api-1",
			DetectedAt:   now,
			DriftType:    "schema_change",
			Severity:     "high",
			Description:  "Field removed",
			Acknowledged: true,
		},
		{
			ID:           2,
			EndpointID:   "api-1",
			DetectedAt:   now.Add(-1 * time.Hour),
			DriftType:    "field_added",
			Severity:     "low",
			Description:  "New field added",
			Acknowledged: false,
		},
		{
			ID:           3,
			EndpointID:   "api-2",
			DetectedAt:   now.Add(-2 * time.Hour),
			DriftType:    "type_change",
			Severity:     "critical",
			Description:  "Type changed",
			Acknowledged: false,
		},
	}

	summary := generateDriftSummary(drifts)

	assert.Equal(t, 3, summary.TotalDrifts)
	assert.InDelta(t, 33.33, summary.AcknowledgedRate, 0.01) // 1 out of 3 acknowledged

	// Check severity breakdown
	assert.Equal(t, 1, summary.BySeverity["high"])
	assert.Equal(t, 1, summary.BySeverity["low"])
	assert.Equal(t, 1, summary.BySeverity["critical"])

	// Check endpoint breakdown
	assert.Equal(t, 2, summary.ByEndpoint["api-1"])
	assert.Equal(t, 1, summary.ByEndpoint["api-2"])

	// Check type breakdown
	assert.Equal(t, 1, summary.ByType["schema_change"])
	assert.Equal(t, 1, summary.ByType["field_added"])
	assert.Equal(t, 1, summary.ByType["type_change"])
}

func TestGenerateDriftTrends(t *testing.T) {
	now := time.Now()
	startTime := now.Add(-7 * 24 * time.Hour)

	drifts := []*storage.Drift{
		{
			ID:         1,
			EndpointID: "api-1",
			DetectedAt: now,
			Severity:   "high",
		},
		{
			ID:         2,
			EndpointID: "api-1",
			DetectedAt: now.Add(-1 * 24 * time.Hour),
			Severity:   "low",
		},
		{
			ID:         3,
			EndpointID: "api-2",
			DetectedAt: now.Add(-1 * 24 * time.Hour),
			Severity:   "critical",
		},
	}

	trends := generateDriftTrends(drifts, startTime, now)

	// Check daily breakdown
	assert.Len(t, trends.DailyBreakdown, 2) // 2 different days

	// Check endpoint activity
	assert.Len(t, trends.MostActiveEndpoints, 2) // 2 different endpoints

	// Find api-1 activity
	var api1Activity *EndpointActivity
	for i := range trends.MostActiveEndpoints {
		if trends.MostActiveEndpoints[i].EndpointID == "api-1" {
			api1Activity = &trends.MostActiveEndpoints[i]
			break
		}
	}
	require.NotNil(t, api1Activity)
	assert.Equal(t, 2, api1Activity.Count)
	assert.Equal(t, 1, api1Activity.Severe) // Only 1 high severity
}

func TestCalculateEndpointStatus(t *testing.T) {
	tests := []struct {
		name     string
		runs     []*storage.MonitoringRun
		expected string
	}{
		{
			name:     "no runs",
			runs:     []*storage.MonitoringRun{},
			expected: "unknown",
		},
		{
			name: "all healthy",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 200},
				{ResponseStatus: 201},
				{ResponseStatus: 204},
			},
			expected: "healthy",
		},
		{
			name: "all unhealthy",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 500},
				{ResponseStatus: 404},
				{ResponseStatus: 503},
			},
			expected: "unhealthy",
		},
		{
			name: "mostly healthy",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 200},
				{ResponseStatus: 200},
				{ResponseStatus: 200},
				{ResponseStatus: 200},
				{ResponseStatus: 500}, // 1 failure out of 5
			},
			expected: "healthy", // 80% success rate
		},
		{
			name: "mostly unhealthy",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 200},
				{ResponseStatus: 500},
				{ResponseStatus: 500},
				{ResponseStatus: 500},
				{ResponseStatus: 500}, // 20% success rate
			},
			expected: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateEndpointStatus(tt.runs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		runs     []*storage.MonitoringRun
		expected float64
	}{
		{
			name:     "no runs",
			runs:     []*storage.MonitoringRun{},
			expected: 0.0,
		},
		{
			name: "all successful",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 200},
				{ResponseStatus: 201},
				{ResponseStatus: 204},
			},
			expected: 100.0,
		},
		{
			name: "all failed",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 500},
				{ResponseStatus: 404},
				{ResponseStatus: 503},
			},
			expected: 0.0,
		},
		{
			name: "mixed results",
			runs: []*storage.MonitoringRun{
				{ResponseStatus: 200},
				{ResponseStatus: 500},
				{ResponseStatus: 200},
				{ResponseStatus: 404},
			},
			expected: 50.0, // 2 out of 4 successful
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSuccessRate(tt.runs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPeriod(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
			expected: "1 hours",
		},
		{
			name:     "12 hours",
			duration: 12 * time.Hour,
			expected: "12 hours",
		},
		{
			name:     "1 day",
			duration: 24 * time.Hour,
			expected: "1 day",
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: "7 days",
		},
		{
			name:     "30 days",
			duration: 30 * 24 * time.Hour,
			expected: "30 days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPeriod(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateDriftReport(t *testing.T) {
	now := time.Now()
	period := 24 * time.Hour

	drifts := []*storage.Drift{
		{
			ID:           1,
			EndpointID:   "api-1",
			DetectedAt:   now,
			DriftType:    "schema_change",
			Severity:     "high",
			Description:  "Field removed",
			Acknowledged: true,
		},
		{
			ID:           2,
			EndpointID:   "api-2",
			DetectedAt:   now.Add(-1 * time.Hour),
			DriftType:    "field_added",
			Severity:     "low",
			Description:  "New field added",
			Acknowledged: false,
		},
	}

	report := generateDriftReport(drifts, period)

	assert.Equal(t, "1 day", report.Period)
	assert.Equal(t, 2, len(report.Drifts))
	assert.Equal(t, 2, report.Summary.TotalDrifts)
	assert.Equal(t, 50.0, report.Summary.AcknowledgedRate)
	assert.NotEmpty(t, report.Trends.DailyBreakdown)
	assert.NotEmpty(t, report.Trends.MostActiveEndpoints)
}

func TestOutputReportJSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	report := &DriftReport{
		Period:    "1 day",
		StartTime: time.Now().Add(-24 * time.Hour),
		EndTime:   time.Now(),
		Summary: DriftSummary{
			TotalDrifts:      2,
			AcknowledgedRate: 50.0,
			BySeverity:       map[string]int{"high": 1, "low": 1},
		},
		Drifts: []*storage.Drift{},
		Trends: DriftTrends{},
	}

	err := outputReportJSON(report)
	assert.NoError(t, err)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify JSON structure
	var parsedReport DriftReport
	err = json.Unmarshal([]byte(output), &parsedReport)
	assert.NoError(t, err)
	assert.Equal(t, "1 day", parsedReport.Period)
	assert.Equal(t, 2, parsedReport.Summary.TotalDrifts)
}

func TestOutputReportTable(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	now := time.Now()
	report := &DriftReport{
		Period:    "1 day",
		StartTime: now.Add(-24 * time.Hour),
		EndTime:   now,
		Summary: DriftSummary{
			TotalDrifts:      2,
			AcknowledgedRate: 50.0,
			BySeverity:       map[string]int{"high": 1, "low": 1},
			ByEndpoint:       map[string]int{"api-1": 1, "api-2": 1},
		},
		Drifts: []*storage.Drift{
			{
				ID:           1,
				EndpointID:   "api-1",
				DetectedAt:   now,
				DriftType:    "schema_change",
				Severity:     "high",
				Description:  "Field removed",
				Acknowledged: true,
			},
		},
		Trends: DriftTrends{
			DailyBreakdown: []DayBreakdown{
				{Date: now.Format("2006-01-02"), Count: 2, Severe: 1},
			},
		},
	}

	outputReportTable(report)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify table content
	assert.Contains(t, output, "DriftWatch Report - 1 day")
	assert.Contains(t, output, "Total Drifts: 2")
	assert.Contains(t, output, "Acknowledged Rate: 50.0%")
	assert.Contains(t, output, "High: 1")
	assert.Contains(t, output, "api-1: 1")
	assert.Contains(t, output, "RECENT DRIFTS")
	assert.Contains(t, output, "schema_change")
}

func TestGenerateStatusSummary(t *testing.T) {
	endpoints := []EndpointStatus{
		{Status: "healthy"},
		{Status: "healthy"},
		{Status: "unhealthy"},
		{Status: "unknown"},
	}

	summary := generateStatusSummary(endpoints)

	assert.Equal(t, 4, summary.TotalEndpoints)
	assert.Equal(t, 2, summary.HealthyEndpoints)
	assert.Equal(t, 1, summary.UnhealthyEndpoints)
	assert.Equal(t, 1, summary.UnknownEndpoints)
}

func TestOutputStatusTable(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	now := time.Now()
	report := &StatusReport{
		GeneratedAt: now,
		Summary: StatusSummary{
			TotalEndpoints:     2,
			HealthyEndpoints:   1,
			UnhealthyEndpoints: 1,
			UnknownEndpoints:   0,
		},
		Endpoints: []EndpointStatus{
			{
				ID:               "api-1",
				URL:              "https://api.example.com/v1/users",
				Method:           "GET",
				Status:           "healthy",
				LastChecked:      now.Add(-5 * time.Minute),
				LastResponseTime: 150,
				SuccessRate:      95.5,
				RecentDrifts:     2,
				Enabled:          true,
			},
			{
				ID:               "api-2",
				URL:              "https://api.example.com/v1/orders",
				Method:           "POST",
				Status:           "unhealthy",
				LastChecked:      now.Add(-10 * time.Minute),
				LastResponseTime: 5000,
				SuccessRate:      45.0,
				RecentDrifts:     5,
				Enabled:          true,
			},
		},
	}

	outputStatusTable(report)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify table content
	assert.Contains(t, output, "DriftWatch Status Report")
	assert.Contains(t, output, "Total Endpoints: 2")
	assert.Contains(t, output, "Healthy: 1 | Unhealthy: 1 | Unknown: 0")
	assert.Contains(t, output, "ENDPOINT STATUS")
	assert.Contains(t, output, "api-1")
	assert.Contains(t, output, "api-2")
	assert.Contains(t, output, "GET")
	assert.Contains(t, output, "POST")
	assert.Contains(t, output, "Healthy")
	assert.Contains(t, output, "Unhealthy")
	assert.Contains(t, output, "150ms")
	assert.Contains(t, output, "5000ms")
	assert.Contains(t, output, "95.5%")
	assert.Contains(t, output, "45.0%")
}

func TestExportDriftsCSV(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_drifts_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	now := time.Now()
	drifts := []*storage.Drift{
		{
			ID:           1,
			EndpointID:   "api-1",
			DetectedAt:   now,
			DriftType:    "schema_change",
			Severity:     "high",
			Description:  "Field removed",
			BeforeValue:  "old_value",
			AfterValue:   "new_value",
			FieldPath:    "user.name",
			Acknowledged: true,
		},
		{
			ID:           2,
			EndpointID:   "api-2",
			DetectedAt:   now.Add(-1 * time.Hour),
			DriftType:    "field_added",
			Severity:     "low",
			Description:  "New field added",
			BeforeValue:  "",
			AfterValue:   "new_field_value",
			FieldPath:    "user.email",
			Acknowledged: false,
		},
	}

	err = exportDriftsCSV(drifts, tmpFile)
	assert.NoError(t, err)

	// Close and read file
	tmpFile.Close()
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	csvContent := string(content)

	// Verify CSV structure
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	assert.Len(t, lines, 3) // Header + 2 data rows

	// Verify header
	assert.Contains(t, lines[0], "ID,EndpointID,DetectedAt,DriftType,Severity")

	// Verify data rows
	assert.Contains(t, lines[1], "1,api-1")
	assert.Contains(t, lines[1], "schema_change,high")
	assert.Contains(t, lines[1], "true")

	assert.Contains(t, lines[2], "2,api-2")
	assert.Contains(t, lines[2], "field_added,low")
	assert.Contains(t, lines[2], "false")
}

func TestExportRunsCSV(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_runs_*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	now := time.Now()
	runs := []*storage.MonitoringRun{
		{
			ID:               1,
			EndpointID:       "api-1",
			Timestamp:        now,
			ResponseStatus:   200,
			ResponseTimeMs:   150,
			ValidationResult: "valid",
		},
		{
			ID:               2,
			EndpointID:       "api-2",
			Timestamp:        now.Add(-1 * time.Hour),
			ResponseStatus:   500,
			ResponseTimeMs:   5000,
			ValidationResult: "error",
		},
	}

	err = exportRunsCSV(runs, tmpFile)
	assert.NoError(t, err)

	// Close and read file
	tmpFile.Close()
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	csvContent := string(content)

	// Verify CSV structure
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	assert.Len(t, lines, 3) // Header + 2 data rows

	// Verify header
	assert.Contains(t, lines[0], "ID,EndpointID,Timestamp,ResponseStatus,ResponseTimeMs")

	// Verify data rows
	assert.Contains(t, lines[1], "1,api-1")
	assert.Contains(t, lines[1], "200,150")

	assert.Contains(t, lines[2], "2,api-2")
	assert.Contains(t, lines[2], "500,5000")
}
