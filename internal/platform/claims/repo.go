package claims

import (
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/platform/roles"

	"gorm.io/gorm"

	"context"
	"time"
)

// Repository manages user-VA membership data for authentication claims
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new claims repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// MembershipResult represents the result of a membership lookup for claims
type MembershipResult struct {
	UserID *string `gorm:"column:user_id"`
	VAID   *string `gorm:"column:va_id"`
	LinkID *string `gorm:"column:link_id"`

	Role            *roles.VARole
	IsActive        *bool
	JoinedAt        *time.Time
	AirtablePilotID *string
	Callsign        *string
	UpdatedAt       *time.Time

	UserExists bool
	VAExists   bool `gorm:"-"`
	IsLinked   bool `gorm:"-"`
}

// DeriveMembershipFlags sets the boolean flags based on pointer values
func (m *MembershipResult) DeriveMembershipFlags() {
	m.UserExists = m.UserID != nil
	m.VAExists = m.VAID != nil
	m.IsLinked = m.LinkID != nil
}

// GetMembershipByDiscordIDs retrieves membership information by Discord user and server IDs
func (r *Repository) GetMembershipByDiscordIDs(ctx context.Context, uid string, sid string) (*MembershipResult, error) {
	var result MembershipResult

	err := r.db.WithContext(ctx).
		Table("users u").
		Select(`
		u.id  AS user_id,
		va.id AS va_id,
		vur.id AS link_id,
		vur.role,
		vur.is_active,
		vur.joined_at,
		vur.airtable_pilot_id,
		vur.callsign,
		vur.updated_at
	`).
		Joins(`
		LEFT JOIN virtual_airlines va
		    ON va.discord_server_id = ?
	`, sid).
		Joins(`
		LEFT JOIN va_user_roles vur
		    ON vur.user_id = u.id
		   AND vur.va_id = va.id
	`).
		Where("u.discord_id = ?", uid).
		Limit(1).
		Scan(&result).Error

	if err != nil {
		logging.Error(("Unable to fetch membership"))
		return nil, err
	}
	result.DeriveMembershipFlags()
	return &result, nil
}
