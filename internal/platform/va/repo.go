package va

import (
	"context"
	"fmt"

	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
)

// Repository handles VA table operations using GORM
// Consolidates va_gorm_repository.go + va_gorm_repository_extended.go
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new GORM-based VA repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetByID retrieves a VA by its ID
func (r *Repository) GetByID(ctx context.Context, vaID string) (*VA, error) {
	var va VA

	err := r.db.WithContext(ctx).
		Where("id = ?", vaID).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("VA not found")
		}
		return nil, fmt.Errorf("failed to fetch VA: %w", err)
	}

	return &va, nil
}

// GetByDiscordServerID retrieves a VA by Discord server ID
func (r *Repository) GetByDiscordServerID(ctx context.Context, discordServerID string) (*VA, error) {
	var va VA

	err := r.db.WithContext(ctx).
		Where("discord_server_id = ?", discordServerID).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("VA not found with Discord server ID: %s", discordServerID)
		}
		return nil, fmt.Errorf("failed to fetch VA by Discord server ID: %w", err)
	}

	return &va, nil
}

// GetByCode retrieves a VA by its code (e.g., "SIA", "UAL")
func (r *Repository) GetByCode(ctx context.Context, code string) (*VA, error) {
	var va VA

	err := r.db.WithContext(ctx).
		Where("code = ?", code).
		First(&va).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("VA not found with code: %s", code)
		}
		return nil, fmt.Errorf("failed to fetch VA by code: %w", err)
	}

	return &va, nil
}

// GetAll retrieves all active VAs ordered by name
func (r *Repository) GetAll(ctx context.Context) ([]VA, error) {
	var vas []VA

	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("name ASC").
		Find(&vas).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VAs: %w", err)
	}

	return vas, nil
}

// Create creates a new VA
func (r *Repository) Create(ctx context.Context, va *VA) error {
	err := r.db.WithContext(ctx).Create(va).Error
	if err != nil {
		return fmt.Errorf("failed to create VA: %w", err)
	}
	return nil
}

// CreateWithParameters creates a new VA using individual parameters (legacy method)
func (r *Repository) CreateWithParameters(ctx context.Context, name, code, discordID string, isActive bool) (*VA, error) {
	va := &VA{
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

// Update updates an existing VA
func (r *Repository) Update(ctx context.Context, va *VA) error {
	err := r.db.WithContext(ctx).Save(va).Error
	if err != nil {
		return fmt.Errorf("failed to update VA: %w", err)
	}
	return nil
}

// CreateWithAdmin is deprecated - use Create() and memberships.Service.Create() separately
// to avoid circular dependencies between va and memberships packages

// GetFlightModesConfig retrieves the flight modes configuration for a VA
func (r *Repository) GetFlightModesConfig(ctx context.Context, vaID string) (map[string]interface{}, error) {
	var vaGorm gormModels.VA

	err := r.db.WithContext(ctx).
		Where("id = ?", vaID).
		Select("flight_modes_config").
		First(&vaGorm).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to fetch flight modes config: %w", err)
	}

	if vaGorm.FlightModesConfig == nil {
		return map[string]interface{}{}, nil
	}

	return vaGorm.FlightModesConfig, nil
}

// UpdateFlightModesConfig updates the flight modes configuration for a VA
func (r *Repository) UpdateFlightModesConfig(ctx context.Context, vaID string, config gormModels.JSONB) error {
	result := r.db.WithContext(ctx).
		Model(&VA{}).
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

// GetVAConfigs retrieves all configuration key-value pairs for a VA
func (r *Repository) GetVAConfigs(ctx context.Context, vaID string) ([]VAConfig, error) {
	var configs []VAConfig

	err := r.db.WithContext(ctx).
		Where("va_id = ?", vaID).
		Find(&configs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA configs: %w", err)
	}

	return configs, nil
}

// UpsertVAConfig inserts or updates a configuration key-value pair for a VA
func (r *Repository) UpsertVAConfig(ctx context.Context, vaID, key, value string) error {
	// Use PostgreSQL ON CONFLICT for upsert operation
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
func (r *Repository) GetAllActiveVACallsignConfigs(ctx context.Context) ([]map[string]string, error) {
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

// ====================
// Data Provider Config Operations
// ====================

// IsAirtableConfigured checks if VA has active Airtable credentials
func (r *Repository) IsAirtableConfigured(ctx context.Context, vaID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DataProviderConfig{}).
		Where("va_id = ? AND provider_type = ? AND config_type = ? AND is_active = ?", vaID, "airtable", "credentials", true).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check Airtable configuration: %w", err)
	}

	return count > 0, nil
}

// GetAirtableCredentials retrieves Airtable credentials and sync settings
func (r *Repository) GetAirtableCredentials(ctx context.Context, vaID string) (*ProviderCredentials, error) {
	var config DataProviderConfig
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type = ? AND is_active = ?", vaID, "airtable", "credentials", true).
		First(&config).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No config found
		}
		return nil, fmt.Errorf("failed to get Airtable credentials: %w", err)
	}

	creds, err := ParseCredentialsConfig(config.ConfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials config: %w", err)
	}

	return creds, nil
}

// GetAirtableSchema retrieves a specific schema configuration
func (r *Repository) GetAirtableSchema(ctx context.Context, vaID string, schemaType string) (*SchemaConfig, error) {
	var config DataProviderConfig
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type = ? AND is_active = ?", vaID, "airtable", schemaType, true).
		First(&config).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No schema found
		}
		return nil, fmt.Errorf("failed to get Airtable schema: %w", err)
	}

	schema, err := ParseSchemaConfig(config.ConfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema config: %w", err)
	}

	return schema, nil
}

// GetAirtableSchemas retrieves all schemas for a VA
func (r *Repository) GetAirtableSchemas(ctx context.Context, vaID string) (map[string]*SchemaConfig, error) {
	var configs []DataProviderConfig
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type != ? AND is_active = ?", vaID, "airtable", "credentials", true).
		Find(&configs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get Airtable schemas: %w", err)
	}

	schemas := make(map[string]*SchemaConfig)
	for _, config := range configs {
		schema, err := ParseSchemaConfig(config.ConfigData)
		if err != nil {
			// Log error but continue with other schemas
			continue
		}
		schemas[config.ConfigType] = schema
	}

	return schemas, nil
}

// SaveAirtableCredentials saves or updates Airtable credentials
func (r *Repository) SaveAirtableCredentials(ctx context.Context, vaID string, creds *ProviderCredentials) error {
	configData, err := MarshalCredentialsConfig(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Check if credentials config already exists
	var existingConfig DataProviderConfig
	err = r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type = ?", vaID, "airtable", "credentials").
		First(&existingConfig).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing config: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		// Create new config
		newConfig := DataProviderConfig{
			VAID:             vaID,
			ProviderType:     "airtable",
			ConfigType:       "credentials",
			ConfigData:       configData,
			ConfigVersion:    1,
			IsActive:         true,
			ValidationStatus: ValidationStatusPending,
		}

		if err := r.db.WithContext(ctx).Create(&newConfig).Error; err != nil {
			return fmt.Errorf("failed to create credentials config: %w", err)
		}
	} else {
		// Update existing config
		existingConfig.ConfigData = configData
		existingConfig.ConfigVersion = existingConfig.ConfigVersion + 1
		existingConfig.IsActive = true
		existingConfig.ValidationStatus = ValidationStatusPending

		if err := r.db.WithContext(ctx).Save(&existingConfig).Error; err != nil {
			return fmt.Errorf("failed to update credentials config: %w", err)
		}
	}

	return nil
}

// SaveAirtableSchema saves or updates a specific schema
func (r *Repository) SaveAirtableSchema(ctx context.Context, vaID string, schemaType string, schema *SchemaConfig) error {
	configData, err := MarshalSchemaConfig(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Check if schema config already exists
	var existingConfig DataProviderConfig
	err = r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type = ?", vaID, "airtable", schemaType).
		First(&existingConfig).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing config: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		// Create new config
		newConfig := DataProviderConfig{
			VAID:             vaID,
			ProviderType:     "airtable",
			ConfigType:       schemaType,
			ConfigData:       configData,
			ConfigVersion:    1,
			IsActive:         true,
			ValidationStatus: ValidationStatusPending,
		}

		if err := r.db.WithContext(ctx).Create(&newConfig).Error; err != nil {
			return fmt.Errorf("failed to create schema config: %w", err)
		}
	} else {
		// Update existing config
		existingConfig.ConfigData = configData
		existingConfig.ConfigVersion = existingConfig.ConfigVersion + 1
		existingConfig.IsActive = true
		existingConfig.ValidationStatus = ValidationStatusPending

		if err := r.db.WithContext(ctx).Save(&existingConfig).Error; err != nil {
			return fmt.Errorf("failed to update schema config: %w", err)
		}
	}

	return nil
}
