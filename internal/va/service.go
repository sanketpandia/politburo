package va

import (
	"context"

	"infinite-experiment/politburo/internal/models/gorm"
)

// Service provides core VA business logic operations
type Service struct {
	repo     *Repository
	roleRepo *RoleRepository
}

// NewService creates a new VA service
func NewService(repo *Repository, roleRepo *RoleRepository) *Service {
	return &Service{
		repo:     repo,
		roleRepo: roleRepo,
	}
}

// GetByID retrieves a VA by its ID
func (s *Service) GetByID(ctx context.Context, vaID string) (*gorm.VA, error) {
	return s.repo.GetByID(ctx, vaID)
}

// GetByDiscordServerID retrieves a VA by Discord server ID
func (s *Service) GetByDiscordServerID(ctx context.Context, discordServerID string) (*gorm.VA, error) {
	return s.repo.GetByDiscordServerID(ctx, discordServerID)
}

// GetByCode retrieves a VA by its code
func (s *Service) GetByCode(ctx context.Context, code string) (*gorm.VA, error) {
	return s.repo.GetByCode(ctx, code)
}

// GetAll retrieves all active VAs
func (s *Service) GetAll(ctx context.Context) ([]gorm.VA, error) {
	return s.repo.GetAll(ctx)
}

// Create creates a new VA
func (s *Service) Create(ctx context.Context, va *gorm.VA) error {
	return s.repo.Create(ctx, va)
}

// CreateWithAdmin creates a new VA and assigns an admin user
func (s *Service) CreateWithAdmin(ctx context.Context, name, code, discordID string, isActive bool, adminUserID string) (*gorm.VA, *gorm.UserVARole, error) {
	return s.repo.CreateWithAdmin(ctx, name, code, discordID, isActive, adminUserID)
}

// Update updates an existing VA
func (s *Service) Update(ctx context.Context, va *gorm.VA) error {
	return s.repo.Update(ctx, va)
}

// UpdateFlightModesConfig updates the flight modes configuration for a VA
func (s *Service) UpdateFlightModesConfig(ctx context.Context, vaID string, config gorm.JSONB) error {
	return s.repo.UpdateFlightModesConfig(ctx, vaID, config)
}

// GetUserRole retrieves a user's role in a specific VA
func (s *Service) GetUserRole(ctx context.Context, userID, vaID string) (*gorm.UserVARole, error) {
	return s.roleRepo.GetByUserAndVA(ctx, userID, vaID)
}

// GetAllUserVAs retrieves all VAs a user belongs to
func (s *Service) GetAllUserVAs(ctx context.Context, userID string) ([]gorm.UserVARole, error) {
	return s.roleRepo.GetAllByUserID(ctx, userID)
}

// GetAllVAUsers retrieves all users in a specific VA
func (s *Service) GetAllVAUsers(ctx context.Context, vaID string) ([]gorm.UserVARole, error) {
	return s.roleRepo.GetAllByVAID(ctx, vaID)
}

// CreateUserRole creates a new user role in a VA
func (s *Service) CreateUserRole(ctx context.Context, role *gorm.UserVARole) error {
	return s.roleRepo.Create(ctx, role)
}

// UpdateUserRole updates a user's role in a VA
func (s *Service) UpdateUserRole(ctx context.Context, role *gorm.UserVARole) error {
	return s.roleRepo.Update(ctx, role)
}

// DeleteUserRole removes a user from a VA (soft delete)
func (s *Service) DeleteUserRole(ctx context.Context, roleID string) error {
	return s.roleRepo.Delete(ctx, roleID)
}
