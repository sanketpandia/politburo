package memberships

import "time"

// JoinVARequest represents the request to join a VA
type JoinVARequest struct {
	Callsign string `json:"callsign" validate:"required"`
}

// JoinVAResponse represents the successful membership creation response
type JoinVAResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	UserID   string `json:"user_id"`
	VAID     string `json:"va_id"`
	Callsign string `json:"callsign"`
	Role     string `json:"role"`
}

// UserDetailResponse represents detailed user information including VA affiliations
type UserDetailResponse struct {
	IsRegistered          bool                `json:"is_registered"`
	GlobalUserExists      bool                `json:"global_user_exists"`
	UserID                string              `json:"user_id,omitempty"`
	DiscordID             string              `json:"discord_id"`
	IFCommunityID         string              `json:"if_community_id,omitempty"`
	IFApiID               *string             `json:"if_api_id,omitempty"`
	UserName              *string             `json:"username,omitempty"`
	IsActive              bool                `json:"is_active"`
	CreatedAt             *time.Time          `json:"created_at,omitempty"`
	Affiliations          []VAAffiliation     `json:"affiliations"`
	CurrentServer         CurrentServerStatus `json:"current_server"`
	CurrentVA             *CurrentVAStatus    `json:"current_va"`
	MembershipsSummary    MembershipsSummary  `json:"memberships_summary"`
	OtherMembershipsCount int                 `json:"other_memberships_count"`
	OtherMemberships      []MembershipSummary `json:"other_memberships,omitempty"`
}

// CurrentServerStatus represents the Discord server's configured VA context.
type CurrentServerStatus struct {
	DiscordServerID string `json:"discord_server_id"`
	IsConfiguredVA  bool   `json:"is_configured_va"`
	VAID            string `json:"va_id,omitempty"`
	VAName          string `json:"va_name,omitempty"`
	VACode          string `json:"va_code,omitempty"`
}

// VAAffiliation represents a user's membership in a virtual airline
type VAAffiliation struct {
	VAID     string    `json:"va_id"`
	VAName   string    `json:"va_name"`
	VACode   string    `json:"va_code"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
	JoinedAt time.Time `json:"joined_at"`
	Callsign string    `json:"callsign,omitempty"`
}

// CurrentVAStatus represents the user's status in the current VA context
type CurrentVAStatus struct {
	IsMember bool   `json:"is_member"`
	VAID     string `json:"va_id,omitempty"`
	VAName   string `json:"va_name,omitempty"`
	VACode   string `json:"va_code,omitempty"`
	Role     string `json:"role,omitempty"`
	IsActive bool   `json:"is_active"`
	Callsign string `json:"callsign,omitempty"`
}

// MembershipsSummary summarizes all known VA affiliations for the user.
type MembershipsSummary struct {
	TotalCount  int `json:"total_count"`
	ActiveCount int `json:"active_count"`
}

// MembershipSummary is a compact safe summary of a VA membership.
type MembershipSummary struct {
	VAID     string `json:"va_id"`
	VAName   string `json:"va_name"`
	VACode   string `json:"va_code"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}
