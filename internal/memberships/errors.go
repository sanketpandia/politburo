package memberships

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrVANotFound         = errors.New("virtual airline not found")
	ErrUserAlreadyMember  = errors.New("user is already a member of this VA")
	ErrCallsignTaken      = errors.New("callsign is already taken in this VA")
	ErrInvalidCallsign    = errors.New("invalid callsign format")
	ErrMembershipCreation = errors.New("failed to create membership")
)
