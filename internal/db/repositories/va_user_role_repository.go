package repositories

import (
	"context"
	"fmt"

	models "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
)

// VAUserRoleRepository manages VA user role data with GORM
type VAUserRoleRepository struct {
	db *gorm.DB
}

// NewVAUserRoleRepository creates a new VA user role repository
func NewVAUserRoleRepository(db *gorm.DB) *VAUserRoleRepository {
	return &VAUserRoleRepository{db: db}
}

// GetByID retrieves a user's VA role by ID
func (r *VAUserRoleRepository) GetByID(ctx context.Context, id string) (*models.UserVARole, error) {
	var role models.UserVARole

	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("VA").
		Where("id = ?", id).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("VA user role not found")
		}
		return nil, fmt.Errorf("failed to fetch VA user role: %w", err)
	}

	return &role, nil
}

// GetByUserAndVA retrieves a user's role in a specific VA
func (r *VAUserRoleRepository) GetByUserAndVA(ctx context.Context, userID, vaID string) (*models.UserVARole, error) {
	var role models.UserVARole

	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("VA").
		Where("user_id = ? AND va_id = ?", userID, vaID).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user is not a member of this VA")
		}
		return nil, fmt.Errorf("failed to fetch VA user role: %w", err)
	}

	return &role, nil
}

// GetAllByUserID retrieves all VA roles for a specific user (with VA details)
func (r *VAUserRoleRepository) GetAllByUserID(ctx context.Context, userID string) ([]models.UserVARole, error) {
	var roles []models.UserVARole

	err := r.db.WithContext(ctx).
		Preload("VA").
		Where("user_id = ? AND va_user_roles.is_active = ?", userID, true).
		Joins("JOIN virtual_airlines va ON va.id = va_user_roles.va_id").
		Order("va.name ASC").
		Find(&roles).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch user VA roles: %w", err)
	}

	return roles, nil
}

// GetAllByVAID retrieves all users in a specific VA with their roles
func (r *VAUserRoleRepository) GetAllByVAID(ctx context.Context, vaID string) ([]models.UserVARole, error) {
	var roles []models.UserVARole

	err := r.db.WithContext(ctx).
		Preload("User").
		Where("va_id = ? AND is_active = ?", vaID, true).
		Order("callsign ASC").
		Find(&roles).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA users: %w", err)
	}

	return roles, nil
}

// Create creates a new VA user role
func (r *VAUserRoleRepository) Create(ctx context.Context, role *models.UserVARole) error {
	err := r.db.WithContext(ctx).Create(role).Error
	if err != nil {
		return fmt.Errorf("failed to create VA user role: %w", err)
	}
	return nil
}

// Update updates an existing VA user role
func (r *VAUserRoleRepository) Update(ctx context.Context, role *models.UserVARole) error {
	// Omit associations to avoid trying to update User and VA tables
	err := r.db.WithContext(ctx).Omit("User", "VA").Save(role).Error
	if err != nil {
		return fmt.Errorf("failed to update VA user role: %w", err)
	}
	return nil
}

// Delete deletes a VA user role (soft delete by setting is_active to false)
func (r *VAUserRoleRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).
		Model(&models.UserVARole{}).
		Where("id = ?", id).
		Update("is_active", false).Error

	if err != nil {
		return fmt.Errorf("failed to delete VA user role: %w", err)
	}
	return nil
}

// GetByDiscordIDs retrieves user's role in VA by Discord IDs
func (r *VAUserRoleRepository) GetByDiscordIDs(ctx context.Context, discordUserID, discordServerID string) (*models.UserVARole, error) {
	var role models.UserVARole

	err := r.db.WithContext(ctx).
		Joins("JOIN users ON users.id = va_user_roles.user_id").
		Joins("JOIN virtual_airlines ON virtual_airlines.id = va_user_roles.va_id").
		Preload("User").
		Preload("VA").
		Where("users.discord_id = ? AND virtual_airlines.discord_server_id = ? AND va_user_roles.is_active = ?",
			discordUserID, discordServerID, true).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found in this VA")
		}
		return nil, fmt.Errorf("failed to fetch VA user role by Discord IDs: %w", err)
	}

	return &role, nil
}

// GetByCallsignAndVAID retrieves a user's role by callsign and VA ID, optionally excluding a specific ID
func (r *VAUserRoleRepository) GetByCallsignAndVAID(ctx context.Context, callsign, vaID, excludeID string) (*models.UserVARole, error) {
	var role models.UserVARole

	query := r.db.WithContext(ctx).
		Where("va_id = ? AND callsign = ? AND is_active = ?", vaID, callsign, true)

	// Exclude a specific role ID if provided (used to avoid matching current pilot)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}

	err := query.First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found is OK, return nil without error
		}
		return nil, fmt.Errorf("failed to check callsign uniqueness: %w", err)
	}

	return &role, nil
}
