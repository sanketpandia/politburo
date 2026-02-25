package memberships

import (
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/platform/roles"

	"gorm.io/gorm"

	"context"
	"time"
)

// Repository manages user-VA membership data with GORM
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new membership repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

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

func (m *MembershipResult) DeriveMembershipFlags() {
	m.UserExists = m.UserID != nil
	m.VAExists = m.VAID != nil
	m.IsLinked = m.LinkID != nil
}

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

// GetUserStatusByUserID retrieves user with all VA affiliations by user ID
// Uses efficient queries to fetch user details and all memberships
func (r *Repository) GetUserStatusByUserID(ctx context.Context, userID string, vaID string) (*UserStatusResult, error) {
	logging.Debug("Fetching user status", "user_id", userID, "va_id", vaID)

	// Step 1: Get user basic info
	type UserInfo struct {
		ID            string  `gorm:"column:id"`
		DiscordID     string  `gorm:"column:discord_id"`
		IFCommunityID string  `gorm:"column:if_community_id"`
		IFApiID       *string `gorm:"column:if_api_id"`
		IsActive      bool    `gorm:"column:is_active"`
		CreatedAt     time.Time `gorm:"column:created_at"`
	}

	var userInfo UserInfo
	err := r.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		First(&userInfo).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logging.Error("User not found", "error", err, "user_id", userID)
			return nil, err
		}
		logging.Error("Failed to fetch user", "error", err, "user_id", userID)
		return nil, err
	}

	// Step 2: Get all affiliations with VA details (single query with JOIN)
	type AffiliationRow struct {
		VAID     string       `gorm:"column:va_id"`
		VAName   string       `gorm:"column:va_name"`
		VACode   string       `gorm:"column:va_code"`
		Role     roles.VARole `gorm:"column:role"`
		IsActive bool         `gorm:"column:is_active"`
		JoinedAt time.Time    `gorm:"column:joined_at"`
		Callsign string       `gorm:"column:callsign"`
	}

	var affiliationRows []AffiliationRow
	err = r.db.WithContext(ctx).
		Table("va_user_roles vur").
		Select(`
			vur.va_id,
			va.name AS va_name,
			va.code AS va_code,
			vur.role,
			vur.is_active,
			vur.joined_at,
			vur.callsign
		`).
		Joins("JOIN virtual_airlines va ON va.id = vur.va_id").
		Where("vur.user_id = ?", userID).
		Scan(&affiliationRows).Error

	if err != nil {
		logging.Error("Failed to fetch affiliations", "error", err, "user_id", userID)
		return nil, err
	}

	// Step 3: Build result
	result := &UserStatusResult{
		UserID:        userInfo.ID,
		DiscordID:     userInfo.DiscordID,
		IFCommunityID: userInfo.IFCommunityID,
		IFApiID:       userInfo.IFApiID,
		IsActive:      userInfo.IsActive,
		CreatedAt:     userInfo.CreatedAt,
		Affiliations:  make([]Affiliation, 0, len(affiliationRows)),
		CurrentVA:     nil,
	}

	// Convert rows to Affiliation structs and identify current VA
	for _, row := range affiliationRows {
		affiliation := Affiliation{
			VAID:     row.VAID,
			VAName:   row.VAName,
			VACode:   row.VACode,
			Role:     row.Role,
			IsActive: row.IsActive,
			JoinedAt: row.JoinedAt,
			Callsign: row.Callsign,
		}
		result.Affiliations = append(result.Affiliations, affiliation)

		// If this is the current VA (matches vaID parameter), set CurrentVA
		if row.VAID == vaID {
			result.CurrentVA = &CurrentVAStatus{
				IsMember: true,
				VAID:     &row.VAID,
				VAName:   &row.VAName,
				VACode:   &row.VACode,
				Role:     &row.Role,
				IsActive: &row.IsActive,
				Callsign: &row.Callsign,
			}
		}
	}

	// If CurrentVA is still nil, user is not a member of the current VA
	if result.CurrentVA == nil {
		result.CurrentVA = &CurrentVAStatus{
			IsMember: false,
		}
	}

	logging.Debug("User status fetched successfully", "user_id", userID, "affiliations_count", len(result.Affiliations))
	return result, nil
}

// GetByID retrieves a user's VA role by ID
// func (r *Repository) GetByID(ctx context.Context, id string) (*UserVARole, error) {
// 	var role UserVARole

// 	err := r.db.WithContext(ctx).
// 		Preload("User").
// 		Preload("VA").
// 		Where("id = ?", id).
// 		First(&role).Error

// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return nil, fmt.Errorf("VA user role not found")
// 		}
// 		return nil, fmt.Errorf("failed to fetch VA user role: %w", err)
// 	}

// 	return &role, nil
// }

// // GetByUserAndVA retrieves a user's role in a specific VA
// func (r *Repository) GetByUserAndVA(ctx context.Context, userID, vaID string) (*UserVARole, error) {
// 	var role UserVARole

// 	err := r.db.WithContext(ctx).
// 		Preload("User").
// 		Preload("VA").
// 		Where("user_id = ? AND va_id = ?", userID, vaID).
// 		First(&role).Error

// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return nil, fmt.Errorf("user is not a member of this VA")
// 		}
// 		return nil, fmt.Errorf("failed to fetch VA user role: %w", err)
// 	}

// 	return &role, nil
// }

// // GetAllByUserID retrieves all VA roles for a specific user (with VA details)
// func (r *Repository) GetAllByUserID(ctx context.Context, userID string) ([]UserVARole, error) {
// 	var roles []UserVARole

// 	err := r.db.WithContext(ctx).
// 		Preload("VA").
// 		Where("user_id = ? AND va_user_roles.is_active = ?", userID, true).
// 		Joins("JOIN virtual_airlines va ON va.id = va_user_roles.va_id").
// 		Order("va.name ASC").
// 		Find(&roles).Error

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch user VA roles: %w", err)
// 	}

// 	return roles, nil
// }

// // GetAllByVAID retrieves all users in a specific VA with their roles
// func (r *Repository) GetAllByVAID(ctx context.Context, vaID string) ([]UserVARole, error) {
// 	var roles []UserVARole

// 	err := r.db.WithContext(ctx).
// 		Preload("User").
// 		Where("va_id = ? AND is_active = ?", vaID, true).
// 		Find(&roles).Error

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch VA users: %w", err)
// 	}

// 	return roles, nil
// }

// // Create creates a new VA user role
// func (r *Repository) Create(ctx context.Context, role *UserVARole) error {
// 	err := r.db.WithContext(ctx).Create(role).Error
// 	if err != nil {
// 		return fmt.Errorf("failed to create VA user role: %w", err)
// 	}
// 	return nil
// }

// // Update updates an existing VA user role
// func (r *Repository) Update(ctx context.Context, role *UserVARole) error {
// 	// Omit associations to avoid trying to update User and VA tables
// 	err := r.db.WithContext(ctx).Omit("User", "VA").Save(role).Error
// 	if err != nil {
// 		return fmt.Errorf("failed to update VA user role: %w", err)
// 	}
// 	return nil
// }

// // Delete deletes a VA user role (soft delete by setting is_active to false)
// func (r *Repository) Delete(ctx context.Context, id string) error {
// 	err := r.db.WithContext(ctx).
// 		Model(&UserVARole{}).
// 		Where("id = ?", id).
// 		Update("is_active", false).Error

// 	if err != nil {
// 		return fmt.Errorf("failed to delete VA user role: %w", err)
// 	}
// 	return nil
// }

// // GetByDiscordIDs retrieves user's role in VA by Discord IDs
// func (r *Repository) GetByDiscordIDs(ctx context.Context, discordUserID, discordServerID string) (*UserVARole, error) {
// 	var role UserVARole

// 	err := r.db.WithContext(ctx).
// 		Joins("JOIN users ON users.id = va_user_roles.user_id").
// 		Joins("JOIN virtual_airlines ON virtual_airlines.id = va_user_roles.va_id").
// 		Preload("User").
// 		Preload("VA").
// 		Where("users.discord_id = ? AND virtual_airlines.discord_server_id = ? AND va_user_roles.is_active = ?",
// 			discordUserID, discordServerID, true).
// 		First(&role).Error

// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return nil, fmt.Errorf("user not found in this VA")
// 		}
// 		return nil, fmt.Errorf("failed to fetch VA user role by Discord IDs: %w", err)
// 	}

// 	return &role, nil
// }

// // GetByCallsignAndVAID retrieves a user's role by callsign and VA ID, optionally excluding a specific ID
// func (r *Repository) GetByCallsignAndVAID(ctx context.Context, callsign, vaID, excludeID string) (*UserVARole, error) {
// 	var role UserVARole

// 	query := r.db.WithContext(ctx).
// 		Where("va_id = ? AND callsign = ? AND is_active = ?", vaID, callsign, true)

// 	// Exclude a specific role ID if provided (used to avoid matching current pilot)
// 	if excludeID != "" {
// 		query = query.Where("id != ?", excludeID)
// 	}

// 	err := query.First(&role).Error

// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return nil, nil // Not found is OK, return nil without error
// 		}
// 		return nil, fmt.Errorf("failed to check callsign uniqueness: %w", err)
// 	}

// 	return &role, nil
// }
