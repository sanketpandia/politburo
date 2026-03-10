package users

import (
	"infinite-experiment/politburo/internal/platform/roles"
	"time"
)

type User struct {
	ID            string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	DiscordID     string    `gorm:"column:discord_id;uniqueIndex"`
	IFCommunityID string    `gorm:"column:if_community_id"`
	IFApiID       *string   `gorm:"column:if_api_id;type:uuid"`
	IsActive      bool      `gorm:"column:is_active;default:false"`
	UserName      *string   `gorm:"column:username"`
	OTP           *string   `gorm:"column:otp"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// Relationships
	UserVARoles []UserVARole `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for GORM
func (User) TableName() string {
	return "users"
}

type UserVARole struct {
	ID                  string       `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID              string       `gorm:"column:user_id;type:uuid"`
	VAID                string       `gorm:"column:va_id;type:uuid"`
	Role                roles.VARole `gorm:"column:role;type:va_role"`
	IsActive            bool         `gorm:"column:is_active;default:true"`
	JoinedAt            time.Time    `gorm:"column:joined_at;autoCreateTime"`
	Callsign            string       `gorm:"column:callsign"`
	AirtablePilotID     *string      `gorm:"column:airtable_pilot_id"`
	CareerModePilotID   *string      `gorm:"column:career_mode_pilot_id"`
	UpdatedAt           time.Time    `gorm:"column:updated_at;autoUpdateTime"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for GORM
func (UserVARole) TableName() string {
	return "va_user_roles"
}
