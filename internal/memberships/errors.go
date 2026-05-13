package memberships

import (
	"errors"
	"net/http"
)

// Sentinel errors for the membership flow.
var (
	ErrUserNotFound          = errors.New("user not found")
	ErrVANotFound            = errors.New("virtual airline not found")
	ErrUserAlreadyMember     = errors.New("user is already a member of this VA")
	ErrCallsignTaken         = errors.New("callsign is already taken in this VA")
	ErrInvalidCallsign       = errors.New("invalid callsign format")
	ErrMembershipCreation    = errors.New("failed to create membership")
	ErrCallsignNotInAirtable = errors.New("callsign not found in Airtable for this VA")
)

// MembershipError is a structured error returned by Service.JoinVA.
// The handler uses Code, Message, and StatusCode directly — no switch dispatch required.
type MembershipError struct {
	Code       string // machine-readable, e.g. "USER_NOT_FOUND"
	Message    string // human-readable
	StatusCode int    // HTTP status code to use in the response
	Cause      error  // underlying error for logging; nil for expected domain failures
}

func (e *MembershipError) Error() string { return e.Message }

// membershipErr is a constructor helper for MembershipError.
func membershipErr(code, message string, statusCode int, cause error) *MembershipError {
	return &MembershipError{Code: code, Message: message, StatusCode: statusCode, Cause: cause}
}

// sentinelToMembershipError maps each sentinel to a MembershipError.
func sentinelToMembershipError(sentinel error) *MembershipError {
	switch {
	case errors.Is(sentinel, ErrUserNotFound):
		return membershipErr("USER_NOT_FOUND", "User not found", http.StatusNotFound, nil)
	case errors.Is(sentinel, ErrVANotFound):
		return membershipErr("VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound, nil)
	case errors.Is(sentinel, ErrUserAlreadyMember):
		return membershipErr("ALREADY_MEMBER", "You are already a member of this VA", http.StatusConflict, nil)
	case errors.Is(sentinel, ErrCallsignTaken):
		return membershipErr("CALLSIGN_TAKEN", "This callsign is already taken", http.StatusConflict, nil)
	case errors.Is(sentinel, ErrInvalidCallsign):
		return membershipErr("INVALID_CALLSIGN", "Invalid callsign format", http.StatusBadRequest, nil)
	case errors.Is(sentinel, ErrCallsignNotInAirtable):
		return membershipErr("CALLSIGN_NOT_IN_AIRTABLE",
			"Your callsign could not be found in Airtable. Please enter the correct callsign as it appears in the linked Airtable.",
			http.StatusBadRequest, nil)
	case errors.Is(sentinel, ErrMembershipCreation):
		return membershipErr("MEMBERSHIP_CREATION_FAILED", "Failed to create membership", http.StatusInternalServerError, sentinel)
	default:
		return membershipErr("INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError, sentinel)
	}
}
