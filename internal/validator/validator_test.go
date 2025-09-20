package validator

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	assert.NotNil(t, validator)
	assert.Equal(t, ValidationModeLenient, validator.GetValidationMode())
}

func TestSetValidationMode(t *testing.T) {
	validator := NewValidator()

	validator.SetValidationMode(ValidationModeStrict)
	assert.Equal(t, ValidationModeStrict, validator.GetValidationMode())

	validator.SetValidationMode(ValidationModeLenient)
	assert.Equal(t, ValidationModeLenient, validator.GetValidationMode())
}

func TestLoadSpec_ValidSpec(t *testing.T) {
	// Create a temporary OpenAPI spec file
	specContent := `{
		"swagger": "2.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/users": {
				"get": {
					"responses": {
						"200": {
							"description": "Success",
							"schema": {
								"type": "object",
								"properties": {
									"id": {"type": "integer"},
									"name": {"type": "string"}
								},
								"required": ["id", "name"]
							}
						}
					}
				}
			}
		}
	}`

	tempFile := createTempSpecFile(t, specContent)
	defer os.Remove(tempFile)

	validator := NewValidator()
	spec, err := validator.LoadSpec(tempFile)

	require.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Equal(t, "1.0.0", spec.Info.Version)
}

func TestLoadSpec_InvalidFile(t *testing.T) {
	validator := NewValidator()

	// Test with non-existent file
	_, err := validator.LoadSpec("non-existent-file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Test with empty path
	_, err = validator.LoadSpec("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestLoadSpec_InvalidSpec(t *testing.T) {
	// Create a temporary invalid spec file
	invalidSpecContent := `{
		"invalid": "spec"
	}`

	tempFile := createTempSpecFile(t, invalidSpecContent)
	defer os.Remove(tempFile)

	validator := NewValidator()
	_, err := validator.LoadSpec(tempFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid OpenAPI specification")
}

func TestValidateResponse_ValidResponse(t *testing.T) {
	validator := NewValidator()

	// Create a simple operation spec
	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: spec.StringOrArray{"object"},
										Properties: map[string]spec.Schema{
											"id": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"integer"},
												},
											},
											"name": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"string"},
												},
											},
										},
										Required: []string{"id", "name"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"id": 1, "name": "John Doe"}`),
	}

	result, err := validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateResponse_InvalidResponse(t *testing.T) {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeStrict)

	// Create a simple operation spec
	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: spec.StringOrArray{"object"},
										Properties: map[string]spec.Schema{
											"id": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"integer"},
												},
											},
											"name": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"string"},
												},
											},
										},
										Required: []string{"id", "name"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Response missing required field "name"
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"id": 1}`),
	}

	result, err := validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateResponse_UndefinedStatusCode(t *testing.T) {
	validator := NewValidator()

	// Create operation with only 200 response defined
	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
							},
						},
					},
				},
			},
		},
	}

	// Test with undefined status code in strict mode
	validator.SetValidationMode(ValidationModeStrict)
	response := &Response{
		StatusCode: 404,
		Headers:    http.Header{},
		Body:       []byte(`{"error": "Not found"}`),
	}

	result, err := validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, "status code 404 not defined")

	// Test with undefined status code in lenient mode
	validator.SetValidationMode(ValidationModeLenient)
	result, err = validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0].Message, "status code 404 not defined")
}

func TestValidateResponse_InvalidJSON(t *testing.T) {
	validator := NewValidator()

	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: spec.StringOrArray{"object"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"invalid": json}`), // Invalid JSON
	}

	result, err := validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Type, "invalid_json")
}

func TestValidateResponse_NilInputs(t *testing.T) {
	validator := NewValidator()

	// Test with nil response
	_, err := validator.ValidateResponse(nil, &spec.Operation{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "response cannot be nil")

	// Test with nil operation
	response := &Response{StatusCode: 200}
	_, err = validator.ValidateResponse(response, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation cannot be nil")
}

func TestCompareResponses_BasicDifferences(t *testing.T) {
	validator := NewValidator()

	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "John", "age": 30}`),
	}

	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "Jane", "email": "jane@example.com"}`),
	}

	diffs, err := validator.CompareResponses(previous, current)

	require.NoError(t, err)
	assert.NotEmpty(t, diffs)

	// Should detect modified name, removed age, and added email
	diffTypes := make(map[DiffType]int)
	for _, diff := range diffs {
		diffTypes[diff.Type]++
	}

	assert.Greater(t, diffTypes[DiffTypeModified], 0) // name changed
	assert.Greater(t, diffTypes[DiffTypeRemoved], 0)  // age removed
	assert.Greater(t, diffTypes[DiffTypeAdded], 0)    // email added
}

func TestCompareResponses_TypeChanges(t *testing.T) {
	validator := NewValidator()

	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "active": true}`),
	}

	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": "1", "active": "yes"}`),
	}

	diffs, err := validator.CompareResponses(previous, current)

	require.NoError(t, err)
	assert.NotEmpty(t, diffs)

	// Should detect type changes
	typeChanges := 0
	for _, diff := range diffs {
		if diff.Type == DiffTypeTypeChanged {
			typeChanges++
			assert.Equal(t, SeverityCritical, diff.Severity)
		}
	}

	assert.Greater(t, typeChanges, 0)
}

func TestCompareResponses_ArrayDifferences(t *testing.T) {
	validator := NewValidator()

	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"items": [{"id": 1}, {"id": 2}]}`),
	}

	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"items": [{"id": 1}, {"id": 2}, {"id": 3}]}`),
	}

	diffs, err := validator.CompareResponses(previous, current)

	require.NoError(t, err)
	assert.NotEmpty(t, diffs)

	// Should detect array length change and added item
	foundArrayLengthChange := false
	foundAddedItem := false

	for _, diff := range diffs {
		if diff.Path == "$.items" && diff.Type == DiffTypeModified {
			foundArrayLengthChange = true
		}
		if diff.Path == "$.items[2]" && diff.Type == DiffTypeAdded {
			foundAddedItem = true
		}
	}

	assert.True(t, foundArrayLengthChange)
	assert.True(t, foundAddedItem)
}

func TestCompareResponses_NilInputs(t *testing.T) {
	validator := NewValidator()

	response := &Response{StatusCode: 200, Body: []byte(`{}`)}

	// Test with nil previous
	_, err := validator.CompareResponses(nil, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both responses must be non-nil")

	// Test with nil current
	_, err = validator.CompareResponses(response, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both responses must be non-nil")
}

func TestCompareResponses_InvalidJSON(t *testing.T) {
	validator := NewValidator()

	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"valid": "json"}`),
	}

	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"invalid": json}`), // Invalid JSON
	}

	_, err := validator.CompareResponses(previous, current)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse current response body")
}

func TestDetermineSeverity(t *testing.T) {
	validator := &OpenAPIValidator{}

	// Test different diff types
	assert.Equal(t, SeverityCritical, validator.determineSeverity("$.field", DiffTypeRemoved))
	assert.Equal(t, SeverityCritical, validator.determineSeverity("$.field", DiffTypeTypeChanged))
	assert.Equal(t, SeverityLow, validator.determineSeverity("$.field", DiffTypeAdded))

	// Test path-based severity for modifications
	assert.Equal(t, SeverityHigh, validator.determineSeverity("$.user.id", DiffTypeModified))
	assert.Equal(t, SeverityHigh, validator.determineSeverity("$.version", DiffTypeModified))
	assert.Equal(t, SeverityMedium, validator.determineSeverity("$.name", DiffTypeModified))
}

func TestDetectAdditionalFields_LenientMode(t *testing.T) {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeLenient)

	// Create operation with strict schema
	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: spec.StringOrArray{"object"},
										Properties: map[string]spec.Schema{
											"id": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"integer"},
												},
											},
											"name": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"string"},
												},
											},
										},
										Required: []string{"id", "name"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Response with additional field
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"id": 1, "name": "John", "email": "john@example.com"}`),
	}

	result, err := validator.ValidateResponse(response, operation)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.FieldDiffs)

	// Should detect additional field
	foundAdditionalField := false
	for _, diff := range result.FieldDiffs {
		if diff.Path == "$.email" && diff.Type == DiffTypeAdded {
			foundAdditionalField = true
			assert.Equal(t, SeverityLow, diff.Severity)
		}
	}
	assert.True(t, foundAdditionalField)
}

// Helper function to create temporary spec files for testing
func createTempSpecFile(t *testing.T, content string) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test-spec.json")

	err := os.WriteFile(tempFile, []byte(content), 0o644)
	require.NoError(t, err)

	return tempFile
}

// Benchmark tests
func BenchmarkValidateResponse(b *testing.B) {
	validator := NewValidator()

	operation := &spec.Operation{
		OperationProps: spec.OperationProps{
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						200: {
							ResponseProps: spec.ResponseProps{
								Description: "Success",
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: spec.StringOrArray{"object"},
										Properties: map[string]spec.Schema{
											"id": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"integer"},
												},
											},
											"name": {
												SchemaProps: spec.SchemaProps{
													Type: spec.StringOrArray{"string"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"id": 1, "name": "John Doe"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateResponse(response, operation)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareResponses(b *testing.B) {
	validator := NewValidator()

	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "John", "age": 30, "items": [1, 2, 3]}`),
	}

	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "Jane", "email": "jane@example.com", "items": [1, 2, 3, 4]}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.CompareResponses(previous, current)
		if err != nil {
			b.Fatal(err)
		}
	}
}
