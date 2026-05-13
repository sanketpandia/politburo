package pilots

import (
	"errors"
	"net/http"
)

// Sentinel errors for the registration flow.
var (
	ErrIFCUserNotFound        = errors.New("IFC user not found in Infinite Flight system")
	ErrNoRecentFlights        = errors.New("no recent flights found")
	ErrFlightMismatch         = errors.New("last flight does not match logbook")
	ErrRegistrationFailed     = errors.New("failed to register user")
	ErrIFCIdAlreadyRegistered = errors.New("IFC ID is already registered to another user")
)

// RegistrationError is a structured error returned by RegistrationService.RegisterPilot.
// The handler uses Code, Message, and StatusCode directly — no switch dispatch required.
type RegistrationError struct {
	Code       string // machine-readable, e.g. "IFC_USER_NOT_FOUND"
	Message    string // human-readable
	StatusCode int    // HTTP status code to use in the response
	Cause      error  // underlying error for logging; nil for expected domain failures
}

func (e *RegistrationError) Error() string { return e.Message }

// registrationErr is a helper that constructs a RegistrationError from a sentinel.
func registrationErr(code, message string, statusCode int, cause error) *RegistrationError {
	return &RegistrationError{Code: code, Message: message, StatusCode: statusCode, Cause: cause}
}

// sentinelToRegistrationError maps each sentinel error to a RegistrationError.
// Called by RegisterPilot before returning to the handler.
func sentinelToRegistrationError(sentinel error) *RegistrationError {
	switch {
	case errors.Is(sentinel, ErrIFCIdAlreadyRegistered):
		return registrationErr(
			"IFC_ID_ALREADY_REGISTERED",
			"This IFC ID is already registered to another Discord account. Each IFC ID can only be linked to one Discord account.",
			http.StatusConflict,
			nil,
		)
	case errors.Is(sentinel, ErrIFCUserNotFound):
		return registrationErr(
			"IFC_USER_NOT_FOUND",
			"IFC user not found. Please verify your IFC username.",
			http.StatusNotFound,
			sentinel,
		)
	case errors.Is(sentinel, ErrNoRecentFlights):
		return registrationErr(
			"NO_RECENT_FLIGHTS",
			"No recent flights found in your logbook.",
			http.StatusBadRequest,
			sentinel,
		)
	case errors.Is(sentinel, ErrFlightMismatch):
		return registrationErr(
			"FLIGHT_MISMATCH",
			"Last flight verification failed. Please verify your last flight route.",
			http.StatusBadRequest,
			sentinel,
		)
	case errors.Is(sentinel, ErrRegistrationFailed):
		return registrationErr(
			"REGISTRATION_FAILED",
			"Failed to register user. Please try again.",
			http.StatusInternalServerError,
			sentinel,
		)
	default:
		return registrationErr(
			"INTERNAL_ERROR",
			"An unexpected error occurred during registration.",
			http.StatusInternalServerError,
			sentinel,
		)
	}
}
