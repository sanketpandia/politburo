package memberships

import (
	"time"

	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/va"
)

// UserVARole represents the relationship between a user and a VA
type UserVARole struct {
	ID                string       `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID            string       `gorm:"column:user_id;type:uuid"`
	VAID              string       `gorm:"column:va_id;type:uuid"`
	Role              roles.VARole `gorm:"column:role;type:va_role"`
	IsActive          bool         `gorm:"column:is_active;default:true"`
	JoinedAt          time.Time    `gorm:"column:joined_at;autoCreateTime"`
	Callsign          string       `gorm:"column:callsign"`
	AirtablePilotID   *string      `gorm:"column:airtable_pilot_id"`
	CareerModePilotID *string      `gorm:"column:career_mode_pilot_id"`
	UpdatedAt         time.Time    `gorm:"column:updated_at;autoUpdateTime"`

	// Relationships
	User users.User `gorm:"foreignKey:UserID"`
	VA   va.VA      `gorm:"foreignKey:VAID"`
}

// TableName specifies the table name for GORM
func (UserVARole) TableName() string {
	return "va_user_roles"
}

// UserStatusResult represents the complete user status with affiliations
// Returned by GetUserStatusByUserID - includes all VA memberships and current VA context
type UserStatusResult struct {
	UserID        string
	DiscordID     string
	IFCommunityID string
	IFApiID       *string
	IsActive      bool
	CreatedAt     time.Time

	// Affiliations - array of VA memberships
	Affiliations []Affiliation

	// Current VA (from vaID parameter context)
	CurrentVA *CurrentVAStatus
}

// Affiliation represents a user's membership in a VA
type Affiliation struct {
	VAID     string
	VAName   string
	VACode   string
	Role     roles.VARole
	IsActive bool
	JoinedAt time.Time
	Callsign string
}

// CurrentVAStatus represents the user's status in the current VA context
type CurrentVAStatus struct {
	IsMember bool
	VAID     *string
	VAName   *string
	VACode   *string
	Role     *roles.VARole
	IsActive *bool
	Callsign *string
}
