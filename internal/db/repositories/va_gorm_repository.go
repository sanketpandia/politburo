package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/platform/roles"
	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
)

// VAGormRepository handles VA table operations using GORM
type VAGormRepository struct {
	db *gorm.DB
}

// NewVAGormRepository creates a new GORM-based VA repository
func NewVAGormRepository(db *gorm.DB) *VAGormRepository {
	return &VAGormRepository{db: db}
}

// GetByID retrieves a VA by its ID
func (r *VAGormRepository) GetByID(ctx context.Context, vaID string) (*gormModels.VA, error) {
	var va gormModels.VA

	err := r.db.WithContext(ctx).
		Where("id = ?", vaID).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch VA: %w", err)
	}

	return &va, nil
}

// GetByDiscordServerID retrieves a VA by Discord server ID
func (r *VAGormRepository) GetByDiscordServerID(ctx context.Context, discordServerID string) (*gormModels.VA, error) {
	var va gormModels.VA

	err := r.db.WithContext(ctx).
		Where("discord_server_id = ?", discordServerID).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch VA: %w", err)
	}

	return &va, nil
}

// GetByCode retrieves a VA by its code
func (r *VAGormRepository) GetByCode(ctx context.Context, code string) (*gormModels.VA, error) {
	var va gormModels.VA

	err := r.db.WithContext(ctx).
		Where("code = ?", code).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch VA: %w", err)
	}

	return &va, nil
}

// GetAll retrieves all active VAs
func (r *VAGormRepository) GetAll(ctx context.Context) ([]gormModels.VA, error) {
	var vas []gormModels.VA

	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Find(&vas).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VAs: %w", err)
	}

	return vas, nil
}

// UpdateFlightModesConfig updates the flight modes configuration for a VA
func (r *VAGormRepository) UpdateFlightModesConfig(ctx context.Context, vaID string, config gormModels.JSONB) error {
	result := r.db.WithContext(ctx).
		Model(&gormModels.VA{}).
		Where("id = ?", vaID).
		Update("flight_modes_config", config)

	if result.Error != nil {
		return fmt.Errorf("failed to update flight modes config: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("VA not found with ID: %s", vaID)
	}

	return nil
}

// Create inserts a new VA into the database
func (r *VAGormRepository) Create(ctx context.Context, name, code, discordID string, isActive bool) (*gormModels.VA, error) {
	va := &gormModels.VA{
		Name:      name,
		Code:      code,
		DiscordID: discordID,
		IsActive:  isActive,
	}

	err := r.db.WithContext(ctx).Create(va).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create VA: %w", err)
	}

	return va, nil
}

// CreateWithAdmin creates a new VA and assigns an admin user in a single transaction
func (r *VAGormRepository) CreateWithAdmin(ctx context.Context, name, code, discordID string, isActive bool, adminUserID string) (*gormModels.VA, *gormModels.UserVARole, error) {
	var va *gormModels.VA
	var membership *gormModels.UserVARole

	// Execute in transaction
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create VA
		va = &gormModels.VA{
			Name:      name,
			Code:      code,
			DiscordID: discordID,
			IsActive:  isActive,
		}

		if err := tx.Create(va).Error; err != nil {
			return fmt.Errorf("failed to create VA: %w", err)
		}

		// Create admin membership
		membership = &gormModels.UserVARole{
			UserID:   adminUserID,
			VAID:     va.ID,
			Role:     roles.RoleAdmin,
			IsActive: true,
			Callsign: "",
		}

		if err := tx.Create(membership).Error; err != nil {
			return fmt.Errorf("failed to create admin membership: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return va, membership, nil
}

// GetVAConfigs retrieves all configuration key-value pairs for a VA
func (r *VAGormRepository) GetVAConfigs(ctx context.Context, vaID string) ([]gormModels.VAConfig, error) {
	var configs []gormModels.VAConfig

	err := r.db.WithContext(ctx).
		Where("va_id = ?", vaID).
		Find(&configs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA configs: %w", err)
	}

	return configs, nil
}

// UpsertVAConfig inserts or updates a configuration key-value pair for a VA
func (r *VAGormRepository) UpsertVAConfig(ctx context.Context, vaID, key, value string) error {
	// GORM's Save will update if record exists (based on primary key/unique constraint)
	// But since we have a unique constraint on (va_id, config_key), we need to use raw SQL
	err := r.db.WithContext(ctx).
		Exec(`
			INSERT INTO va_configs (va_id, config_key, config_value, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
			ON CONFLICT (va_id, config_key)
			DO UPDATE SET
				config_value = EXCLUDED.config_value,
				updated_at = NOW()
		`, vaID, key, value).Error

	if err != nil {
		return fmt.Errorf("failed to upsert VA config: %w", err)
	}

	return nil
}

// GetAllActiveVACallsignConfigs retrieves callsign prefix/suffix for all active VAs
func (r *VAGormRepository) GetAllActiveVACallsignConfigs(ctx context.Context) ([]map[string]string, error) {
	type ConfigRow struct {
		VAID        string `gorm:"column:id"`
		ConfigKey   string `gorm:"column:config_key"`
		ConfigValue string `gorm:"column:config_value"`
	}

	var rows []ConfigRow
	err := r.db.WithContext(ctx).
		Table("va_configs vc").
		Select("va.id, vc.config_key, vc.config_value").
		Joins("INNER JOIN virtual_airlines va ON va.id = vc.va_id").
		Where("vc.config_key IN (?, ?)", "callsign_suffix", "callsign_prefix").
		Where("va.is_active = ?", true).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA callsign configs: %w", err)
	}

	// Group results by VA ID
	vaConfigsMap := make(map[string]map[string]string)
	for _, row := range rows {
		if _, exists := vaConfigsMap[row.VAID]; !exists {
			vaConfigsMap[row.VAID] = map[string]string{
				"va_id": row.VAID,
			}
		}
		// Only add prefix/suffix if they have values
		if row.ConfigValue != "" {
			vaConfigsMap[row.VAID][row.ConfigKey] = row.ConfigValue
		}
	}

	// Convert map to slice
	result := make([]map[string]string, 0, len(vaConfigsMap))
	for _, config := range vaConfigsMap {
		result = append(result, config)
	}

	return result, nil
}
