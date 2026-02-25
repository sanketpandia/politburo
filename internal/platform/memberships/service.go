package memberships

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrUserNotFound is returned when a user is not found
var ErrUserNotFound = errors.New("user not found")

// Service provides membership business logic operations at platform level
type Service struct {
	repo *Repository
}

// NewService creates a new membership service
func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// GetUserStatusByUserID retrieves user status with all VA affiliations
// Platform-level method - provides raw data access only
func (s *Service) GetUserStatusByUserID(ctx context.Context, userID string, vaID string) (*UserStatusResult, error) {
	result, err := s.repo.GetUserStatusByUserID(ctx, userID, vaID)
	if err != nil {
		// Convert GORM-specific errors to domain errors
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return result, nil
}

/*
// GetByID retrieves a membership by its ID
func (s *Service) GetByID(ctx context.Context, id string) (*UserVARole, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByUserAndVA retrieves a user's membership in a specific VA
func (s *Service) GetByUserAndVA(ctx context.Context, userID, vaID string) (*UserVARole, error) {
	return s.repo.GetByUserAndVA(ctx, userID, vaID)
}

// GetAllByUserID retrieves all memberships for a specific user
func (s *Service) GetAllByUserID(ctx context.Context, userID string) ([]UserVARole, error) {
	return s.repo.GetAllByUserID(ctx, userID)
}

// GetAllByVAID retrieves all memberships in a specific VA
func (s *Service) GetAllByVAID(ctx context.Context, vaID string) ([]UserVARole, error) {
	return s.repo.GetAllByVAID(ctx, vaID)
}

// Create creates a new membership
func (s *Service) Create(ctx context.Context, membership *UserVARole) error {
	return s.repo.Create(ctx, membership)
}

// Update updates an existing membership
func (s *Service) Update(ctx context.Context, membership *UserVARole) error {
	return s.repo.Update(ctx, membership)
}

// Delete removes a membership (soft delete)
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// GetByDiscordIDs retrieves a membership by Discord user and server IDs
func (s *Service) GetByDiscordIDs(ctx context.Context, discordUserID, discordServerID string) (*UserVARole, error) {
	return s.repo.GetByDiscordIDs(ctx, discordUserID, discordServerID)
}

// ValidateCallsign checks if a callsign is available in a VA
func (s *Service) ValidateCallsign(ctx context.Context, callsign, vaID, excludeID string) (*UserVARole, error) {
	return s.repo.GetByCallsignAndVAID(ctx, callsign, vaID, excludeID)
}
*/
