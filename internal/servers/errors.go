package servers

import (
	"errors"
	"net/http"
)

// Sentinel errors for the server initialization flow.
var (
	ErrServerAlreadyRegistered = errors.New("server is already registered as a VA")
	ErrUserNotRegistered       = errors.New("user must be registered before initializing server")
	ErrVACreationFailed        = errors.New("failed to create virtual airline")
	ErrInvalidCallsignConfig   = errors.New("at least one callsign pattern (prefix or suffix) is required")
)

// ServerError is a structured error returned by RegistrationService.InitServer.
// The handler uses Code, Message, and StatusCode directly — no switch dispatch required.
type ServerError struct {
	Code       string // machine-readable, e.g. "SERVER_ALREADY_REGISTERED"
	Message    string // human-readable
	StatusCode int    // HTTP status code to use in the response
	Cause      error  // underlying error for logging; nil for expected domain failures
}

func (e *ServerError) Error() string { return e.Message }

// serverErr is a constructor helper for ServerError.
func serverErr(code, message string, statusCode int, cause error) *ServerError {
	return &ServerError{Code: code, Message: message, StatusCode: statusCode, Cause: cause}
}

// sentinelToServerError maps each sentinel to a ServerError.
func sentinelToServerError(sentinel error) *ServerError {
	switch {
	case errors.Is(sentinel, ErrServerAlreadyRegistered):
		return serverErr("SERVER_ALREADY_REGISTERED",
			"This Discord server is already registered as a VA",
			http.StatusConflict, nil)
	case errors.Is(sentinel, ErrUserNotRegistered):
		return serverErr("USER_NOT_REGISTERED",
			"You must register as a user before initializing a server",
			http.StatusBadRequest, nil)
	case errors.Is(sentinel, ErrInvalidCallsignConfig):
		return serverErr("INVALID_CALLSIGN_CONFIG",
			"At least one callsign pattern (prefix or suffix) is required",
			http.StatusBadRequest, nil)
	case errors.Is(sentinel, ErrVACreationFailed):
		return serverErr("VA_CREATION_FAILED",
			"Failed to create virtual airline",
			http.StatusInternalServerError, sentinel)
	default:
		return serverErr("INTERNAL_ERROR",
			"An unexpected error occurred during server initialization",
			http.StatusInternalServerError, sentinel)
	}
}
