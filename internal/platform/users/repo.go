package users

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/entities"
	"infinite-experiment/politburo/internal/platform/roles"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new GORM-based user repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetUserWithVAAffiliations retrieves a user by Discord ID with all VA affiliations preloaded
func (r *Repository) GetUserWithVAAffiliations(ctx context.Context, userDiscordID string) (*User, error) {
	var user User

	err := r.db.WithContext(ctx).
		Preload("UserVARoles").
		Where("discord_id = ?", userDiscordID).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found with discord_id: %s", userDiscordID)
		}
		return nil, fmt.Errorf("failed to fetch user with affiliations: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by UUID
func (r *Repository) GetByID(ctx context.Context, userID string) (*User, error) {
	var user User

	err := r.db.WithContext(ctx).
		Where("id = ?", userID).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return &user, nil
}

// GetUserByDiscordID retrieves a user by Discord ID without relationships
func (r *Repository) GetUserByDiscordID(ctx context.Context, discordID string) (*User, error) {
	var user User

	err := r.db.WithContext(ctx).
		Where("discord_id = ?", discordID).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return &user, nil
}

// FindUserMembership retrieves membership information for a user in a VA by Discord IDs
// Returns a Membership DTO with nullable fields (UserID, VAID, Role can all be nil)
// This replicates the original CTE-based SQL query behavior
// Note: This function has a dependency on VA model which will be in internal/va/ package
// For now, we'll need to use raw queries or adjust this later
func (r *Repository) FindUserMembership(ctx context.Context, discordServerID string, userDiscordID string) (*entities.Membership, error) {
	// Step 1: Find user by discord_id
	var user User
	userErr := r.db.WithContext(ctx).
		Where("discord_id = ?", userDiscordID).
		First(&user).Error

	// Step 2: Find VA by discord_server_id (using raw query to avoid circular dependency)
	var vaID string
	vaErr := r.db.WithContext(ctx).
		Table("virtual_airlines").
		Select("id").
		Where("discord_server_id = ?", discordServerID).
		Scan(&vaID).Error

	// Initialize result with nullable fields
	result := &entities.Membership{
		UserID: nil,
		VAID:   nil,
		Role:   nil,
	}

	// If user found, set UserID
	if userErr == nil {
		result.UserID = &user.ID
	} else if userErr != gorm.ErrRecordNotFound {
		// Real error (not just "not found")
		return nil, fmt.Errorf("failed to fetch user: %w", userErr)
	}

	// If VA found, set VAID
	if vaErr == nil {
		result.VAID = &vaID
	} else if vaErr != gorm.ErrRecordNotFound {
		// Real error (not just "not found")
		return nil, fmt.Errorf("failed to fetch VA: %w", vaErr)
	}

	// Step 3: If both user and VA exist, try to find the role relationship
	if result.UserID != nil && result.VAID != nil {
		var userVARole UserVARole
		roleErr := r.db.WithContext(ctx).
			Where("user_id = ? AND va_id = ?", user.ID, vaID).
			First(&userVARole).Error

		if roleErr == nil {
			result.Role = &userVARole.Role
		} else if roleErr != gorm.ErrRecordNotFound {
			// Real error (not just "not found")
			return nil, fmt.Errorf("failed to fetch role: %w", roleErr)
		}
		// If role not found, it stays nil (which is fine)
	}

	return result, nil
}

// InsertUser creates a new user in the database
func (r *Repository) InsertUser(ctx context.Context, discordID, ifCommunityID string, ifApiID *string, isActive bool) (*User, error) {
	user := &User{
		DiscordID:     discordID,
		IFCommunityID: ifCommunityID,
		IFApiID:       ifApiID,
		IsActive:      isActive,
	}

	err := r.db.WithContext(ctx).Create(user).Error
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return user, nil
}

// DeleteAllUsers deletes all users, their roles, and VA associations (DANGER: God mode only)
func (r *Repository) DeleteAllUsers(ctx context.Context) error {
	// Delete in correct order due to foreign keys
	// 1. Delete all user-VA roles
	if err := r.db.WithContext(ctx).Where("id IS NOT NULL").Delete(&UserVARole{}).Error; err != nil {
		return fmt.Errorf("failed to delete user roles: %w", err)
	}

	// 2. Delete all VAs (using table name to avoid importing VA model)
	if err := r.db.WithContext(ctx).Exec("DELETE FROM virtual_airlines WHERE id IS NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to delete VAs: %w", err)
	}

	// 3. Delete all users
	if err := r.db.WithContext(ctx).Where("discord_id IS NOT NULL").Delete(&User{}).Error; err != nil {
		return fmt.Errorf("failed to delete users: %w", err)
	}

	return nil
}

// InsertMembership creates a new membership (user-VA relationship) with a role
func (r *Repository) InsertMembership(ctx context.Context, userID, vaID string, role, callsign string) (*UserVARole, error) {
	membership := &UserVARole{
		UserID:   userID,
		VAID:     vaID,
		Role:     roles.VARole(role),
		Callsign: callsign,
		IsActive: true,
	}

	err := r.db.WithContext(ctx).Create(membership).Error
	if err != nil {
		return nil, fmt.Errorf("failed to insert membership: %w", err)
	}

	return membership, nil
}

// UpdateUserRole updates a user's role in a specific VA
func (r *Repository) UpdateUserRole(ctx context.Context, vaID, userID, newRole string) error {
	result := r.db.WithContext(ctx).
		Model(&UserVARole{}).
		Where("va_id = ? AND user_id = ?", vaID, userID).
		Update("role", roles.VARole(newRole))

	if result.Error != nil {
		return fmt.Errorf("failed to update user role: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found in this VA")
	}

	return nil
}
