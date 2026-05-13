// Package validation provides request decoding and struct validation for HTTP handlers.
package validation

import "net/http"

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError is returned by DecodeAndValidate when the request body is
// structurally valid JSON but one or more fields fail validation rules.
// StatusCode is always http.StatusUnprocessableEntity (422).
type ValidationError struct {
	Fields     []FieldError
	StatusCode int
}

// newValidationError constructs a ValidationError from a slice of field errors.
func newValidationError(fields []FieldError) *ValidationError {
	return &ValidationError{
		Fields:     fields,
		StatusCode: http.StatusUnprocessableEntity,
	}
}
