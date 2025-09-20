package drift

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiffEngine(t *testing.T) {
	engine := NewDiffEngine()
	assert.NotNil(t, engine)
	assert.IsType(t, &DefaultDiffEngine{}, engine)
}

func TestCompareResponses_StatusCodeChange(t *testing.T) {
	engine := NewDiffEngine()

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"status": "ok"}`),
		Timestamp:  time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 500,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"error": "internal server error"}`),
		Timestamp:  time.Now(),
	}

	result, err := engine.CompareResponses(previous, current)
	require.NoError(t, err)
	assert.True(t, result.HasChanges)

	// Should detect status code change plus body changes (status field removed, error field added)
	assert.GreaterOrEqual(t, len(result.StructuralChanges), 1)
	assert.GreaterOrEqual(t, len(result.BreakingChanges), 1)

	// Find the status code change
	var statusChange *StructuralChange
	for _, change := range result.StructuralChanges {
		if change.Type == ChangeTypeStatusChange {
			statusChange = &change
			break
		}
	}

	require.NotNil(t, statusChange, "Should detect status code change")
	assert.Equal(t, ChangeTypeStatusChange, statusChange.Type)
	assert.Equal(t, "$.status_code", statusChange.Path)
	assert.Equal(t, 200, statusChange.OldValue)
	assert.Equal(t, 500, statusChange.NewValue)
	assert.Equal(t, SeverityCritical, statusChange.Severity)
	assert.True(t, statusChange.Breaking)
}

func TestCompareResponses_HeaderChanges(t *testing.T) {
	engine := NewDiffEngine()

	tests := []struct {
		name             string
		previousHeaders  map[string]string
		currentHeaders   map[string]string
		expectedChanges  int
		expectedBreaking int
	}{
		{
			name:             "Header removed",
			previousHeaders:  map[string]string{"Content-Type": "application/json", "X-Custom": "value"},
			currentHeaders:   map[string]string{"Content-Type": "application/json"},
			expectedChanges:  1,
			expectedBreaking: 0, // X-Custom is not a breaking header
		},
		{
			name:             "Critical header removed",
			previousHeaders:  map[string]string{"Content-Type": "application/json", "ETag": "123"},
			currentHeaders:   map[string]string{"Content-Type": "application/json"},
			expectedChanges:  1,
			expectedBreaking: 1, // ETag removal is breaking
		},
		{
			name:             "Header added",
			previousHeaders:  map[string]string{"Content-Type": "application/json"},
			currentHeaders:   map[string]string{"Content-Type": "application/json", "X-New": "value"},
			expectedChanges:  1,
			expectedBreaking: 0, // Adding headers is not breaking
		},
		{
			name:             "Header value changed",
			previousHeaders:  map[string]string{"Content-Type": "application/json"},
			currentHeaders:   map[string]string{"Content-Type": "application/xml"},
			expectedChanges:  1,
			expectedBreaking: 0, // Header value changes are tracked as data changes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &Response{
				StatusCode: 200,
				Headers:    tt.previousHeaders,
				Body:       []byte(`{}`),
				Timestamp:  time.Now().Add(-time.Hour),
			}

			current := &Response{
				StatusCode: 200,
				Headers:    tt.currentHeaders,
				Body:       []byte(`{}`),
				Timestamp:  time.Now(),
			}

			result, err := engine.CompareResponses(previous, current)
			require.NoError(t, err)

			totalChanges := len(result.StructuralChanges) + len(result.DataChanges)
			assert.Equal(t, tt.expectedChanges, totalChanges, "Expected %d changes, got %d", tt.expectedChanges, totalChanges)
			assert.Equal(t, tt.expectedBreaking, len(result.BreakingChanges), "Expected %d breaking changes, got %d", tt.expectedBreaking, len(result.BreakingChanges))
		})
	}
}

func TestCompareResponses_BodyChanges(t *testing.T) {
	engine := NewDiffEngine()

	tests := []struct {
		name             string
		previousBody     string
		currentBody      string
		expectedChanges  int
		expectedBreaking int
		expectedSeverity Severity
	}{
		{
			name:             "Field added",
			previousBody:     `{"name": "John"}`,
			currentBody:      `{"name": "John", "age": 30}`,
			expectedChanges:  1,
			expectedBreaking: 0,
			expectedSeverity: SeverityLow,
		},
		{
			name:             "Field removed",
			previousBody:     `{"name": "John", "age": 30}`,
			currentBody:      `{"name": "John"}`,
			expectedChanges:  1,
			expectedBreaking: 1,
			expectedSeverity: SeverityHigh,
		},
		{
			name:             "Field value changed",
			previousBody:     `{"name": "John", "age": 30}`,
			currentBody:      `{"name": "Jane", "age": 30}`,
			expectedChanges:  1,
			expectedBreaking: 0,
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "Field type changed",
			previousBody:     `{"age": 30}`,
			currentBody:      `{"age": "30"}`,
			expectedChanges:  1,
			expectedBreaking: 1,
			expectedSeverity: SeverityCritical,
		},
		{
			name:             "Critical field removed",
			previousBody:     `{"id": "123", "name": "John"}`,
			currentBody:      `{"name": "John"}`,
			expectedChanges:  1,
			expectedBreaking: 1,
			expectedSeverity: SeverityCritical,
		},
		{
			name:             "Array length changed",
			previousBody:     `{"items": [1, 2, 3]}`,
			currentBody:      `{"items": [1, 2]}`,
			expectedChanges:  2, // Array length change + item removed
			expectedBreaking: 1, // Item removal is breaking
			expectedSeverity: SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &Response{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       []byte(tt.previousBody),
				Timestamp:  time.Now().Add(-time.Hour),
			}

			current := &Response{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       []byte(tt.currentBody),
				Timestamp:  time.Now(),
			}

			result, err := engine.CompareResponses(previous, current)
			require.NoError(t, err)

			if tt.expectedChanges > 0 {
				assert.True(t, result.HasChanges)
				totalChanges := len(result.StructuralChanges) + len(result.DataChanges)
				assert.Equal(t, tt.expectedChanges, totalChanges, "Expected %d changes, got %d", tt.expectedChanges, totalChanges)
				assert.Equal(t, tt.expectedBreaking, len(result.BreakingChanges), "Expected %d breaking changes, got %d", tt.expectedBreaking, len(result.BreakingChanges))
			} else {
				assert.False(t, result.HasChanges)
			}
		})
	}
}

func TestCompareResponses_PerformanceChanges(t *testing.T) {
	engine := NewDiffEngine()

	tests := []struct {
		name             string
		previousTime     time.Duration
		currentTime      time.Duration
		expectChange     bool
		expectedSeverity Severity
	}{
		{
			name:         "No significant change",
			previousTime: 100 * time.Millisecond,
			currentTime:  105 * time.Millisecond,
			expectChange: false,
		},
		{
			name:             "Medium improvement",
			previousTime:     500 * time.Millisecond,
			currentTime:      400 * time.Millisecond,
			expectChange:     true,
			expectedSeverity: SeverityMedium, // -20% change is medium, 100ms absolute
		},
		{
			name:             "High degradation",
			previousTime:     200 * time.Millisecond,
			currentTime:      320 * time.Millisecond,
			expectChange:     true,
			expectedSeverity: SeverityHigh, // 60% change is high, 120ms absolute
		},
		{
			name:             "Significant improvement",
			previousTime:     200 * time.Millisecond,
			currentTime:      100 * time.Millisecond,
			expectChange:     true,
			expectedSeverity: SeverityCritical, // -50% change is critical
		},
		{
			name:             "Significant degradation",
			previousTime:     100 * time.Millisecond,
			currentTime:      250 * time.Millisecond,
			expectChange:     true,
			expectedSeverity: SeverityCritical, // 150% change is critical
		},
		{
			name:             "Critical degradation",
			previousTime:     100 * time.Millisecond,
			currentTime:      300 * time.Millisecond,
			expectChange:     true,
			expectedSeverity: SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &Response{
				StatusCode:   200,
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"status": "ok"}`),
				ResponseTime: tt.previousTime,
				Timestamp:    time.Now().Add(-time.Hour),
			}

			current := &Response{
				StatusCode:   200,
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"status": "ok"}`),
				ResponseTime: tt.currentTime,
				Timestamp:    time.Now(),
			}

			result, err := engine.CompareResponses(previous, current)
			require.NoError(t, err)

			if tt.expectChange {
				assert.NotNil(t, result.PerformanceChanges)
				assert.Equal(t, tt.expectedSeverity, result.PerformanceChanges.Severity)
				assert.Equal(t, tt.currentTime-tt.previousTime, result.PerformanceChanges.ResponseTimeDelta)
			} else {
				assert.Nil(t, result.PerformanceChanges)
			}
		})
	}
}

func TestClassifyChange(t *testing.T) {
	engine := NewDiffEngine().(*DefaultDiffEngine)

	tests := []struct {
		name             string
		diff             FieldDiff
		expectedCategory ChangeCategory
		expectedBreaking bool
		expectedSeverity Severity
		expectedImpact   ImpactLevel
	}{
		{
			name: "Field added",
			diff: FieldDiff{
				Path:     "$.new_field",
				Type:     DiffTypeAdded,
				NewValue: "value",
				Severity: SeverityLow,
			},
			expectedCategory: ChangeCategoryStructural,
			expectedBreaking: false,
			expectedSeverity: SeverityLow,
			expectedImpact:   ImpactLevelMinor,
		},
		{
			name: "Critical field removed",
			diff: FieldDiff{
				Path:     "$.id",
				Type:     DiffTypeRemoved,
				OldValue: "123",
				Severity: SeverityCritical,
			},
			expectedCategory: ChangeCategoryStructural,
			expectedBreaking: true,
			expectedSeverity: SeverityCritical,
			expectedImpact:   ImpactLevelCritical,
		},
		{
			name: "Type changed",
			diff: FieldDiff{
				Path:     "$.age",
				Type:     DiffTypeTypeChanged,
				OldValue: 30,
				NewValue: "30",
				Severity: SeverityCritical,
			},
			expectedCategory: ChangeCategoryStructural,
			expectedBreaking: true,
			expectedSeverity: SeverityCritical,
			expectedImpact:   ImpactLevelCritical,
		},
		{
			name: "Value modified",
			diff: FieldDiff{
				Path:     "$.name",
				Type:     DiffTypeModified,
				OldValue: "John",
				NewValue: "Jane",
				Severity: SeverityMedium,
			},
			expectedCategory: ChangeCategoryData,
			expectedBreaking: false,
			expectedSeverity: SeverityMedium,
			expectedImpact:   ImpactLevelModerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := engine.ClassifyChange(&tt.diff)

			assert.Equal(t, tt.expectedCategory, classification.Category)
			assert.Equal(t, tt.expectedBreaking, classification.Breaking)
			assert.Equal(t, tt.expectedSeverity, classification.Severity)
			assert.Equal(t, tt.expectedImpact, classification.Impact)
			assert.Greater(t, classification.Confidence, 0.0)
			assert.NotEmpty(t, classification.Reasoning)
		})
	}
}

func TestAssessSeverity(t *testing.T) {
	engine := NewDiffEngine().(*DefaultDiffEngine)

	tests := []struct {
		name             string
		diff             FieldDiff
		context          *ChangeContext
		expectedSeverity Severity
	}{
		{
			name: "Required field removal",
			diff: FieldDiff{
				Path:     "$.name",
				Type:     DiffTypeRemoved,
				Severity: SeverityHigh,
			},
			context: &ChangeContext{
				FieldPath:  "$.name",
				IsRequired: true,
			},
			expectedSeverity: SeverityHigh, // Already high, no change
		},
		{
			name: "Optional field removal becomes more severe when required",
			diff: FieldDiff{
				Path:     "$.optional_field",
				Type:     DiffTypeRemoved,
				Severity: SeverityMedium,
			},
			context: &ChangeContext{
				FieldPath:  "$.optional_field",
				IsRequired: true,
			},
			expectedSeverity: SeverityHigh, // Upgraded from medium
		},
		{
			name: "Critical field pattern",
			diff: FieldDiff{
				Path:     "$.user_id",
				Type:     DiffTypeModified,
				Severity: SeverityMedium,
			},
			context: &ChangeContext{
				FieldPath: "$.user_id",
			},
			expectedSeverity: SeverityHigh, // Upgraded due to critical pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := engine.AssessSeverity(&tt.diff, tt.context)
			assert.Equal(t, tt.expectedSeverity, severity)
		})
	}
}

func TestAnalyzeTrends(t *testing.T) {
	engine := NewDiffEngine()

	// Create a series of responses with changes
	responses := []*Response{
		{
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"version": 1, "data": "test"}`),
			ResponseTime: 100 * time.Millisecond,
			Timestamp:    time.Now().Add(-3 * time.Hour),
		},
		{
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"version": 1, "data": "test", "new_field": "added"}`),
			ResponseTime: 120 * time.Millisecond,
			Timestamp:    time.Now().Add(-2 * time.Hour),
		},
		{
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"version": 2, "data": "test", "new_field": "added"}`),
			ResponseTime: 150 * time.Millisecond,
			Timestamp:    time.Now().Add(-1 * time.Hour),
		},
		{
			StatusCode:   200,
			Headers:      map[string]string{"Content-Type": "application/json"},
			Body:         []byte(`{"version": 2, "data": "updated", "new_field": "added"}`),
			ResponseTime: 180 * time.Millisecond,
			Timestamp:    time.Now(),
		},
	}

	analysis, err := engine.AnalyzeTrends(responses)
	require.NoError(t, err)
	assert.NotNil(t, analysis)

	assert.Equal(t, len(responses), analysis.TotalResponses)
	assert.Greater(t, analysis.Period, time.Duration(0))
	assert.Greater(t, analysis.ChangeFrequency, 0.0)
	assert.Less(t, analysis.StabilityScore, 1.0)
	assert.NotNil(t, analysis.PerformanceTrend)
	assert.Equal(t, TrendDirectionDegrading, analysis.PerformanceTrend.Trend)
}

func TestAnalyzeTrends_InsufficientData(t *testing.T) {
	engine := NewDiffEngine()

	// Test with insufficient data
	responses := []*Response{
		{
			StatusCode: 200,
			Body:       []byte(`{"test": "data"}`),
			Timestamp:  time.Now(),
		},
	}

	_, err := engine.AnalyzeTrends(responses)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "need at least 2 responses")
}

func TestCompareResponses_NilInputs(t *testing.T) {
	engine := NewDiffEngine()

	// Test nil previous response
	_, err := engine.CompareResponses(nil, &Response{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both responses must be non-nil")

	// Test nil current response
	_, err = engine.CompareResponses(&Response{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both responses must be non-nil")
}

func TestCompareResponses_InvalidJSON(t *testing.T) {
	engine := NewDiffEngine()

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"valid": "json"}`),
		Timestamp:  time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{invalid json`),
		Timestamp:  time.Now(),
	}

	_, err := engine.CompareResponses(previous, current)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compare response bodies")
}

func TestDiffSummaryGeneration(t *testing.T) {
	engine := NewDiffEngine()

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123", "name": "John", "age": 30}`),
		Timestamp:  time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 500,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"error": "server error", "new_field": "added"}`),
		Timestamp:  time.Now(),
	}

	result, err := engine.CompareResponses(previous, current)
	require.NoError(t, err)

	assert.NotNil(t, result.Summary)
	assert.Greater(t, result.Summary.TotalChanges, 0)
	assert.Greater(t, result.Summary.BreakingChanges, 0)
	assert.Greater(t, result.Summary.CriticalChanges, 0)

	// Verify summary counts match actual changes
	expectedTotal := len(result.StructuralChanges) + len(result.DataChanges)
	if result.PerformanceChanges != nil {
		expectedTotal++
	}
	assert.Equal(t, expectedTotal, result.Summary.TotalChanges)
}

func TestCriticalFieldDetection(t *testing.T) {
	engine := NewDiffEngine().(*DefaultDiffEngine)

	tests := []struct {
		path       string
		isCritical bool
	}{
		{"$.id", true},
		{"$.user_id", true},
		{"$.uuid", true},
		{"$.api_key", true},
		{"$.token", true},
		{"$.version", true},
		{"$.status", true},
		{"$.error_code", true},
		{"$.name", false},
		{"$.description", false},
		{"$.metadata", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := engine.isCriticalField(tt.path)
			assert.Equal(t, tt.isCritical, result)
		})
	}
}

func TestSeverityMapping(t *testing.T) {
	engine := NewDiffEngine().(*DefaultDiffEngine)

	tests := []struct {
		severity       Severity
		expectedImpact ImpactLevel
	}{
		{SeverityCritical, ImpactLevelCritical},
		{SeverityHigh, ImpactLevelMajor},
		{SeverityMedium, ImpactLevelModerate},
		{SeverityLow, ImpactLevelMinor},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			impact := engine.mapSeverityToImpact(tt.severity)
			assert.Equal(t, tt.expectedImpact, impact)
		})
	}
}

func TestNestedObjectComparison(t *testing.T) {
	engine := NewDiffEngine()

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body: []byte(`{
			"user": {
				"id": "123",
				"profile": {
					"name": "John",
					"age": 30
				}
			}
		}`),
		Timestamp: time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body: []byte(`{
			"user": {
				"id": "123",
				"profile": {
					"name": "Jane",
					"age": 30,
					"email": "jane@example.com"
				}
			}
		}`),
		Timestamp: time.Now(),
	}

	result, err := engine.CompareResponses(previous, current)
	require.NoError(t, err)

	assert.True(t, result.HasChanges)

	// Should detect name change and email addition
	totalChanges := len(result.StructuralChanges) + len(result.DataChanges)
	assert.Equal(t, 2, totalChanges)

	// Find the changes
	var nameChange, emailChange bool
	for _, change := range result.DataChanges {
		if change.Path == "$.user.profile.name" {
			nameChange = true
			assert.Equal(t, "John", change.OldValue)
			assert.Equal(t, "Jane", change.NewValue)
		}
	}

	for _, change := range result.StructuralChanges {
		if change.Path == "$.user.profile.email" {
			emailChange = true
			assert.Equal(t, ChangeTypeFieldAdded, change.Type)
			assert.Equal(t, "jane@example.com", change.NewValue)
		}
	}

	assert.True(t, nameChange, "Should detect name change")
	assert.True(t, emailChange, "Should detect email addition")
}

// Benchmark tests
func BenchmarkCompareResponses_SimpleJSON(b *testing.B) {
	engine := NewDiffEngine()

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123", "name": "John", "age": 30}`),
		Timestamp:  time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123", "name": "Jane", "age": 31}`),
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.CompareResponses(previous, current)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareResponses_ComplexJSON(b *testing.B) {
	engine := NewDiffEngine()

	// Create complex JSON structures
	complexData := map[string]interface{}{
		"users": []map[string]interface{}{
			{"id": "1", "name": "John", "profile": map[string]interface{}{"age": 30, "city": "NYC"}},
			{"id": "2", "name": "Jane", "profile": map[string]interface{}{"age": 25, "city": "LA"}},
		},
		"metadata": map[string]interface{}{
			"total":   2,
			"page":    1,
			"filters": []string{"active", "verified"},
		},
	}

	previousJSON, _ := json.Marshal(complexData)

	// Modify the data slightly
	complexData["metadata"].(map[string]interface{})["total"] = 3
	currentJSON, _ := json.Marshal(complexData)

	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       previousJSON,
		Timestamp:  time.Now().Add(-time.Hour),
	}

	current := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       currentJSON,
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.CompareResponses(previous, current)
		if err != nil {
			b.Fatal(err)
		}
	}
}
