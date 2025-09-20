package validator

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_SimpleAPI_ValidResponse(t *testing.T) {
	validator := NewValidator()

	// Load the simple API spec
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /users operation
	operation := swagger.Paths.Paths["/users"].Get
	require.NotNil(t, operation)

	// Test valid response
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body: []byte(`[
			{
				"id": 1,
				"name": "John Doe",
				"email": "john@example.com",
				"age": 30,
				"created_at": "2023-01-01T00:00:00Z"
			},
			{
				"id": 2,
				"name": "Jane Smith",
				"email": "jane@example.com",
				"created_at": "2023-01-02T00:00:00Z"
			}
		]`),
	}

	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestIntegration_SimpleAPI_InvalidResponse(t *testing.T) {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeStrict)

	// Load the simple API spec
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /users operation
	operation := swagger.Paths.Paths["/users"].Get
	require.NotNil(t, operation)

	// Test invalid response - missing required fields
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body: []byte(`[
			{
				"id": 1,
				"name": "John Doe"
				// Missing required "email" field
			}
		]`),
	}

	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestIntegration_SimpleAPI_AdditionalFields(t *testing.T) {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeLenient)

	// Load the simple API spec
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /users operation
	operation := swagger.Paths.Paths["/users"].Get
	require.NotNil(t, operation)

	// Test response with additional fields
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body: []byte(`[
			{
				"id": 1,
				"name": "John Doe",
				"email": "john@example.com",
				"phone": "+1234567890",
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}
		]`),
	}

	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.FieldDiffs)

	// Should detect additional fields
	additionalFields := 0
	for _, diff := range result.FieldDiffs {
		if diff.Type == DiffTypeAdded {
			additionalFields++
		}
	}
	assert.Greater(t, additionalFields, 0)
}

func TestIntegration_ComplexAPI_NestedValidation(t *testing.T) {
	validator := NewValidator()

	// Load the complex API spec
	specPath := filepath.Join("testdata", "complex-api.yaml")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /products operation
	operation := swagger.Paths.Paths["/products"].Get
	require.NotNil(t, operation)

	// Test valid complex response
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body: []byte(`{
			"products": [
				{
					"id": "prod-123",
					"name": "Laptop",
					"description": "High-performance laptop",
					"price": 999.99,
					"category": {
						"id": "cat-1",
						"name": "Electronics"
					},
					"tags": ["laptop", "computer", "portable"],
					"attributes": {
						"brand": "TechCorp",
						"model": "TC-2023"
					},
					"inventory": {
						"quantity": 50,
						"available": 45,
						"reserved": 5,
						"warehouse_locations": [
							{
								"warehouse_id": "wh-001",
								"quantity": 30,
								"aisle": "A",
								"shelf": "1"
							},
							{
								"warehouse_id": "wh-002",
								"quantity": 20,
								"aisle": "B",
								"shelf": "3"
							}
						]
					},
					"created_at": "2023-01-01T00:00:00Z",
					"updated_at": "2023-01-15T12:30:00Z"
				}
			],
			"pagination": {
				"total": 100,
				"limit": 10,
				"offset": 0,
				"has_more": true
			}
		}`),
	}

	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestIntegration_ErrorResponse(t *testing.T) {
	validator := NewValidator()

	// Load the simple API spec
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /users operation
	operation := swagger.Paths.Paths["/users"].Get
	require.NotNil(t, operation)

	// Test error response
	response := &Response{
		StatusCode: 500,
		Headers:    http.Header{},
		Body: []byte(`{
			"code": 500,
			"message": "Internal server error",
			"details": "Database connection failed"
		}`),
	}

	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestIntegration_ResponseComparison_RealWorldScenario(t *testing.T) {
	validator := NewValidator()

	// Simulate API evolution scenario
	previousResponse := &Response{
		StatusCode: 200,
		Body: []byte(`{
			"id": 1,
			"name": "John Doe",
			"email": "john@example.com",
			"age": 30,
			"status": "active",
			"preferences": {
				"theme": "dark",
				"notifications": true
			},
			"roles": ["user"]
		}`),
	}

	// API evolved - added new fields, changed some values, removed a field
	currentResponse := &Response{
		StatusCode: 200,
		Body: []byte(`{
			"id": 1,
			"name": "John Doe",
			"email": "john.doe@example.com",
			"age": 31,
			"status": "premium",
			"preferences": {
				"theme": "light",
				"notifications": true,
				"language": "en"
			},
			"roles": ["user", "premium"],
			"subscription": {
				"plan": "premium",
				"expires_at": "2024-12-31T23:59:59Z"
			}
		}`),
	}

	diffs, err := validator.CompareResponses(previousResponse, currentResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, diffs)

	// Analyze the differences
	diffsByType := make(map[DiffType][]FieldDiff)
	for _, diff := range diffs {
		diffsByType[diff.Type] = append(diffsByType[diff.Type], diff)
	}

	// Should detect modifications
	assert.NotEmpty(t, diffsByType[DiffTypeModified])

	// Should detect additions
	assert.NotEmpty(t, diffsByType[DiffTypeAdded])

	// Verify specific changes
	foundEmailChange := false
	foundSubscriptionAdd := false
	foundLanguageAdd := false

	for _, diff := range diffs {
		switch {
		case diff.Path == "$.email" && diff.Type == DiffTypeModified:
			foundEmailChange = true
			assert.Equal(t, "john@example.com", diff.OldValue)
			assert.Equal(t, "john.doe@example.com", diff.NewValue)
		case diff.Path == "$.subscription" && diff.Type == DiffTypeAdded:
			foundSubscriptionAdd = true
			assert.Equal(t, SeverityLow, diff.Severity)
		case diff.Path == "$.preferences.language" && diff.Type == DiffTypeAdded:
			foundLanguageAdd = true
		}
	}

	assert.True(t, foundEmailChange, "Should detect email change")
	assert.True(t, foundSubscriptionAdd, "Should detect subscription addition")
	assert.True(t, foundLanguageAdd, "Should detect language preference addition")
}

func TestIntegration_ValidationModes_Comparison(t *testing.T) {
	validator := NewValidator()

	// Load the simple API spec
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /users operation
	operation := swagger.Paths.Paths["/users"].Get
	require.NotNil(t, operation)

	// Response with undefined status code
	response := &Response{
		StatusCode: 418, // I'm a teapot - not defined in spec
		Headers:    http.Header{},
		Body:       []byte(`{"message": "I'm a teapot"}`),
	}

	// Test in strict mode
	validator.SetValidationMode(ValidationModeStrict)
	strictResult, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.False(t, strictResult.Valid)
	assert.NotEmpty(t, strictResult.Errors)
	assert.Empty(t, strictResult.Warnings)

	// Test in lenient mode
	validator.SetValidationMode(ValidationModeLenient)
	lenientResult, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, lenientResult.Valid)
	assert.Empty(t, lenientResult.Errors)
	assert.NotEmpty(t, lenientResult.Warnings)

	// Both should have the same warning/error message content
	assert.Contains(t, strictResult.Errors[0].Message, "status code 418 not defined")
	assert.Contains(t, lenientResult.Warnings[0].Message, "status code 418 not defined")
}

func TestIntegration_LargeResponse_Performance(t *testing.T) {
	validator := NewValidator()

	// Load the complex API spec
	specPath := filepath.Join("testdata", "complex-api.yaml")
	swagger, err := validator.LoadSpec(specPath)
	require.NoError(t, err)

	// Get the GET /products operation
	operation := swagger.Paths.Paths["/products"].Get
	require.NotNil(t, operation)

	// Generate a large response with many products
	largeResponse := `{
		"products": [`

	for i := 0; i < 100; i++ {
		if i > 0 {
			largeResponse += ","
		}
		largeResponse += fmt.Sprintf(`{
			"id": "prod-%d",
			"name": "Product %d",
			"price": 99.99,
			"category": {
				"id": "cat-1",
				"name": "Electronics"
			},
			"inventory": {
				"quantity": 10,
				"available": 8
			}
		}`, i, i)
	}

	largeResponse += `],
		"pagination": {
			"total": 100,
			"limit": 100,
			"offset": 0,
			"has_more": false
		}
	}`

	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(largeResponse),
	}

	// This should complete without timeout or excessive memory usage
	result, err := validator.ValidateResponse(response, operation)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}
