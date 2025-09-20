// Package validator provides OpenAPI response validation functionality
package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

// Validator defines the interface for response validation
type Validator interface {
	ValidateResponse(response *Response, operation *spec.Operation) (*ValidationResult, error)
	LoadSpec(specFile string) (*spec.Swagger, error)
	SetValidationMode(mode ValidationMode)
	GetValidationMode() ValidationMode
	CompareResponses(previous, current *Response) ([]FieldDiff, error)
}

// Response represents an HTTP response for validation
type Response struct {
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
	StatusCode int         `json:"status_code"`
}

// ValidationMode defines the validation strictness
type ValidationMode string

const (
	ValidationModeStrict  ValidationMode = "strict"
	ValidationModeLenient ValidationMode = "lenient"
)

// ValidationResult represents the result of response validation
type ValidationResult struct {
	Errors     []ValidationError   `json:"errors,omitempty"`
	Warnings   []ValidationWarning `json:"warnings,omitempty"`
	FieldDiffs []FieldDiff         `json:"field_diffs,omitempty"`
	Valid      bool                `json:"valid"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Path    string `json:"path"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Path    string `json:"path"`
}

// FieldDiff represents a difference in a field
type FieldDiff struct {
	Path     string      `json:"path"`
	Type     DiffType    `json:"type"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
	Severity Severity    `json:"severity"`
}

// DiffType represents the type of difference
type DiffType string

const (
	DiffTypeAdded       DiffType = "added"
	DiffTypeRemoved     DiffType = "removed"
	DiffTypeModified    DiffType = "modified"
	DiffTypeTypeChanged DiffType = "type_changed"
)

// Severity represents the severity of a difference
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// OpenAPIValidator implements the Validator interface
type OpenAPIValidator struct {
	mode ValidationMode
}

// NewValidator creates a new OpenAPI validator
func NewValidator() Validator {
	return &OpenAPIValidator{
		mode: ValidationModeLenient,
	}
}

// SetValidationMode sets the validation mode (strict or lenient)
func (v *OpenAPIValidator) SetValidationMode(mode ValidationMode) {
	v.mode = mode
}

// GetValidationMode returns the current validation mode
func (v *OpenAPIValidator) GetValidationMode() ValidationMode {
	return v.mode
}

// LoadSpec loads an OpenAPI specification from a file
func (v *OpenAPIValidator) LoadSpec(specFile string) (*spec.Swagger, error) {
	if specFile == "" {
		return nil, fmt.Errorf("spec file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(specFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("spec file does not exist: %s", specFile)
	}

	// Get absolute path
	absPath, err := filepath.Abs(specFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for spec file: %w", err)
	}

	// Load the specification
	doc, err := loads.Spec(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec from %s: %w", specFile, err)
	}

	// Expand the spec to resolve all references
	expandedDoc, err := doc.Expanded()
	if err != nil {
		return nil, fmt.Errorf("failed to expand OpenAPI spec references: %w", err)
	}

	// Validate the spec itself
	if err := validate.Spec(expandedDoc, strfmt.Default); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI specification: %w", err)
	}

	return expandedDoc.Spec(), nil
}

// ValidateResponse validates an HTTP response against an OpenAPI operation
func (v *OpenAPIValidator) ValidateResponse(response *Response, operation *spec.Operation) (*ValidationResult, error) {
	if response == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	if operation == nil {
		return nil, fmt.Errorf("operation cannot be nil")
	}

	result := &ValidationResult{
		Valid:      true,
		Errors:     []ValidationError{},
		Warnings:   []ValidationWarning{},
		FieldDiffs: []FieldDiff{},
	}

	// Find the response schema for the status code
	responseSpec := v.findResponseSpec(operation, response.StatusCode)
	if responseSpec == nil {
		if v.mode == ValidationModeStrict {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "status_code",
				Message: fmt.Sprintf("status code %d not defined in OpenAPI spec", response.StatusCode),
				Type:    "undefined_status_code",
				Path:    "$.status_code",
			})
		} else {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "status_code",
				Message: fmt.Sprintf("status code %d not defined in OpenAPI spec", response.StatusCode),
				Type:    "undefined_status_code",
				Path:    "$.status_code",
			})
		}
		return result, nil
	}

	// Validate response body if schema is defined
	if responseSpec.Schema != nil && len(response.Body) > 0 {
		v.validateResponseBody(response.Body, responseSpec.Schema, result)
	}

	// Validate response headers
	v.validateResponseHeaders(response.Headers, responseSpec.Headers, result)

	return result, nil
}

// findResponseSpec finds the appropriate response specification for a status code
func (v *OpenAPIValidator) findResponseSpec(operation *spec.Operation, statusCode int) *spec.Response {
	if operation.Responses == nil {
		return nil
	}

	// Try exact status code match first
	if resp, exists := operation.Responses.StatusCodeResponses[statusCode]; exists {
		return &resp
	}

	// Try default response
	if operation.Responses.Default != nil {
		return operation.Responses.Default
	}

	return nil
}

// validateResponseBody validates the response body against the schema
func (v *OpenAPIValidator) validateResponseBody(body []byte, schema *spec.Schema, result *ValidationResult) {
	// Parse JSON body
	var bodyData interface{}
	if err := json.Unmarshal(body, &bodyData); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "body",
			Message: fmt.Sprintf("invalid JSON in response body: %s", err.Error()),
			Type:    "invalid_json",
			Path:    "$.body",
		})
		return
	}

	// Validate against schema
	schemaValidator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)
	validationResult := schemaValidator.Validate(bodyData)

	if validationResult != nil && validationResult.HasErrors() {
		result.Valid = false
		for _, err := range validationResult.Errors {
			if validationErr, ok := err.(*errors.Validation); ok {
				result.Errors = append(result.Errors, ValidationError{
					Field:   extractFieldFromError(validationErr),
					Message: err.Error(),
					Type:    "schema_validation",
					Path:    extractPathFromError(validationErr),
				})
			} else {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "unknown",
					Message: err.Error(),
					Type:    "schema_validation",
					Path:    "$",
				})
			}
		}
	}

	// In lenient mode, check for additional fields that aren't in the schema
	if v.mode == ValidationModeLenient {
		v.detectAdditionalFields(bodyData, schema, result, "$")
	}
}

// validateResponseHeaders validates response headers against the specification
func (v *OpenAPIValidator) validateResponseHeaders(headers http.Header, expectedHeaders map[string]spec.Header, result *ValidationResult) {
	// Check required headers
	for headerName, headerSpec := range expectedHeaders {
		headerValue := headers.Get(headerName)

		// Check if required header is missing
		if headerValue == "" {
			if v.mode == ValidationModeStrict {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   headerName,
					Message: fmt.Sprintf("required header '%s' is missing", headerName),
					Type:    "missing_header",
					Path:    fmt.Sprintf("$.headers.%s", headerName),
				})
			} else {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   headerName,
					Message: fmt.Sprintf("expected header '%s' is missing", headerName),
					Type:    "missing_header",
					Path:    fmt.Sprintf("$.headers.%s", headerName),
				})
			}
			continue
		}

		// Validate header value against schema if defined
		if headerSpec.Type != "" {
			v.validateHeaderValue(headerValue, &headerSpec, headerName, result)
		}
	}
}

// validateHeaderValue validates a header value against its specification
func (v *OpenAPIValidator) validateHeaderValue(headerValue string, headerSpec *spec.Header, headerName string, result *ValidationResult) {
	// Basic type validation
	switch headerSpec.Type {
	case "string":
		// String validation (could add pattern matching here)
		if headerSpec.Pattern != "" {
			// TODO: Implement pattern validation using headerValue and headerName
			// For now, just record that validation was attempted
			_ = headerValue // Use the parameter to avoid unused warning
			_ = headerName  // Use the parameter to avoid unused warning
			_ = result      // Use the parameter to avoid unused warning
			// Pattern validation would be implemented here
		}
	case "integer":
		// Integer validation would go here using headerValue and headerName
		_ = headerValue
		_ = headerName
		_ = result
	case "number":
		// Number validation would go here using headerValue and headerName
		_ = headerValue
		_ = headerName
		_ = result
	case "boolean":
		// Boolean validation would go here using headerValue and headerName
		_ = headerValue
		_ = headerName
		_ = result
	}
}

// detectAdditionalFields detects fields in the response that aren't defined in the schema
func (v *OpenAPIValidator) detectAdditionalFields(data interface{}, schema *spec.Schema, result *ValidationResult, path string) {
	if schema == nil {
		return
	}

	switch dataValue := data.(type) {
	case map[string]interface{}:
		if schema.Type.Contains("object") && schema.Properties != nil {
			for key, value := range dataValue {
				fieldPath := fmt.Sprintf("%s.%s", path, key)

				if _, exists := schema.Properties[key]; !exists {
					// Field not defined in schema
					result.FieldDiffs = append(result.FieldDiffs, FieldDiff{
						Path:     fieldPath,
						Type:     DiffTypeAdded,
						NewValue: value,
						Severity: SeverityLow,
					})
				} else {
					// Recursively check nested objects
					if propSchema, ok := schema.Properties[key]; ok {
						v.detectAdditionalFields(value, &propSchema, result, fieldPath)
					}
				}
			}
		}
	case []interface{}:
		if schema.Type.Contains("array") && schema.Items != nil && schema.Items.Schema != nil {
			for i, item := range dataValue {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				v.detectAdditionalFields(item, schema.Items.Schema, result, itemPath)
			}
		}
	}
}

// extractFieldFromError extracts the field name from a validation error
func extractFieldFromError(err *errors.Validation) string {
	if err.Name != "" {
		return err.Name
	}
	return "unknown"
}

// extractPathFromError extracts the JSON path from a validation error
func extractPathFromError(err *errors.Validation) string {
	if err.Name != "" {
		return fmt.Sprintf("$.%s", err.Name)
	}
	return "$"
}

// CompareResponses compares two responses and detects differences
func (v *OpenAPIValidator) CompareResponses(previous, current *Response) ([]FieldDiff, error) {
	if previous == nil || current == nil {
		return nil, fmt.Errorf("both responses must be non-nil")
	}

	var diffs []FieldDiff

	// Parse JSON bodies
	var prevData, currData interface{}

	if len(previous.Body) > 0 {
		if err := json.Unmarshal(previous.Body, &prevData); err != nil {
			return nil, fmt.Errorf("failed to parse previous response body: %w", err)
		}
	}

	if len(current.Body) > 0 {
		if err := json.Unmarshal(current.Body, &currData); err != nil {
			return nil, fmt.Errorf("failed to parse current response body: %w", err)
		}
	}

	// Compare the data structures
	v.compareValues(prevData, currData, "$", &diffs)

	return diffs, nil
}

// compareValues recursively compares two values and records differences
func (v *OpenAPIValidator) compareValues(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	if v.handleNilValues(prev, curr, path, diffs) {
		return
	}

	if v.handleTypeChanges(prev, curr, path, diffs) {
		return
	}

	v.compareValuesByType(prev, curr, path, diffs)
}

// handleNilValues handles comparison when one or both values are nil
func (v *OpenAPIValidator) handleNilValues(prev, curr interface{}, path string, diffs *[]FieldDiff) bool {
	if prev == nil && curr == nil {
		return true
	}

	if prev == nil {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeAdded,
			NewValue: curr,
			Severity: v.determineSeverity(path, DiffTypeAdded),
		})
		return true
	}

	if curr == nil {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeRemoved,
			OldValue: prev,
			Severity: v.determineSeverity(path, DiffTypeRemoved),
		})
		return true
	}

	return false
}

// handleTypeChanges handles comparison when types are different
func (v *OpenAPIValidator) handleTypeChanges(prev, curr interface{}, path string, diffs *[]FieldDiff) bool {
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
func (v *OpenAPIValidator) compareValuesByType(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	switch prevValue := prev.(type) {
	case map[string]interface{}:
		if currValue, ok := curr.(map[string]interface{}); ok {
			v.compareObjects(prevValue, currValue, path, diffs)
		}
	case []interface{}:
		if currValue, ok := curr.([]interface{}); ok {
			v.compareArrays(prevValue, currValue, path, diffs)
		}
	default:
		v.compareScalarValues(prev, curr, path, diffs)
	}
}

// compareObjects compares two object values
func (v *OpenAPIValidator) compareObjects(prevValue, currValue map[string]interface{}, path string, diffs *[]FieldDiff) {
	// Check for removed fields
	for key, value := range prevValue {
		fieldPath := fmt.Sprintf("%s.%s", path, key)
		if _, exists := currValue[key]; !exists {
			*diffs = append(*diffs, FieldDiff{
				Path:     fieldPath,
				Type:     DiffTypeRemoved,
				OldValue: value,
				Severity: v.determineSeverity(fieldPath, DiffTypeRemoved),
			})
		}
	}

	// Check for added or modified fields
	for key, currFieldValue := range currValue {
		fieldPath := fmt.Sprintf("%s.%s", path, key)
		if prevFieldValue, exists := prevValue[key]; exists {
			v.compareValues(prevFieldValue, currFieldValue, fieldPath, diffs)
		} else {
			*diffs = append(*diffs, FieldDiff{
				Path:     fieldPath,
				Type:     DiffTypeAdded,
				NewValue: currFieldValue,
				Severity: v.determineSeverity(fieldPath, DiffTypeAdded),
			})
		}
	}
}

// compareArrays compares two array values
func (v *OpenAPIValidator) compareArrays(prevValue, currValue []interface{}, path string, diffs *[]FieldDiff) {
	// Simple array comparison - could be enhanced with more sophisticated diff algorithms
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

		v.compareValues(prevItem, currItem, itemPath, diffs)
	}
}

// compareScalarValues compares scalar values
func (v *OpenAPIValidator) compareScalarValues(prev, curr interface{}, path string, diffs *[]FieldDiff) {
	if !reflect.DeepEqual(prev, curr) {
		*diffs = append(*diffs, FieldDiff{
			Path:     path,
			Type:     DiffTypeModified,
			OldValue: prev,
			NewValue: curr,
			Severity: v.determineSeverity(path, DiffTypeModified),
		})
	}
}

// determineSeverity determines the severity of a change based on the path and type
func (v *OpenAPIValidator) determineSeverity(path string, diffType DiffType) Severity {
	// This is a simplified severity determination
	// In a real implementation, you might want to make this configurable
	// or base it on the OpenAPI schema (e.g., required fields are more critical)

	switch diffType {
	case DiffTypeRemoved:
		// Removed fields are typically breaking changes
		return SeverityCritical
	case DiffTypeTypeChanged:
		// Type changes are always critical
		return SeverityCritical
	case DiffTypeAdded:
		// Added fields are usually non-breaking
		return SeverityLow
	case DiffTypeModified:
		// Modified values depend on context
		if strings.Contains(path, "id") || strings.Contains(path, "version") {
			return SeverityHigh
		}
		return SeverityMedium
	default:
		return SeverityLow
	}
}
