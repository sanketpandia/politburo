package entities

import (
	"infinite-experiment/politburo/internal/platform/roles"
	"time"
)

type User struct {
	ID            string    `db:"id"`
	DiscordID     string    `db:"discord_id"`
	IFCommunityID string    `db:"if_community_id"`
	IFApiID       *string   `db:"if_api_id"`
	IsActive      bool      `db:"is_active"`
	UserName      *string   `db:"username"`
	OTP           *string   `db:"otp"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type Membership struct {
	UserID *string           `db:"user_id"`
	VAID   *string           `db:"va_id"`
	Role   *roles.VARole `db:"role"`
}

type UserVARole struct {
	ID       string           `db:"id"`
	UserID   string           `db:"user_id"`
	VAID     string           `db:"va_id"`
	Role     roles.VARole `db:"role"`
	IsActive bool             `db:"is_active"`
	JoinedAt time.Time        `db:"joined_at"`
	Callsign string           `db:"callsign"`
}
