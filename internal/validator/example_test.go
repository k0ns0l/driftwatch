package validator

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-openapi/spec"
)

func ExampleValidator_basic() {
	// Create a new validator
	validator := NewValidator()

	// Load an OpenAPI specification
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, err := validator.LoadSpec(specPath)
	if err != nil {
		fmt.Printf("Error loading spec: %v\n", err)
		return
	}

	// Get an operation from the spec
	operation := swagger.Paths.Paths["/users"].Get

	// Create a response to validate
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`[{"id": 1, "name": "John Doe", "email": "john@example.com"}]`),
	}

	// Validate the response
	result, err := validator.ValidateResponse(response, operation)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	fmt.Printf("Valid: %t\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Output:
	// Valid: true
	// Errors: 0
	// Warnings: 0
}

func ExampleValidator_strictMode() {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeStrict)

	// Create a simple operation that only defines 200 response
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

	// Response with undefined status code
	response := &Response{
		StatusCode: 404,
		Headers:    http.Header{},
		Body:       []byte(`{"error": "Not found"}`),
	}

	result, _ := validator.ValidateResponse(response, operation)

	fmt.Printf("Valid: %t\n", result.Valid)
	fmt.Printf("Has errors: %t\n", len(result.Errors) > 0)
	if len(result.Errors) > 0 {
		fmt.Printf("Error type: %s\n", result.Errors[0].Type)
	}

	// Output:
	// Valid: false
	// Has errors: true
	// Error type: undefined_status_code
}

func ExampleValidator_compareResponses() {
	validator := NewValidator()

	// Previous API response
	previous := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "John", "age": 30}`),
	}

	// Current API response with changes
	current := &Response{
		StatusCode: 200,
		Body:       []byte(`{"id": 1, "name": "Jane", "email": "jane@example.com"}`),
	}

	diffs, _ := validator.CompareResponses(previous, current)

	fmt.Printf("Found %d differences\n", len(diffs))

	// Count different types of changes
	var modified, removed, added int
	for _, diff := range diffs {
		switch diff.Type {
		case DiffTypeModified:
			modified++
		case DiffTypeRemoved:
			removed++
		case DiffTypeAdded:
			added++
		}
	}

	fmt.Printf("Modified: %d, Removed: %d, Added: %d\n", modified, removed, added)

	// Output:
	// Found 3 differences
	// Modified: 1, Removed: 1, Added: 1
}

func ExampleValidator_lenientMode() {
	validator := NewValidator()
	validator.SetValidationMode(ValidationModeLenient)

	// Load spec and get operation
	specPath := filepath.Join("testdata", "simple-api.json")
	swagger, _ := validator.LoadSpec(specPath)
	operation := swagger.Paths.Paths["/users"].Get

	// Response with additional fields not in spec
	response := &Response{
		StatusCode: 200,
		Headers:    http.Header{},
		Body: []byte(`[{
			"id": 1,
			"name": "John Doe",
			"email": "john@example.com",
			"phone": "+1234567890",
			"address": "123 Main St"
		}]`),
	}

	result, _ := validator.ValidateResponse(response, operation)

	fmt.Printf("Valid: %t\n", result.Valid)
	fmt.Printf("Field diffs: %d\n", len(result.FieldDiffs))

	// Count additional fields detected
	additionalFields := 0
	for _, diff := range result.FieldDiffs {
		if diff.Type == DiffTypeAdded {
			additionalFields++
		}
	}
	fmt.Printf("Additional fields detected: %d\n", additionalFields)

	// Output:
	// Valid: true
	// Field diffs: 2
	// Additional fields detected: 2
}
