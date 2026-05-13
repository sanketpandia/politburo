package httpdto

import (
	"encoding/json"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/platform/validation"
)

type Response[T any] struct {
	Status       string `json:"status"`        // "ok" or "error"
	Result       T      `json:"result,omitempty"`
	Error        *Error `json:"error,omitempty"`
	ResponseTime int64  `json:"responseTimeMs"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteSuccess(w http.ResponseWriter, start time.Time, result interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response[interface{}]{
		Status:       "ok",
		Result:       result,
		ResponseTime: time.Since(start).Milliseconds(),
	})
}

func WriteError(w http.ResponseWriter, start time.Time, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response[interface{}]{
		Status: "error",
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ResponseTime: time.Since(start).Milliseconds(),
	})
}

// validationResponse is the JSON shape for 422 validation error responses.
// Fields is inlined into the error object to match the OpenAPI spec shape.
type validationResponse struct {
	Status string                `json:"status"`
	Error  validationErrorDetail `json:"error"`
	ResponseTime int64           `json:"responseTimeMs"`
}

type validationErrorDetail struct {
	Code    string                    `json:"code"`
	Message string                    `json:"message"`
	Fields  []validation.FieldError   `json:"fields,omitempty"`
}

// WriteValidationError writes a 422 Unprocessable Entity response with
// field-level validation details, matching the ValidationErrorResponse schema
// defined in api/openapi/registration.yaml.
func WriteValidationError(w http.ResponseWriter, start time.Time, ve *validation.ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(validationResponse{
		Status: "error",
		Error: validationErrorDetail{
			Code:    "VALIDATION_FAILED",
			Message: "One or more fields failed validation",
			Fields:  ve.Fields,
		},
		ResponseTime: time.Since(start).Milliseconds(),
	})
}
