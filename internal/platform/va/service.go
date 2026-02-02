package va

import (
	"context"

	gormModels "infinite-experiment/politburo/internal/models/gorm"
)

// Service provides core VA business logic operations
type Service struct {
	repo *Repository
}

// NewService creates a new VA service
func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
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
