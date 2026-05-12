package pireps

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/models/dtos"
)

// FlightModeValidationService validates if a current flight is eligible for a specific mode
type FlightModeValidationService struct {
	liveAPI *common.LiveAPIService
	cache   cache.CacheInterface
}

// ValidationResult represents the result of a flight mode validation
type ValidationResult struct {
	Valid    bool
	ErrorMsg string
}

// NewFlightModeValidationService creates a new flight mode validation service
func NewFlightModeValidationService(
	liveAPI *common.LiveAPIService,
	cache cache.CacheInterface,
) *FlightModeValidationService {
	return &FlightModeValidationService{
		liveAPI: liveAPI,
		cache:   cache,
	}
}

// ValidateFlightForMode validates if a current flight qualifies for a specific mode
func (s *FlightModeValidationService) ValidateFlightForMode(
	ctx context.Context,
	currentRoute string,
	config *dtos.ValidationConfig,
) *ValidationResult {
	// If allow_any_current_route is true, always valid
	if config.AllowAnyCurrentRoute {
		return &ValidationResult{Valid: true}
	}

	// If allowed_routes is empty, always valid
	if len(config.AllowedRoutes) == 0 {
		return &ValidationResult{Valid: true}
	}

	// Check validation mode
	switch config.ValidationMode {
	case "exact_match":
		return s.validateExactMatch(currentRoute, config.AllowedRoutes)
	case "any":
		return &ValidationResult{Valid: true}
	default:
		// Default to exact match if unknown mode
		return s.validateExactMatch(currentRoute, config.AllowedRoutes)
	}
}

// validateExactMatch checks if current route is in allowed routes
func (s *FlightModeValidationService) validateExactMatch(currentRoute string, allowedRoutes []string) *ValidationResult {
	for _, route := range allowedRoutes {
		if route == currentRoute {
			return &ValidationResult{Valid: true}
		}
	}

	msg := fmt.Sprintf("Current route %s not in allowed routes for this mode", currentRoute)
	logging.Info("Flight mode validation failed",
		"rule", "exact_match",
		"current_route", currentRoute,
	)
	return &ValidationResult{Valid: false, ErrorMsg: msg}
}
