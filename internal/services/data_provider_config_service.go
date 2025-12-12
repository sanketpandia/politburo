package services

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models"
	"infinite-experiment/politburo/internal/models/dtos"
	"log"
	"time"

	"github.com/lib/pq"
)

type DataProviderConfigService struct {
	configRepo *repositories.DataProviderConfigRepo
	cache      common.CacheInterface
}

func NewDataProviderConfigService(configRepo *repositories.DataProviderConfigRepo, cache common.CacheInterface) *DataProviderConfigService {
	return &DataProviderConfigService{
		configRepo: configRepo,
		cache:      cache,
	}
}

// SaveOrUpdateConfig saves or updates a data provider config for a VA
func (s *DataProviderConfigService) SaveOrUpdateConfig(ctx context.Context, vaID string, req *dtos.SaveProviderConfigRequest, userID string) (*DataProviderConfigResponse, error) {
	// Validate request
	if err := s.validateConfigRequest(req); err != nil {
		return nil, &ConfigError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: err.Error(),
		}
	}

	// Marshal config data to JSONB
	configDataJSONB, err := repositories.MarshalConfigData(&req.ConfigData)
	if err != nil {
		return nil, &ConfigError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal config data",
			Err:     err,
		}
	}

	// Check if config already exists for this VA and provider type
	existingConfig, err := s.configRepo.GetActiveConfig(ctx, vaID, req.ProviderType)
	if err != nil {
		return nil, &ConfigError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to check existing config",
			Err:     err,
		}
	}

	var config *models.DataProviderConfig

	if existingConfig != nil {
		// UPDATE existing config
		existingConfig.ConfigData = configDataJSONB
		existingConfig.ConfigVersion = existingConfig.ConfigVersion + 1
		existingConfig.IsActive = req.IsActive
		existingConfig.ValidationStatus = models.ValidationStatusPending // Reset validation status
		existingConfig.LastValidatedAt = nil
		existingConfig.ValidationErrors = nil
		existingConfig.UpdatedBy = &userID

		if err := s.configRepo.UpdateConfig(ctx, existingConfig); err != nil {
			return nil, &ConfigError{
				Code:    constants.ErrCodeNetworkError,
				Message: "Failed to update config",
				Err:     err,
			}
		}

		// Evict from cache to force refresh on next fetch
		cacheKey := fmt.Sprintf("provider_config:%s:%s", vaID, req.ProviderType)
		s.cache.Delete(cacheKey)
		log.Printf("[DataProviderConfigService] Evicted cache for updated config: VA=%s, Provider=%s", vaID, req.ProviderType)

		config = existingConfig

	} else {
		// CREATE new config
		newConfig := &models.DataProviderConfig{
			VAID:             vaID,
			ProviderType:     req.ProviderType,
			ConfigData:       configDataJSONB,
			ConfigVersion:    1,
			IsActive:         req.IsActive,
			ValidationStatus: models.ValidationStatusPending,
			FeaturesEnabled:  pq.StringArray{}, // Empty initially
			CreatedBy:        &userID,
			UpdatedBy:        &userID,
		}

		if err := s.configRepo.CreateConfig(ctx, newConfig); err != nil {
			return nil, &ConfigError{
				Code:    constants.ErrCodeNetworkError,
				Message: "Failed to create config",
				Err:     err,
			}
		}

		// Evict from cache to force refresh on next fetch
		cacheKey := fmt.Sprintf("provider_config:%s:%s", vaID, req.ProviderType)
		s.cache.Delete(cacheKey)
		log.Printf("[DataProviderConfigService] Evicted cache for new config: VA=%s, Provider=%s", vaID, req.ProviderType)

		config = newConfig
	}

	// Build response
	response := &DataProviderConfigResponse{
		ID:               config.ID,
		ProviderType:     config.ProviderType,
		ConfigVersion:    config.ConfigVersion,
		IsActive:         config.IsActive,
		ValidationStatus: string(config.ValidationStatus),
		FeaturesEnabled:  []string(config.FeaturesEnabled),
		CreatedAt:        config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        config.UpdatedAt.Format(time.RFC3339),
	}

	// Add parsed config data
	parsedConfig, err := repositories.ParseConfigData(config.ConfigData)
	if err == nil {
		response.ConfigData = parsedConfig
	}

	return response, nil
}

// validateConfigRequest validates the config request
func (s *DataProviderConfigService) validateConfigRequest(req *dtos.SaveProviderConfigRequest) error {
	if req.ProviderType == "" {
		return fmt.Errorf("provider_type is required")
	}

	if req.ConfigData.Version == "" {
		return fmt.Errorf("config version is required")
	}

	if req.ConfigData.Provider == "" {
		return fmt.Errorf("provider is required in config_data")
	}

	// Validate credentials for Airtable
	if req.ProviderType == "airtable" {
		if req.ConfigData.Credentials.APIKey == "" {
			return fmt.Errorf("api_key is required for Airtable")
		}
		if req.ConfigData.Credentials.BaseID == "" {
			return fmt.Errorf("base_id is required for Airtable")
		}
	}

	// Validate schemas if provided (empty schemas allowed for credentials-only saves)
	if len(req.ConfigData.Schemas) > 0 {
		// Validate each schema
		for i, schema := range req.ConfigData.Schemas {
			if schema.EntityType == "" {
				return fmt.Errorf("schema[%d]: entity_type is required", i)
			}
			if schema.TableName == "" {
				return fmt.Errorf("schema[%d]: table_name is required", i)
			}
			if len(schema.Fields) == 0 {
				return fmt.Errorf("schema[%d]: at least one field mapping is required", i)
			}

			// Validate field mappings
			for j, field := range schema.Fields {
				if field.InternalName == "" {
					return fmt.Errorf("schema[%d].fields[%d]: internal_name is required", i, j)
				}
				if field.AirtableName == "" {
					return fmt.Errorf("schema[%d].fields[%d]: airtable_name is required", i, j)
				}
				if field.DataType == "" {
					return fmt.Errorf("schema[%d].fields[%d]: data_type is required", i, j)
				}

				// Validate data type is one of the allowed types
				allowedTypes := map[string]bool{
					"string": true, "int": true, "float": true, "boolean": true, "date": true,
				}
				if !allowedTypes[field.DataType] {
					return fmt.Errorf("schema[%d].fields[%d]: invalid data_type '%s' (allowed: string, int, float, boolean, date)", i, j, field.DataType)
				}

				// Validate display_format if provided
				if field.DisplayFormat != nil {
					allowedFormats := map[string]bool{
						"duration": true, "date": true, "datetime": true, "number": true,
					}
					if !allowedFormats[*field.DisplayFormat] {
						return fmt.Errorf("schema[%d].fields[%d]: invalid display_format '%s' (allowed: duration, date, datetime, number)", i, j, *field.DisplayFormat)
					}
				}

				// Note: display_name and is_user_visible are optional and don't require validation
				// If is_user_visible is not set, it defaults to false (field won't be shown in user APIs)
			}
		}
	}

	return nil
}

// DataProviderConfigResponse is the response for config operations
type DataProviderConfigResponse struct {
	ID               string                   `json:"id"`
	ProviderType     string                   `json:"provider_type"`
	ConfigVersion    int                      `json:"config_version"`
	IsActive         bool                     `json:"is_active"`
	ValidationStatus string                   `json:"validation_status"`
	FeaturesEnabled  []string                 `json:"features_enabled"`
	ConfigData       *dtos.ProviderConfigData `json:"config_data,omitempty"`
	CreatedAt        string                   `json:"created_at"`
	UpdatedAt        string                   `json:"updated_at"`
}

// ConfigError represents a configuration error
type ConfigError struct {
	Code    string
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// GetActiveConfigCached fetches the active config for a VA with caching (24-hour TTL)
func (s *DataProviderConfigService) GetActiveConfigCached(ctx context.Context, vaID, providerType string) (*dtos.ProviderConfigData, error) {
	// Build cache key
	cacheKey := fmt.Sprintf("provider_config:%s:%s", vaID, providerType)

	// Check cache first
	if cached, found := s.cache.Get(cacheKey); found {
		if configData, ok := cached.(*dtos.ProviderConfigData); ok {
			configJSON, _ := json.MarshalIndent(configData, "", "  ")
			log.Printf("[DataProviderConfigService] Cache hit for provider config: VA=%s, Provider=%s\nConfig:\n%s", vaID, providerType, string(configJSON))
			return configData, nil
		}
	}

	// Cache miss - fetch from database
	log.Printf("[DataProviderConfigService] Cache miss for provider config: VA=%s, Provider=%s, fetching from database", vaID, providerType)

	providerConfig, err := s.configRepo.GetActiveConfig(ctx, vaID, providerType)
	if err != nil || providerConfig == nil {
		if err != nil {
			log.Printf("[DataProviderConfigService] Error fetching provider config: %v", err)
		}
		return nil, err
	}

	// Parse config data
	configData, err := repositories.ParseConfigData(providerConfig.ConfigData)
	if err != nil {
		log.Printf("[DataProviderConfigService] Error parsing provider config: %v", err)
		return nil, err
	}

	// Cache for 24 hours
	s.cache.Set(cacheKey, configData, 24*time.Hour)
	configJSON, _ := json.MarshalIndent(configData, "", "  ")
	log.Printf("[DataProviderConfigService] Cached provider config: VA=%s, Provider=%s, TTL=24h\nConfig:\n%s", vaID, providerType, string(configJSON))

	return configData, nil
}
