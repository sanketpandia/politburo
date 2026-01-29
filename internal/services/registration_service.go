package services

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/dtos"
)

// DEPRECATED: This service uses sqlx which has been removed.
// Use RegistrationServiceV2 instead.
// Old implementation moved to registration_service.go.old

type RegistrationService struct {
	// Stub - do not use
}

// Stub constructor for backward compatibility
func NewRegistrationService() *RegistrationService {
	return &RegistrationService{}
}

// InitUserRegistration - DEPRECATED stub
func (svc *RegistrationService) InitUserRegistration(ctx context.Context, ifcId string, lastFlight string) (*dtos.InitApiResponse, string, error) {
	return nil, "", fmt.Errorf("RegistrationService is deprecated, use RegistrationServiceV2")
}

// InitServerRegistration - DEPRECATED stub
func (svc *RegistrationService) InitServerRegistration(ctx context.Context, code string, name string) (bool, []dtos.RegistrationStep, error) {
	return false, nil, fmt.Errorf("RegistrationService is deprecated, use RegistrationServiceV2")
}
