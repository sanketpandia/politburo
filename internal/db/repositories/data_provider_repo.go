package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/models"
	"infinite-experiment/politburo/internal/models/dtos"

	"gorm.io/gorm"
)

type DataProviderConfigRepo struct {
	db *gorm.DB
}

func NewDataProviderConfigRepo(db *gorm.DB) *DataProviderConfigRepo {
	return &DataProviderConfigRepo{db: db}
}

// GetActiveConfig fetches the active config for a VA by provider type
func (r *DataProviderConfigRepo) GetActiveConfig(ctx context.Context, vaID, providerType string) (*models.DataProviderConfig, error) {
	var config models.DataProviderConfig

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND is_active = ?", vaID, providerType, true).
		First(&config).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No config found
		}
		return nil, fmt.Errorf("failed to get active config: %w", err)
	}

	return &config, nil
}

// GetActiveConfigByType fetches the active config for a VA by provider type and config type
func (r *DataProviderConfigRepo) GetActiveConfigByType(ctx context.Context, vaID, providerType, configType string) (*models.DataProviderConfig, error) {
	var config models.DataProviderConfig

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND provider_type = ? AND config_type = ? AND is_active = ?", vaID, providerType, configType, true).
		First(&config).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No config found
		}
		return nil, fmt.Errorf("failed to get active config by type: %w", err)
	}

	return &config, nil
}

// GetConfigByID fetches a config by its ID
func (r *DataProviderConfigRepo) GetConfigByID(ctx context.Context, configID string) (*models.DataProviderConfig, error) {
	var config models.DataProviderConfig

	err := r.db.WithContext(ctx).
		Where("id = ?", configID).
		First(&config).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get config by ID: %w", err)
	}

	return &config, nil
}

// GetConfigsByVA fetches all configs for a VA
func (r *DataProviderConfigRepo) GetConfigsByVA(ctx context.Context, vaID string) ([]models.DataProviderConfig, error) {
	var configs []models.DataProviderConfig

	err := r.db.WithContext(ctx).
		Where("va_id = ?", vaID).
		Order("created_at DESC").
		Find(&configs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get configs for VA: %w", err)
	}

	return configs, nil
}

// CreateConfig creates a new config
func (r *DataProviderConfigRepo) CreateConfig(ctx context.Context, config *models.DataProviderConfig) error {
	err := r.db.WithContext(ctx).Create(config).Error
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}
	return nil
}

// UpdateConfig updates an existing config
func (r *DataProviderConfigRepo) UpdateConfig(ctx context.Context, config *models.DataProviderConfig) error {
	err := r.db.WithContext(ctx).Save(config).Error
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}
	return nil
}

// DeleteConfig deletes a config
func (r *DataProviderConfigRepo) DeleteConfig(ctx context.Context, configID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", configID).
		Delete(&models.DataProviderConfig{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete config: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("config not found")
	}

	return nil
}

// SaveValidationHistory saves a validation result to history
func (r *DataProviderConfigRepo) SaveValidationHistory(ctx context.Context, history *models.ProviderValidationHistory) error {
	err := r.db.WithContext(ctx).Create(history).Error
	if err != nil {
		return fmt.Errorf("failed to save validation history: %w", err)
	}
	return nil
}

// Helper methods to parse JSONB fields

// ParseConfigData parses the config_data JSONB into ProviderConfigData
func ParseConfigData(configData models.JSONB) (*dtos.ProviderConfigData, error) {
	bytes, err := json.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	var config dtos.ProviderConfigData
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config data: %w", err)
	}
	return &config, nil
}

// MarshalConfigData marshals ProviderConfigData to JSONB
func MarshalConfigData(config *dtos.ProviderConfigData) (models.JSONB, error) {
	bytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	var jsonb models.JSONB
	if err := json.Unmarshal(bytes, &jsonb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to JSONB: %w", err)
	}

	return jsonb, nil
}

// ParseValidationErrors parses validation_errors JSONB
func ParseValidationErrors(errorData models.JSONB) ([]dtos.ValidationError, error) {
	if errorData == nil || len(errorData) == 0 {
		return nil, nil
	}

	bytes, err := json.Marshal(errorData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation errors: %w", err)
	}

	var errors []dtos.ValidationError
	if err := json.Unmarshal(bytes, &errors); err != nil {
		return nil, fmt.Errorf("failed to parse validation errors: %w", err)
	}
	return errors, nil
}

// MarshalValidationErrors marshals validation errors to JSONB
func MarshalValidationErrors(errors []dtos.ValidationError) (models.JSONB, error) {
	if errors == nil || len(errors) == 0 {
		return nil, nil
	}

	bytes, err := json.Marshal(errors)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation errors: %w", err)
	}

	var jsonb models.JSONB
	if err := json.Unmarshal(bytes, &jsonb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to JSONB: %w", err)
	}

	return jsonb, nil
}
