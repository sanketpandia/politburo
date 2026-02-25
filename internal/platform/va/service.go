package va

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/cache"

	gormModels "infinite-experiment/politburo/internal/models/gorm"
)

// Service provides core VA business logic operations
type Service struct {
	repo  *Repository
	cache cache.CacheInterface
}

// NewService creates a new VA service
func NewService(repo *Repository) *Service {
	return &Service{
		repo:  repo,
		cache: nil,
	}
}

// NewServiceWithCache creates a new VA service with cache support
func NewServiceWithCache(repo *Repository, cache cache.CacheInterface) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

// GetByID retrieves a VA by its ID
func (s *Service) GetByID(ctx context.Context, vaID string) (*VA, error) {
	return s.repo.GetByID(ctx, vaID)
}

// GetByDiscordServerID retrieves a VA by Discord server ID
func (s *Service) GetByDiscordServerID(ctx context.Context, discordServerID string) (*VA, error) {
	return s.repo.GetByDiscordServerID(ctx, discordServerID)
}

// GetByCode retrieves a VA by its code
func (s *Service) GetByCode(ctx context.Context, code string) (*VA, error) {
	return s.repo.GetByCode(ctx, code)
}

// GetAll retrieves all active VAs
func (s *Service) GetAll(ctx context.Context) ([]VA, error) {
	return s.repo.GetAll(ctx)
}

// Create creates a new VA
func (s *Service) Create(ctx context.Context, va *VA) error {
	return s.repo.Create(ctx, va)
}

// CreateWithAdmin is deprecated - use Create() and memberships.Service.Create() separately
// to avoid circular dependencies between va and memberships packages

// Update updates an existing VA
func (s *Service) Update(ctx context.Context, va *VA) error {
	return s.repo.Update(ctx, va)
}

// UpdateFlightModesConfig updates the flight modes configuration for a VA
func (s *Service) UpdateFlightModesConfig(ctx context.Context, vaID string, config gormModels.JSONB) error {
	return s.repo.UpdateFlightModesConfig(ctx, vaID, config)
}

// UpsertConfig creates or updates a VA configuration key-value pair
func (s *Service) UpsertConfig(ctx context.Context, vaID, key, value string) error {
	return s.repo.UpsertVAConfig(ctx, vaID, key, value)
}

// GetFlightModesConfig retrieves the flight modes configuration for a VA
func (s *Service) GetFlightModesConfig(ctx context.Context, vaID string) (map[string]interface{}, error) {
	return s.repo.GetFlightModesConfig(ctx, vaID)
}

// ValidateAndSaveFlightModesConfig validates the flight modes configuration and saves it to the database
// Validates against the complete schema from PIREP_LOGGING_IMPLEMENTATION_PLAN.md
func (s *Service) ValidateAndSaveFlightModesConfig(ctx context.Context, vaID string, configPayload map[string]interface{}) error {
	// Validate basic structure - must have flight_modes key
	if _, ok := configPayload["flight_modes"]; !ok {
		return fmt.Errorf("configuration must contain 'flight_modes' key")
	}

	flightModes, ok := configPayload["flight_modes"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("'flight_modes' must be an object/map")
	}

	// Validate each mode has required keys and proper structure
	for modeID, modeData := range flightModes {
		modeConfig, ok := modeData.(map[string]interface{})
		if !ok {
			return fmt.Errorf("mode '%s' must be an object/map", modeID)
		}

		// Required keys for a mode
		if _, hasEnabled := modeConfig["enabled"]; !hasEnabled {
			return fmt.Errorf("mode '%s' must have 'enabled' field", modeID)
		}

		if _, hasDisplayName := modeConfig["display_name"]; !hasDisplayName {
			return fmt.Errorf("mode '%s' must have 'display_name' field", modeID)
		}

		// Validate requires_route_selection
		if _, hasRouteSelection := modeConfig["requires_route_selection"]; !hasRouteSelection {
			return fmt.Errorf("mode '%s' must have 'requires_route_selection' field", modeID)
		}

		// Validate fields array
		fields, ok := modeConfig["fields"].([]interface{})
		if !ok {
			return fmt.Errorf("mode '%s': 'fields' must be an array", modeID)
		}

		for idx, fieldData := range fields {
			field, ok := fieldData.(map[string]interface{})
			if !ok {
				return fmt.Errorf("mode '%s': field[%d] must be an object", modeID, idx)
			}

			// Validate field properties
			if _, hasName := field["name"]; !hasName {
				return fmt.Errorf("mode '%s': field[%d] must have 'name'", modeID, idx)
			}

			if _, hasType := field["type"]; !hasType {
				return fmt.Errorf("mode '%s': field[%d] must have 'type' (text, textarea, number)", modeID, idx)
			}

			if _, hasLabel := field["label"]; !hasLabel {
				return fmt.Errorf("mode '%s': field[%d] must have 'label'", modeID, idx)
			}

			if _, hasRequired := field["required"]; !hasRequired {
				return fmt.Errorf("mode '%s': field[%d] must have 'required'", modeID, idx)
			}
		}

		// Validate validations object
		validations, ok := modeConfig["validations"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("mode '%s': 'validations' must be an object", modeID)
		}

		if _, hasAllowAny := validations["allow_any_current_route"]; !hasAllowAny {
			return fmt.Errorf("mode '%s': validations must have 'allow_any_current_route'", modeID)
		}

		if _, hasValidationMode := validations["validation_mode"]; !hasValidationMode {
			return fmt.Errorf("mode '%s': validations must have 'validation_mode' (any, exact_match)", modeID)
		}

		// Validate metadata exists (can be empty but must exist)
		if _, hasMetadata := modeConfig["metadata"]; !hasMetadata {
			return fmt.Errorf("mode '%s' must have 'metadata' object", modeID)
		}

		// auto_route is optional, but if present must be valid
		if autoRoute, hasAutoRoute := modeConfig["auto_route"]; hasAutoRoute && autoRoute != nil {
			if autoRouteObj, ok := autoRoute.(map[string]interface{}); ok {
				if _, hasRouteName := autoRouteObj["route_name"]; !hasRouteName {
					return fmt.Errorf("mode '%s': auto_route must have 'route_name'", modeID)
				}

				if _, hasMultiplier := autoRouteObj["multiplier"]; !hasMultiplier {
					return fmt.Errorf("mode '%s': auto_route must have 'multiplier'", modeID)
				}
			}
		}
	}

	// If validation passes, save to database
	// Convert map[string]interface{} to gormModels.JSONB
	jsonbConfig := gormModels.JSONB(configPayload)

	if err := s.repo.UpdateFlightModesConfig(ctx, vaID, jsonbConfig); err != nil {
		return fmt.Errorf("failed to save flight modes configuration: %w", err)
	}

	return nil
}

// ====================
// Data Provider Config Operations
// ====================

// IsAirtableConfigured checks if VA has active Airtable configuration
func (s *Service) IsAirtableConfigured(ctx context.Context, vaID string) (bool, error) {
	return s.repo.IsAirtableConfigured(ctx, vaID)
}

// GetAirtableCredentials retrieves cached Airtable credentials
func (s *Service) GetAirtableCredentials(ctx context.Context, vaID string) (*ProviderCredentials, error) {
	// Try cache first if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("airtable_creds:%s", vaID)
		if cached, found := s.cache.Get(cacheKey); found {
			if creds, ok := cached.(*ProviderCredentials); ok {
				return creds, nil
			}
		}
	}

	// Cache miss - fetch from repository
	creds, err := s.repo.GetAirtableCredentials(ctx, vaID)
	if err != nil {
		return nil, err
	}

	// Cache for 24 hours if available
	if s.cache != nil && creds != nil {
		cacheKey := fmt.Sprintf("airtable_creds:%s", vaID)
		s.cache.Set(cacheKey, creds, 24*time.Hour)
	}

	return creds, nil
}

// GetAirtableSchema retrieves cached schema configuration
func (s *Service) GetAirtableSchema(ctx context.Context, vaID string, schemaType string) (*SchemaConfig, error) {
	// Try cache first if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("airtable_schema:%s:%s", vaID, schemaType)
		if cached, found := s.cache.Get(cacheKey); found {
			if schema, ok := cached.(*SchemaConfig); ok {
				return schema, nil
			}
		}
	}

	// Cache miss - fetch from repository
	schema, err := s.repo.GetAirtableSchema(ctx, vaID, schemaType)
	if err != nil {
		return nil, err
	}

	// Cache for 24 hours if available
	if s.cache != nil && schema != nil {
		cacheKey := fmt.Sprintf("airtable_schema:%s:%s", vaID, schemaType)
		s.cache.Set(cacheKey, schema, 24*time.Hour)
	}

	return schema, nil
}

// GetAirtableSchemas retrieves all schemas for a VA (not cached individually, but can be cached as a map)
func (s *Service) GetAirtableSchemas(ctx context.Context, vaID string) (map[string]*SchemaConfig, error) {
	// For simplicity, not caching the entire map for now, as individual schemas are cached.
	// If performance becomes an issue, a single cache entry for the map can be added.
	return s.repo.GetAirtableSchemas(ctx, vaID)
}

// SaveAirtableCredentials saves credentials (with cache invalidation)
func (s *Service) SaveAirtableCredentials(ctx context.Context, vaID string, creds *ProviderCredentials) error {
	if err := s.repo.SaveAirtableCredentials(ctx, vaID, creds); err != nil {
		return err
	}

	// Invalidate cache
	if s.cache != nil {
		cacheKey := fmt.Sprintf("airtable_creds:%s", vaID)
		s.cache.Delete(cacheKey)
	}

	return nil
}

// SaveAirtableSchema saves schema (with cache invalidation)
func (s *Service) SaveAirtableSchema(ctx context.Context, vaID string, schemaType string, schema *SchemaConfig) error {
	if err := s.repo.SaveAirtableSchema(ctx, vaID, schemaType, schema); err != nil {
		return err
	}

	// Invalidate cache
	if s.cache != nil {
		cacheKey := fmt.Sprintf("airtable_schema:%s:%s", vaID, schemaType)
		s.cache.Delete(cacheKey)
	}

	return nil
}
