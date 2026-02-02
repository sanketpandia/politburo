package servers

import "errors"

var (
	ErrServerAlreadyRegistered = errors.New("server is already registered as a VA")
	ErrUserNotRegistered       = errors.New("user must be registered before initializing server")
	ErrVACreationFailed        = errors.New("failed to create virtual airline")
	ErrInvalidCallsignConfig   = errors.New("at least one callsign pattern (prefix or suffix) is required")
)
