package services

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
)

// FlightModesConfigService handles flight modes configuration management
type FlightModesConfigService struct {
	vaGormRepo *repositories.VAGormRepository
}

// NewFlightModesConfigService creates a new flight modes config service
func NewFlightModesConfigService(vaGormRepo *repositories.VAGormRepository) *FlightModesConfigService {
	return &FlightModesConfigService{
		vaGormRepo: vaGormRepo,
	}
}

// ValidateAndSaveConfig validates the flight modes configuration and saves it to the database
// Validates against the complete schema from PIREP_LOGGING_IMPLEMENTATION_PLAN.md
func (s *FlightModesConfigService) ValidateAndSaveConfig(ctx context.Context, vaID string, configPayload map[string]interface{}) error {
	if _, err := dtos.ParseModeRuntimeEnvelope(configPayload); err != nil {
		return fmt.Errorf("invalid v2 flight mode config: %w", err)
	}

	// If validation passes, save to database
	// Convert map[string]interface{} to gormModels.JSONB
	jsonbConfig := gormModels.JSONB(configPayload)

	if err := s.vaGormRepo.UpdateFlightModesConfig(ctx, vaID, jsonbConfig); err != nil {
		return fmt.Errorf("failed to save flight modes configuration: %w", err)
	}

	return nil
}

// GetConfig retrieves the flight modes configuration for a VA
func (s *FlightModesConfigService) GetConfig(ctx context.Context, vaID string) (map[string]interface{}, error) {
	va, err := s.vaGormRepo.GetByID(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA: %w", err)
	}

	if va == nil {
		return nil, fmt.Errorf("VA not found with ID: %s", vaID)
	}

	if va.FlightModesConfig == nil {
		return map[string]interface{}{}, nil
	}

	return va.FlightModesConfig, nil
}
