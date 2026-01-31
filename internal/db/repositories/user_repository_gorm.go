package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/models/entities"
	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
)

type UserRepositoryGORM struct {
	db *gorm.DB
}

// NewUserRepositoryGORM creates a new GORM-based user repository
func NewUserRepositoryGORM(db *gorm.DB) *UserRepositoryGORM {
	return &UserRepositoryGORM{db: db}
}

// GetUserWithVAAffiliations retrieves a user by Discord ID with all VA affiliations preloaded
func (r *UserRepositoryGORM) GetUserWithVAAffiliations(ctx context.Context, userDiscordID string) (*gormModels.User, error) {
	var user gormModels.User

	err := r.db.WithContext(ctx).
		Preload("UserVARoles.VA").
		Where("discord_id = ?", userDiscordID).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found with discord_id %s: %w", userDiscordID, gorm.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("failed to fetch user with affiliations: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by UUID
func (r *UserRepositoryGORM) GetByID(ctx context.Context, userID string) (*gormModels.User, error) {
	var user gormModels.User

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
func (r *UserRepositoryGORM) GetUserByDiscordID(ctx context.Context, discordID string) (*gormModels.User, error) {
	var user gormModels.User

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
func (r *UserRepositoryGORM) FindUserMembership(ctx context.Context, discordServerID string, userDiscordID string) (*entities.Membership, error) {
	// Step 1: Find user by discord_id
	var user gormModels.User
	userErr := r.db.WithContext(ctx).
		Where("discord_id = ?", userDiscordID).
		First(&user).Error

	// Step 2: Find VA by discord_server_id
	var va gormModels.VA
	vaErr := r.db.WithContext(ctx).
		Where("discord_server_id = ?", discordServerID).
		First(&va).Error

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
		result.VAID = &va.ID
	} else if vaErr != gorm.ErrRecordNotFound {
		// Real error (not just "not found")
		return nil, fmt.Errorf("failed to fetch VA: %w", vaErr)
	}

	// Step 3: If both user and VA exist, try to find the role relationship
	if result.UserID != nil && result.VAID != nil {
		var userVARole gormModels.UserVARole
		roleErr := r.db.WithContext(ctx).
			Where("user_id = ? AND va_id = ?", user.ID, va.ID).
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
func (r *UserRepositoryGORM) InsertUser(ctx context.Context, discordID, ifCommunityID string, ifApiID *string, isActive bool) (*gormModels.User, error) {
	user := &gormModels.User{
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
func (r *UserRepositoryGORM) DeleteAllUsers(ctx context.Context) error {
	// Delete in correct order due to foreign keys
	// 1. Delete all user-VA roles
	if err := r.db.WithContext(ctx).Where("id IS NOT NULL").Delete(&gormModels.UserVARole{}).Error; err != nil {
		return fmt.Errorf("failed to delete user roles: %w", err)
	}

	// 2. Delete all VAs
	if err := r.db.WithContext(ctx).Where("id IS NOT NULL").Delete(&gormModels.VA{}).Error; err != nil {
		return fmt.Errorf("failed to delete VAs: %w", err)
	}

	// 3. Delete all users
	if err := r.db.WithContext(ctx).Where("discord_id IS NOT NULL").Delete(&gormModels.User{}).Error; err != nil {
		return fmt.Errorf("failed to delete users: %w", err)
	}

	return nil
}

// InsertMembership creates a new membership (user-VA relationship) with a role
func (r *UserRepositoryGORM) InsertMembership(ctx context.Context, userID, vaID string, role, callsign string) (*gormModels.UserVARole, error) {
	membership := &gormModels.UserVARole{
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
func (r *UserRepositoryGORM) UpdateUserRole(ctx context.Context, vaID, userID, newRole string) error {
	result := r.db.WithContext(ctx).
		Model(&gormModels.UserVARole{}).
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
