// Package drift provides API drift detection and classification functionality
package drift

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/k0ns0l/driftwatch/internal/validator"
)

// DiffEngine defines the interface for drift detection
type DiffEngine interface {
	CompareResponses(previous, current *Response) (*DiffResult, error)
	AnalyzeTrends(responses []*Response) (*TrendAnalysis, error)
	ClassifyChange(diff *FieldDiff) *ChangeClassification
	AssessSeverity(diff *FieldDiff, context *ChangeContext) Severity
}

// Response represents an HTTP response for drift analysis
type Response struct {
	Headers      map[string]string `json:"headers"`
	Body         []byte            `json:"body"`
	Timestamp    time.Time         `json:"timestamp"`
	ResponseTime time.Duration     `json:"response_time"`
	StatusCode   int               `json:"status_code"`
}

// DiffResult represents the result of comparing two responses
type DiffResult struct {
	StructuralChanges  []StructuralChange `json:"structural_changes"`
	DataChanges        []DataChange       `json:"data_changes"`
	BreakingChanges    []BreakingChange   `json:"breaking_changes"`
	PerformanceChanges *PerformanceChange `json:"performance_changes,omitempty"`
	Summary            *DiffSummary       `json:"summary"`
	HasChanges         bool               `json:"has_changes"`
}

// StructuralChange represents a change in the API structure
type StructuralChange struct {
	Path        string      `json:"path"`
	Description string      `json:"description"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	Type        ChangeType  `json:"type"`
	Severity    Severity    `json:"severity"`
	Breaking    bool        `json:"breaking"`
}

// DataChange represents a change in data values
type DataChange struct {
	Path        string      `json:"path"`
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	ChangeType  ChangeType  `json:"change_type"`
	Severity    Severity    `json:"severity"`
	Description string      `json:"description"`
}

// PerformanceChange represents a change in performance characteristics
type PerformanceChange struct {
	Description       string        `json:"description"`
	ResponseTimeDelta time.Duration `json:"response_time_delta"`
	Severity          Severity      `json:"severity"`
}

// BreakingChange represents a potentially breaking API change
type BreakingChange struct {
	Type        ChangeType  `json:"type"`
	Path        string      `json:"path"`
	Description string      `json:"description"`
	Impact      ImpactLevel `json:"impact"`
	Mitigation  string      `json:"mitigation,omitempty"`
}

// DiffSummary provides a high-level summary of changes
type DiffSummary struct {
	TotalChanges    int `json:"total_changes"`
	BreakingChanges int `json:"breaking_changes"`
	CriticalChanges int `json:"critical_changes"`
	HighChanges     int `json:"high_changes"`
	MediumChanges   int `json:"medium_changes"`
	LowChanges      int `json:"low_changes"`
}

// TrendAnalysis represents analysis of changes over time
type TrendAnalysis struct {
	CommonChanges    []CommonChange    `json:"common_changes"`
	PerformanceTrend *PerformanceTrend `json:"performance_trend,omitempty"`
	Period           time.Duration     `json:"period"`
	ChangeFrequency  float64           `json:"change_frequency"`
	StabilityScore   float64           `json:"stability_score"`
	TotalResponses   int               `json:"total_responses"`
}

// CommonChange represents frequently occurring changes
type CommonChange struct {
	Path       string     `json:"path"`
	LastSeen   time.Time  `json:"last_seen"`
	ChangeType ChangeType `json:"change_type"`
	Frequency  int        `json:"frequency"`
}

// PerformanceTrend represents performance trends over time
type PerformanceTrend struct {
	PercentileChanges   map[string]time.Duration `json:"percentile_changes"`
	AverageResponseTime time.Duration            `json:"average_response_time"`
	Trend               TrendDirection           `json:"trend"`
}

// FieldDiff represents a difference in a specific field
type FieldDiff struct {
	Path     string      `json:"path"`
	Type     DiffType    `json:"type"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
	Severity Severity    `json:"severity"`
}

// ChangeClassification represents the classification of a change
type ChangeClassification struct {
	Reasoning  string         `json:"reasoning"`
	Category   ChangeCategory `json:"category"`
	Severity   Severity       `json:"severity"`
	Impact     ImpactLevel    `json:"impact"`
	Confidence float64        `json:"confidence"`
	Breaking   bool           `json:"breaking"`
}

// ChangeContext provides context for change assessment
type ChangeContext struct {
	FieldPath      string                 `json:"field_path"`
	FieldType      string                 `json:"field_type"`
	SchemaContext  map[string]interface{} `json:"schema_context,omitempty"`
	HistoricalData []interface{}          `json:"historical_data,omitempty"`
	IsRequired     bool                   `json:"is_required"`
}

// Enums and constants
type ChangeType string

const (
	ChangeTypeSchemaChange  ChangeType = "schema_change"
	ChangeTypeFieldAdded    ChangeType = "field_added"
	ChangeTypeFieldRemoved  ChangeType = "field_removed"
	ChangeTypeFieldModified ChangeType = "field_modified"
	ChangeTypeTypeChange    ChangeType = "type_change"
	ChangeTypeValueChange   ChangeType = "value_change"
	ChangeTypeArrayChange   ChangeType = "array_change"
	ChangeTypeStatusChange  ChangeType = "status_change"
	ChangeTypeHeaderChange  ChangeType = "header_change"
)

type DiffType string

const (
	DiffTypeAdded       DiffType = "added"
	DiffTypeRemoved     DiffType = "removed"
	DiffTypeModified    DiffType = "modified"
	DiffTypeTypeChanged DiffType = "type_changed"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type ChangeCategory string

const (
	ChangeCategoryStructural    ChangeCategory = "structural"
	ChangeCategoryData          ChangeCategory = "data"
	ChangeCategoryPerformance   ChangeCategory = "performance"
	ChangeCategoryCompatibility ChangeCategory = "compatibility"
)

type ImpactLevel string

const (
	ImpactLevelNone     ImpactLevel = "none"
	ImpactLevelMinor    ImpactLevel = "minor"
	ImpactLevelModerate ImpactLevel = "moderate"
	ImpactLevelMajor    ImpactLevel = "major"
	ImpactLevelCritical ImpactLevel = "critical"
)

type TrendDirection string

const (
	TrendDirectionImproving TrendDirection = "improving"
	TrendDirectionStable    TrendDirection = "stable"
	TrendDirectionDegrading TrendDirection = "degrading"
)

// DefaultDiffEngine implements the DiffEngine interface
type DefaultDiffEngine struct {
	validator validator.Validator
}

// NewDiffEngine creates a new drift detection engine
func NewDiffEngine() DiffEngine {
	return &DefaultDiffEngine{
		validator: validator.NewValidator(),
	}
}

// CompareResponses compares two responses and detects drift
func (d *DefaultDiffEngine) CompareResponses(previous, current *Response) (*DiffResult, error) {
	if previous == nil || current == nil {
		return nil, fmt.Errorf("both responses must be non-nil")
	}

	result := &DiffResult{
		HasChanges:        false,
		StructuralChanges: []StructuralChange{},
		DataChanges:       []DataChange{},
		BreakingChanges:   []BreakingChange{},
		Summary:           &DiffSummary{},
	}

	// Compare status codes
	d.compareStatusCodes(previous, current, result)

	// Compare headers
	d.compareHeaders(previous, current, result)

	// Compare response bodies
	if err := d.compareResponseBodies(previous, current, result); err != nil {
		return nil, fmt.Errorf("failed to compare response bodies: %w", err)
	}

	// Compare performance
	d.comparePerformance(previous, current, result)

	// Generate summary
	d.generateSummary(result)

	return result, nil
}

// compareStatusCodes compares HTTP status codes
func (d *DefaultDiffEngine) compareStatusCodes(previous, current *Response, result *DiffResult) {
	if previous.StatusCode != current.StatusCode {
		result.HasChanges = true

		change := StructuralChange{
			Type:        ChangeTypeStatusChange,
			Path:        "$.status_code",
			Description: fmt.Sprintf("Status code changed from %d to %d", previous.StatusCode, current.StatusCode),
			OldValue:    previous.StatusCode,
			NewValue:    current.StatusCode,
		}

		// Assess severity and breaking nature
		change.Severity = d.assessStatusCodeSeverity(previous.StatusCode, current.StatusCode)
		change.Breaking = d.isStatusCodeChangeBreaking(previous.StatusCode, current.StatusCode)

		result.StructuralChanges = append(result.StructuralChanges, change)

		if change.Breaking {
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        ChangeTypeStatusChange,
				Path:        "$.status_code",
				Description: change.Description,
				Impact:      d.mapSeverityToImpact(change.Severity),
				Mitigation:  "Update client code to handle the new status code",
			})
		}
	}
}

// compareHeaders compares HTTP headers
func (d *DefaultDiffEngine) compareHeaders(previous, current *Response, result *DiffResult) {
	// Check for removed headers
	for key, oldValue := range previous.Headers {
		if newValue, exists := current.Headers[key]; !exists {
			result.HasChanges = true

			change := StructuralChange{
				Type:        ChangeTypeHeaderChange,
				Path:        fmt.Sprintf("$.headers.%s", key),
				Description: fmt.Sprintf("Header '%s' was removed", key),
				OldValue:    oldValue,
				Severity:    d.assessHeaderRemovalSeverity(key),
				Breaking:    d.isHeaderRemovalBreaking(key),
			}

			result.StructuralChanges = append(result.StructuralChanges, change)

			if change.Breaking {
				result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
					Type:        ChangeTypeHeaderChange,
					Path:        change.Path,
					Description: change.Description,
					Impact:      d.mapSeverityToImpact(change.Severity),
					Mitigation:  fmt.Sprintf("Update client code to handle missing '%s' header", key),
				})
			}
		} else if oldValue != newValue {
			// Header value changed
			result.HasChanges = true

			change := DataChange{
				Path:        fmt.Sprintf("$.headers.%s", key),
				OldValue:    oldValue,
				NewValue:    newValue,
				ChangeType:  ChangeTypeHeaderChange,
				Severity:    d.assessHeaderValueSeverity(key, oldValue, newValue),
				Description: fmt.Sprintf("Header '%s' value changed from '%s' to '%s'", key, oldValue, newValue),
			}

			result.DataChanges = append(result.DataChanges, change)
		}
	}

	// Check for added headers
	for key, newValue := range current.Headers {
		if _, exists := previous.Headers[key]; !exists {
			result.HasChanges = true

			change := StructuralChange{
				Type:        ChangeTypeHeaderChange,
				Path:        fmt.Sprintf("$.headers.%s", key),
				Description: fmt.Sprintf("Header '%s' was added", key),
				NewValue:    newValue,
				Severity:    SeverityLow, // Adding headers is typically non-breaking
				Breaking:    false,
			}

			result.StructuralChanges = append(result.StructuralChanges, change)
		}
	}
}

// compareResponseBodies compares response body content
func (d *DefaultDiffEngine) compareResponseBodies(previous, current *Response, result *DiffResult) error {
	// Parse JSON bodies
	var prevData, currData interface{}

	if len(previous.Body) > 0 {
		if err := json.Unmarshal(previous.Body, &prevData); err != nil {
			return fmt.Errorf("failed to parse previous response body: %w", err)
		}
	}

	if len(current.Body) > 0 {
		if err := json.Unmarshal(current.Body, &currData); err != nil {
			return fmt.Errorf("failed to parse current response body: %w", err)
		}
	}

	// Compare the data structures
	diffs := []FieldDiff{}
	d.compareValues(prevData, currData, "$", &diffs)

	// Process field diffs and categorize them
	for _, diff := range diffs {
		result.HasChanges = true

		classification := d.ClassifyChange(&diff)

		switch classification.Category {
		case ChangeCategoryStructural:
			change := StructuralChange{
				Type:        d.mapDiffTypeToChangeType(diff.Type),
				Path:        diff.Path,
				Description: d.generateChangeDescription(diff),
				Severity:    classification.Severity,
				Breaking:    classification.Breaking,
				OldValue:    diff.OldValue,
				NewValue:    diff.NewValue,
			}

			result.StructuralChanges = append(result.StructuralChanges, change)

			if classification.Breaking {
				result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
					Type:        change.Type,
					Path:        change.Path,
					Description: change.Description,
					Impact:      classification.Impact,
					Mitigation:  d.generateMitigation(diff),
				})
			}

		case ChangeCategoryData:
			change := DataChange{
				Path:        diff.Path,
				OldValue:    diff.OldValue,
				NewValue:    diff.NewValue,
				ChangeType:  d.mapDiffTypeToChangeType(diff.Type),
				Severity:    classification.Severity,
				Description: d.generateChangeDescription(diff),
			}

			result.DataChanges = append(result.DataChanges, change)
		}
	}

	return nil
}

// comparePerformance compares response performance metrics
func (d *DefaultDiffEngine) comparePerformance(previous, current *Response, result *DiffResult) {
	if previous.ResponseTime == 0 || current.ResponseTime == 0 {
		return // Skip if response times are not available
	}

	delta := current.ResponseTime - previous.ResponseTime

	// Only report significant performance changes (>10% change or >100ms absolute)
	threshold := previous.ResponseTime / 10 // 10% threshold
	if threshold < 100*time.Millisecond {
		threshold = 100 * time.Millisecond
	}

	if delta >= threshold || delta <= -threshold {
		result.HasChanges = true

		severity := d.assessPerformanceSeverity(delta, previous.ResponseTime)
		description := d.generatePerformanceDescription(delta, previous.ResponseTime, current.ResponseTime)

		result.PerformanceChanges = &PerformanceChange{
			ResponseTimeDelta: delta,
			Severity:          severity,
			Description:       description,
		}
	}
}

// compareValues recursively compares two values and records differences
func (d *DefaultDiffEngine) compareValues(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	if d.handleNilValues(prev, curr, path, diffs) {
		return
	}

	if d.handleTypeChanges(prev, curr, path, diffs) {
		return
	}

	d.compareValuesByType(prev, curr, path, diffs)
}

// handleNilValues handles comparison when one or both values are nil
func (d *DefaultDiffEngine) handleNilValues(prev, curr interface{}, path string, diffs *[]FieldDiff) bool {
	if prev == nil && curr == nil {
		return true
	}

	if prev == nil {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeAdded,
			NewValue: curr,
			Severity: d.determineSeverity(path, DiffTypeAdded),
		})
		return true
	}

	if curr == nil {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeRemoved,
			OldValue: prev,
			Severity: d.determineSeverity(path, DiffTypeRemoved),
		})
		return true
	}

	return false
}

// handleTypeChanges handles comparison when types are different
func (d *DefaultDiffEngine) handleTypeChanges(prev, curr interface{}, path string, diffs *[]FieldDiff) bool {
	prevType := reflect.TypeOf(prev)
	currType := reflect.TypeOf(curr)

	if prevType != currType {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeTypeChanged,
			OldValue: prev,
			NewValue: curr,
			Severity: SeverityCritical,
		})
		return true
	}

	return false
}

// compareValuesByType compares values based on their type
func (d *DefaultDiffEngine) compareValuesByType(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	switch prevValue := prev.(type) {
	case map[string]interface{}:
		if currValue, ok := curr.(map[string]interface{}); ok {
			d.compareObjects(prevValue, currValue, path, diffs)
		}
	case []interface{}:
		if currValue, ok := curr.([]interface{}); ok {
			d.compareArrays(prevValue, currValue, path, diffs)
		}
	default:
		d.compareScalarValues(prev, curr, path, diffs)
	}
}

// compareObjects compares two object values
func (d *DefaultDiffEngine) compareObjects(prevValue, currValue map[string]interface{}, path string, diffs *[]FieldDiff) {
	// Check for removed fields
	for key, value := range prevValue {
		fieldPath := fmt.Sprintf("%s.%s", path, key)
		if _, exists := currValue[key]; !exists {
			*diffs = append(*diffs, FieldDiff{
				Path:     fieldPath,
				Type:     DiffTypeRemoved,
				OldValue: value,
				Severity: d.determineSeverity(fieldPath, DiffTypeRemoved),
			})
		}
	}

	// Check for added or modified fields
	for key, currFieldValue := range currValue {
		fieldPath := fmt.Sprintf("%s.%s", path, key)
		if prevFieldValue, exists := prevValue[key]; exists {
			d.compareValues(prevFieldValue, currFieldValue, fieldPath, diffs)
		} else {
			*diffs = append(*diffs, FieldDiff{
				Path:     fieldPath,
				Type:     DiffTypeAdded,
				NewValue: currFieldValue,
				Severity: d.determineSeverity(fieldPath, DiffTypeAdded),
			})
		}
	}
}

// compareArrays compares two array values
func (d *DefaultDiffEngine) compareArrays(prevValue, currValue []interface{}, path string, diffs *[]FieldDiff) {
	// Array length change
	if len(prevValue) != len(currValue) {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeModified,
			OldValue: fmt.Sprintf("array length: %d", len(prevValue)),
			NewValue: fmt.Sprintf("array length: %d", len(currValue)),
			Severity: SeverityMedium,
		})
	}

	// Compare array elements
	maxLen := len(prevValue)
	if len(currValue) > maxLen {
		maxLen = len(currValue)
	}

	for i := 0; i < maxLen; i++ {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		var prevItem, currItem interface{}

		if i < len(prevValue) {
			prevItem = prevValue[i]
		}
		if i < len(currValue) {
			currItem = currValue[i]
		}

		d.compareValues(prevItem, currItem, itemPath, diffs)
	}
}

// compareScalarValues compares scalar values
func (d *DefaultDiffEngine) compareScalarValues(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	if !reflect.DeepEqual(prev, curr) {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeModified,
			OldValue: prev,
			NewValue: curr,
			Severity: d.determineSeverity(path, DiffTypeModified),
		})
	}
}

// ClassifyChange classifies a field difference
func (d *DefaultDiffEngine) ClassifyChange(diff *FieldDiff) *ChangeClassification {
	classification := &ChangeClassification{
		Confidence: 0.8, // Default confidence
	}

	// Determine category
	if d.isStructuralChange(diff) {
		classification.Category = ChangeCategoryStructural
	} else {
		classification.Category = ChangeCategoryData
	}

	// Determine if breaking
	classification.Breaking = d.isBreakingChange(diff)

	// Determine severity
	classification.Severity = diff.Severity

	// Determine impact
	classification.Impact = d.mapSeverityToImpact(diff.Severity)

	// Generate reasoning
	classification.Reasoning = d.generateClassificationReasoning(diff)

	return classification
}

// AssessSeverity assesses the severity of a change with additional context
func (d *DefaultDiffEngine) AssessSeverity(diff *FieldDiff, context *ChangeContext) Severity {
	baseSeverity := d.determineSeverity(diff.Path, diff.Type)

	// Adjust severity based on context
	if context != nil {
		if context.IsRequired {
			// Required fields have higher severity
			if baseSeverity == SeverityLow {
				baseSeverity = SeverityMedium
			} else if baseSeverity == SeverityMedium {
				baseSeverity = SeverityHigh
			}
		}

		// Check for critical field patterns
		if d.isCriticalField(context.FieldPath) {
			if baseSeverity < SeverityHigh {
				baseSeverity = SeverityHigh
			}
		}
	}

	return baseSeverity
}

// Helper methods for severity and classification assessment

func (d *DefaultDiffEngine) determineSeverity(path string, diffType DiffType) Severity {
	switch diffType {
	case DiffTypeRemoved:
		if d.isCriticalField(path) {
			return SeverityCritical
		}
		return SeverityHigh
	case DiffTypeTypeChanged:
		return SeverityCritical
	case DiffTypeAdded:
		return SeverityLow
	case DiffTypeModified:
		if d.isCriticalField(path) {
			return SeverityHigh
		}
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func (d *DefaultDiffEngine) isCriticalField(path string) bool {
	criticalPatterns := []string{
		"id", "uuid", "key", "token", "version", "status", "type", "error", "code",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range criticalPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

func (d *DefaultDiffEngine) isStructuralChange(diff *FieldDiff) bool {
	return diff.Type == DiffTypeAdded || diff.Type == DiffTypeRemoved || diff.Type == DiffTypeTypeChanged
}

func (d *DefaultDiffEngine) isBreakingChange(diff *FieldDiff) bool {
	switch diff.Type {
	case DiffTypeRemoved, DiffTypeTypeChanged:
		return true
	case DiffTypeModified:
		return d.isCriticalField(diff.Path)
	default:
		return false
	}
}

func (d *DefaultDiffEngine) assessStatusCodeSeverity(oldCode, newCode int) Severity {
	// 2xx -> non-2xx is critical
	if oldCode >= 200 && oldCode < 300 && (newCode < 200 || newCode >= 300) {
		return SeverityCritical
	}

	// non-2xx -> 2xx is an improvement (medium severity for tracking)
	if (oldCode < 200 || oldCode >= 300) && newCode >= 200 && newCode < 300 {
		return SeverityMedium
	}

	// Within same class (e.g., 4xx -> 4xx)
	return SeverityMedium
}

func (d *DefaultDiffEngine) isStatusCodeChangeBreaking(oldCode, newCode int) bool {
	// Success to error is breaking
	if oldCode >= 200 && oldCode < 300 && (newCode < 200 || newCode >= 300) {
		return true
	}

	return false
}

func (d *DefaultDiffEngine) assessHeaderRemovalSeverity(headerName string) Severity {
	criticalHeaders := []string{
		"content-type", "authorization", "location", "etag", "cache-control",
	}

	lowerHeader := strings.ToLower(headerName)
	for _, critical := range criticalHeaders {
		if lowerHeader == critical {
			return SeverityCritical
		}
	}

	return SeverityMedium
}

func (d *DefaultDiffEngine) isHeaderRemovalBreaking(headerName string) bool {
	breakingHeaders := []string{
		"content-type", "location", "etag",
	}

	lowerHeader := strings.ToLower(headerName)
	for _, breaking := range breakingHeaders {
		if lowerHeader == breaking {
			return true
		}
	}

	return false
}

func (d *DefaultDiffEngine) assessHeaderValueSeverity(headerName, _, _ string) Severity {
	if strings.ToLower(headerName) == "content-type" {
		return SeverityHigh
	}

	return SeverityLow
}

func (d *DefaultDiffEngine) assessPerformanceSeverity(delta, baseline time.Duration) Severity {
	percentChange := float64(delta) / float64(baseline) * 100

	if percentChange >= 100 || percentChange <= -50 {
		return SeverityCritical
	} else if percentChange >= 50 || percentChange <= -25 {
		return SeverityHigh
	} else if percentChange >= 25 || percentChange <= -10 {
		return SeverityMedium
	}

	return SeverityLow
}

func (d *DefaultDiffEngine) mapDiffTypeToChangeType(diffType DiffType) ChangeType {
	switch diffType {
	case DiffTypeAdded:
		return ChangeTypeFieldAdded
	case DiffTypeRemoved:
		return ChangeTypeFieldRemoved
	case DiffTypeModified:
		return ChangeTypeFieldModified
	case DiffTypeTypeChanged:
		return ChangeTypeTypeChange
	default:
		return ChangeTypeFieldModified
	}
}

func (d *DefaultDiffEngine) mapSeverityToImpact(severity Severity) ImpactLevel {
	switch severity {
	case SeverityCritical:
		return ImpactLevelCritical
	case SeverityHigh:
		return ImpactLevelMajor
	case SeverityMedium:
		return ImpactLevelModerate
	case SeverityLow:
		return ImpactLevelMinor
	default:
		return ImpactLevelNone
	}
}

func (d *DefaultDiffEngine) generateChangeDescription(diff FieldDiff) string {
	switch diff.Type {
	case DiffTypeAdded:
		return fmt.Sprintf("Field '%s' was added with value: %v", diff.Path, diff.NewValue)
	case DiffTypeRemoved:
		return fmt.Sprintf("Field '%s' was removed (previous value: %v)", diff.Path, diff.OldValue)
	case DiffTypeModified:
		return fmt.Sprintf("Field '%s' changed from %v to %v", diff.Path, diff.OldValue, diff.NewValue)
	case DiffTypeTypeChanged:
		return fmt.Sprintf("Field '%s' type changed from %T to %T", diff.Path, diff.OldValue, diff.NewValue)
	default:
		return fmt.Sprintf("Field '%s' was modified", diff.Path)
	}
}

func (d *DefaultDiffEngine) generateMitigation(diff FieldDiff) string {
	switch diff.Type {
	case DiffTypeRemoved:
		return fmt.Sprintf("Update client code to handle missing field '%s'", diff.Path)
	case DiffTypeTypeChanged:
		return fmt.Sprintf("Update client code to handle type change for field '%s'", diff.Path)
	case DiffTypeModified:
		if d.isCriticalField(diff.Path) {
			return fmt.Sprintf("Review and update logic that depends on field '%s'", diff.Path)
		}
		return "Review if the value change affects client logic"
	default:
		return "Review the change and update client code if necessary"
	}
}

func (d *DefaultDiffEngine) generatePerformanceDescription(delta, oldTime, newTime time.Duration) string {
	percentChange := float64(delta) / float64(oldTime) * 100

	if delta > 0 {
		return fmt.Sprintf("Response time increased by %v (%.1f%%) from %v to %v",
			delta, percentChange, oldTime, newTime)
	} else {
		return fmt.Sprintf("Response time decreased by %v (%.1f%%) from %v to %v",
			-delta, -percentChange, oldTime, newTime)
	}
}

func (d *DefaultDiffEngine) generateClassificationReasoning(diff *FieldDiff) string {
	reasons := []string{}

	if diff.Type == DiffTypeRemoved {
		reasons = append(reasons, "field removal is potentially breaking")
	}

	if diff.Type == DiffTypeTypeChanged {
		reasons = append(reasons, "type changes are breaking")
	}

	if d.isCriticalField(diff.Path) {
		reasons = append(reasons, "field is identified as critical")
	}

	if len(reasons) == 0 {
		return "standard change classification applied"
	}

	return strings.Join(reasons, "; ")
}

func (d *DefaultDiffEngine) generateSummary(result *DiffResult) {
	summary := result.Summary

	// Count changes by severity
	for _, change := range result.StructuralChanges {
		summary.TotalChanges++
		if change.Breaking {
			summary.BreakingChanges++
		}
		d.incrementSeverityCount(summary, change.Severity)
	}

	for _, change := range result.DataChanges {
		summary.TotalChanges++
		d.incrementSeverityCount(summary, change.Severity)
	}

	if result.PerformanceChanges != nil {
		summary.TotalChanges++
		d.incrementSeverityCount(summary, result.PerformanceChanges.Severity)
	}
}

func (d *DefaultDiffEngine) incrementSeverityCount(summary *DiffSummary, severity Severity) {
	switch severity {
	case SeverityCritical:
		summary.CriticalChanges++
	case SeverityHigh:
		summary.HighChanges++
	case SeverityMedium:
		summary.MediumChanges++
	case SeverityLow:
		summary.LowChanges++
	}
}

// AnalyzeTrends analyzes trends across multiple responses (placeholder implementation)
func (d *DefaultDiffEngine) AnalyzeTrends(responses []*Response) (*TrendAnalysis, error) {
	if len(responses) < 2 {
		return nil, fmt.Errorf("need at least 2 responses for trend analysis")
	}

	// This is a simplified implementation - in practice, you'd want more sophisticated analysis
	analysis := &TrendAnalysis{
		TotalResponses: len(responses),
		CommonChanges:  []CommonChange{},
		StabilityScore: 1.0, // Start with perfect stability
	}

	// Calculate period
	if len(responses) > 1 {
		analysis.Period = responses[len(responses)-1].Timestamp.Sub(responses[0].Timestamp)
	}

	// Simple change frequency calculation
	changeCount := 0
	for i := 1; i < len(responses); i++ {
		result, err := d.CompareResponses(responses[i-1], responses[i])
		if err != nil {
			continue
		}
		if result.HasChanges {
			changeCount++
		}
	}

	analysis.ChangeFrequency = float64(changeCount) / float64(len(responses)-1)
	analysis.StabilityScore = 1.0 - analysis.ChangeFrequency

	// Performance trend analysis
	if len(responses) > 1 {
		analysis.PerformanceTrend = d.analyzePerformanceTrend(responses)
	}

	return analysis, nil
}

func (d *DefaultDiffEngine) analyzePerformanceTrend(responses []*Response) *PerformanceTrend {
	if len(responses) < 2 {
		return nil
	}

	var totalTime time.Duration
	validResponses := 0

	for _, resp := range responses {
		if resp.ResponseTime > 0 {
			totalTime += resp.ResponseTime
			validResponses++
		}
	}

	if validResponses == 0 {
		return nil
	}

	avgTime := totalTime / time.Duration(validResponses)

	// Simple trend detection: compare first half with second half
	midpoint := len(responses) / 2
	var firstHalfAvg, secondHalfAvg time.Duration
	firstHalfCount, secondHalfCount := 0, 0

	for i := 0; i < midpoint; i++ {
		if responses[i].ResponseTime > 0 {
			firstHalfAvg += responses[i].ResponseTime
			firstHalfCount++
		}
	}

	for i := midpoint; i < len(responses); i++ {
		if responses[i].ResponseTime > 0 {
			secondHalfAvg += responses[i].ResponseTime
			secondHalfCount++
		}
	}

	if firstHalfCount > 0 {
		firstHalfAvg /= time.Duration(firstHalfCount)
	}
	if secondHalfCount > 0 {
		secondHalfAvg /= time.Duration(secondHalfCount)
	}

	trend := TrendDirectionStable
	if secondHalfAvg > firstHalfAvg*11/10 { // 10% worse
		trend = TrendDirectionDegrading
	} else if secondHalfAvg < firstHalfAvg*9/10 { // 10% better
		trend = TrendDirectionImproving
	}

	return &PerformanceTrend{
		AverageResponseTime: avgTime,
		Trend:               trend,
		PercentileChanges:   make(map[string]time.Duration), // Could be enhanced with actual percentile calculations
	}
}
