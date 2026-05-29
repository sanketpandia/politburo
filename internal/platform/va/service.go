package va

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/models/dtos"

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
	if _, err := dtos.ParseModeRuntimeEnvelope(configPayload); err != nil {
		return fmt.Errorf("invalid v2 flight mode config: %w", err)
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
