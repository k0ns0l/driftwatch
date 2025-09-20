package drift

import (
	"fmt"
	"time"

	"github.com/k0ns0l/driftwatch/internal/storage"
)

// ExampleDiffEngine_CompareResponses demonstrates basic drift detection
func ExampleDiffEngine_CompareResponses() {
	engine := NewDiffEngine()

	// Previous API response
	previous := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123", "name": "John", "age": 30}`),
		Timestamp:  time.Now().Add(-time.Hour),
	}

	// Current API response with changes
	current := &Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123", "name": "Jane", "age": 30, "email": "jane@example.com"}`),
		Timestamp:  time.Now(),
	}

	result, err := engine.CompareResponses(previous, current)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Changes detected: %t\n", result.HasChanges)
	fmt.Printf("Total changes: %d\n", result.Summary.TotalChanges)
	fmt.Printf("Breaking changes: %d\n", result.Summary.BreakingChanges)

	// Output:
	// Changes detected: true
	// Total changes: 2
	// Breaking changes: 0
}

// ExampleDiffEngine_ClassifyChange demonstrates change classification
func ExampleDiffEngine_ClassifyChange() {
	engine := NewDiffEngine().(*DefaultDiffEngine)

	// Example of a critical field removal
	diff := &FieldDiff{
		Path:     "$.user_id",
		Type:     DiffTypeRemoved,
		OldValue: "user123",
		Severity: SeverityCritical,
	}

	classification := engine.ClassifyChange(diff)

	fmt.Printf("Category: %s\n", classification.Category)
	fmt.Printf("Breaking: %t\n", classification.Breaking)
	fmt.Printf("Severity: %s\n", classification.Severity)
	fmt.Printf("Impact: %s\n", classification.Impact)

	// Output:
	// Category: structural
	// Breaking: true
	// Severity: critical
	// Impact: critical
}

// Example_integration demonstrates integration with storage layer
func Example_integration() {
	// This example shows how drift detection integrates with the storage layer
	engine := NewDiffEngine()

	// Simulate monitoring runs from storage
	runs := []*storage.MonitoringRun{
		{
			ID:              1,
			EndpointID:      "api-users",
			Timestamp:       time.Now().Add(-2 * time.Hour),
			ResponseStatus:  200,
			ResponseTimeMs:  150,
			ResponseBody:    `{"users": [{"id": "1", "name": "John"}]}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			ID:              2,
			EndpointID:      "api-users",
			Timestamp:       time.Now().Add(-1 * time.Hour),
			ResponseStatus:  200,
			ResponseTimeMs:  180,
			ResponseBody:    `{"users": [{"id": "1", "name": "John", "email": "john@example.com"}]}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		},
	}

	// Convert storage runs to drift responses
	responses := make([]*Response, len(runs))
	for i, run := range runs {
		responses[i] = &Response{
			StatusCode:   run.ResponseStatus,
			Headers:      run.ResponseHeaders,
			Body:         []byte(run.ResponseBody),
			ResponseTime: time.Duration(run.ResponseTimeMs) * time.Millisecond,
			Timestamp:    run.Timestamp,
		}
	}

	// Compare consecutive responses
	if len(responses) >= 2 {
		result, err := engine.CompareResponses(responses[0], responses[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if result.HasChanges {
			fmt.Printf("Drift detected between responses\n")
			fmt.Printf("Structural changes: %d\n", len(result.StructuralChanges))
			fmt.Printf("Data changes: %d\n", len(result.DataChanges))

			// Convert to storage drift records
			for _, change := range result.StructuralChanges {
				drift := &storage.Drift{
					DriftType: string(change.Type),
					FieldPath: change.Path,
				}

				// Use the drift record (example: could be saved to database)
				fmt.Printf("Drift record created: %s at %s\n",
					drift.DriftType, drift.FieldPath)
			}
		}
	}

	// Output:
	// Drift detected between responses
	// Structural changes: 1
	// Data changes: 0
	// Drift record created: field_added at $.users[0].email
}

// Example_trendAnalysis demonstrates trend analysis capabilities
func Example_trendAnalysis() {
	engine := NewDiffEngine()

	// Create a series of responses showing API evolution
	responses := []*Response{
		{
			StatusCode:   200,
			Body:         []byte(`{"version": "1.0", "data": "stable"}`),
			ResponseTime: 100 * time.Millisecond,
			Timestamp:    time.Now().Add(-4 * time.Hour),
		},
		{
			StatusCode:   200,
			Body:         []byte(`{"version": "1.1", "data": "stable"}`),
			ResponseTime: 110 * time.Millisecond,
			Timestamp:    time.Now().Add(-3 * time.Hour),
		},
		{
			StatusCode:   200,
			Body:         []byte(`{"version": "1.1", "data": "updated", "new_feature": true}`),
			ResponseTime: 120 * time.Millisecond,
			Timestamp:    time.Now().Add(-2 * time.Hour),
		},
		{
			StatusCode:   200,
			Body:         []byte(`{"version": "1.2", "data": "updated", "new_feature": true}`),
			ResponseTime: 130 * time.Millisecond,
			Timestamp:    time.Now().Add(-1 * time.Hour),
		},
	}

	analysis, err := engine.AnalyzeTrends(responses)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Total responses analyzed: %d\n", analysis.TotalResponses)
	fmt.Printf("Change frequency: %.2f\n", analysis.ChangeFrequency)
	fmt.Printf("Stability score: %.2f\n", analysis.StabilityScore)
	fmt.Printf("Performance trend: %s\n", analysis.PerformanceTrend.Trend)

	// Output:
	// Total responses analyzed: 4
	// Change frequency: 1.00
	// Stability score: 0.00
	// Performance trend: degrading
}
