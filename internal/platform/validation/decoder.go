package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// DecodeAndValidate decodes a JSON request body into dst and validates the
// result using struct tags.
//
// Return values:
//   - (nil, nil)               — success; dst is populated and valid.
//   - (nil, *ValidationError)  — JSON decoded successfully but validation failed;
//     the caller should write a 422 response.
//   - (err, nil)               — JSON decode failed (malformed body or wrong
//     Content-Type); the caller should write a 400 response.
func DecodeAndValidate(r *http.Request, dst any) (error, *ValidationError) {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err), nil
	}

	if ve := validateStruct(dst); ve != nil {
		return nil, ve
	}

	return nil, nil
}

// validateStruct runs go-playground validation against dst and converts any
// ValidationErrors into our FieldError slice.
func validateStruct(dst any) *ValidationError {
	err := Validator().Struct(dst)
	if err == nil {
		return nil
	}

	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		// Not a field-level validation error; propagate as internal error.
		return newValidationError([]FieldError{{Field: "_", Message: err.Error()}})
	}

	fields := make([]FieldError, 0, len(validationErrs))
	for _, fe := range validationErrs {
		fields = append(fields, FieldError{
			Field:   fe.Field(),
			Message: fieldErrorMessage(fe),
		})
	}
	return newValidationError(fields)
}

// fieldErrorMessage converts a single validator.FieldError to a human-readable
// message. Extend the switch as new tags are used.
func fieldErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", fe.Field(), fe.Param())
	case "email":
		return fe.Field() + " must be a valid email address"
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("%s failed %s validation", fe.Field(), fe.Tag())
	}
}
