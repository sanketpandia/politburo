package services

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/entities"
)

// DEPRECATED: This service uses sqlx which has been removed.
// Old implementation moved to va_management_service.go.old
// TODO: Migrate to GORM or remove entirely

type VAManagementService struct {
	// Stub - do not use
}

// Stub constructor for backward compatibility
func NewVAManagementService() *VAManagementService {
	return &VAManagementService{}
}

// SyncUser - DEPRECATED stub
func (s *VAManagementService) SyncUser(ctx context.Context, userID string, callsign string) (string, error) {
	return "", fmt.Errorf("VAManagementService is deprecated, migrate to GORM implementation")
}

// UpdateUserRole - DEPRECATED stub
func (s *VAManagementService) UpdateUserRole(ctx context.Context, userID string, newRole string) (*entities.Membership, error) {
	return nil, fmt.Errorf("VAManagementService is deprecated, migrate to GORM implementation")
}
