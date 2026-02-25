package pilots

import (
	"context"
	"errors"
	"fmt"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/va"

	"gorm.io/gorm"
)

// LiveAPIProvider defines the methods needed from the Live API provider
type LiveAPIProvider interface {
	GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error)
	GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error)
}

// Custom errors for registration flow
var (
	ErrIFCUserNotFound    = errors.New("IFC user not found in Infinite Flight system")
	ErrNoRecentFlights    = errors.New("no recent flights found")
	ErrFlightMismatch     = errors.New("last flight does not match logbook")
	ErrRegistrationFailed = errors.New("failed to register user")
)

// RegistrationService handles pilot registration business logic
type RegistrationService struct {
	usersSvc        *users.Service
	vaSvc           *va.Service
	liveAPIProvider LiveAPIProvider
}

// NewRegistrationService creates a new registration service
func NewRegistrationService(
	usersSvc *users.Service,
	vaSvc *va.Service,
	liveAPIProvider LiveAPIProvider,
) *RegistrationService {
	return &RegistrationService{
		usersSvc:        usersSvc,
		vaSvc:           vaSvc,
		liveAPIProvider: liveAPIProvider,
	}
}

// RegisterPilot handles the full pilot registration flow with validation
func (s *RegistrationService) RegisterPilot(
	ctx context.Context,
	discordUserID string,
	discordServerID string,
	ifcId string,
	lastFlight string,
) (*RegisterPilotResponse, error) {
	// Step 1: Validate user exists in Infinite Flight Live API
	logging.Info("Validating IFC user", "ifc_id", ifcId)
	userStatsResp, statusCode, err := s.liveAPIProvider.GetUserByIfcId(ctx, ifcId)
	if err != nil {
		logging.Error("Live API error during user validation", "error", err, "status", statusCode, "ifc_id", ifcId)
		return nil, fmt.Errorf("%w: %v", ErrIFCUserNotFound, err)
	}

	if userStatsResp == nil || len(userStatsResp.Result) == 0 {
		logging.Warn("No user found with IFC ID", "ifc_id", ifcId)
		return nil, fmt.Errorf("%w: no user found with IFC ID: %s", ErrIFCUserNotFound, ifcId)
	}

	ifProfile := userStatsResp.Result[0]
	ifApiID := ifProfile.UserID
	logging.Info("Found IF user", "ifc_id", ifcId, "if_api_id", ifApiID)

	// Step 2: Validate last flight matches user's flight history
	logging.Info("Validating last flight", "if_api_id", ifApiID, "expected_flight", lastFlight)
	recentRoute, err := s.findRecentFlightRoute(ctx, ifApiID)
	if err != nil {
		logging.Error("Failed to fetch flight history", "error", err, "if_api_id", ifApiID)
		return nil, fmt.Errorf("%w: %v", ErrNoRecentFlights, err)
	}

	if recentRoute == "" {
		logging.Warn("No recent flights found", "if_api_id", ifApiID)
		return nil, ErrNoRecentFlights
	}

	if recentRoute != lastFlight {
		logging.Warn("Flight mismatch", "expected", lastFlight, "found", recentRoute, "if_api_id", ifApiID)
		return nil, fmt.Errorf("%w: expected %s, found %s", ErrFlightMismatch, lastFlight, recentRoute)
	}

	logging.Info("Last flight validated", "route", recentRoute)

	// Step 3: Create user in database
	logging.Info("Creating user", "discord_id", discordUserID, "ifc_id", ifcId, "if_api_id", ifApiID)
	err = s.usersSvc.RegisterUser(ctx, discordUserID, ifcId, &ifApiID, true)
	if err != nil {
		logging.Error("Failed to create user", "error", err, "discord_id", discordUserID)
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}

	logging.Info("User created successfully", "discord_id", discordUserID, "ifc_id", ifcId)

	// Step 4: Check if server is registered as a VA
	logging.Info("Checking VA registration status", "discord_server_id", discordServerID)
	vaEntity, err := s.vaSvc.GetByDiscordServerID(ctx, discordServerID)
	isVARegistered := false

	if err == nil && vaEntity != nil {
		isVARegistered = true
		logging.Info("Server is registered as VA", "va_code", vaEntity.Code, "va_name", vaEntity.Name)
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// Log error but don't fail registration
		logging.Warn("Error checking VA status", "error", err, "discord_server_id", discordServerID)
	} else {
		logging.Info("Server is not registered as VA", "discord_server_id", discordServerID)
	}

	// Step 5: Build and return response
	response := &RegisterPilotResponse{
		Success:        true,
		Message:        "Pilot registered successfully",
		IsVARegistered: isVARegistered,
	}

	logging.Info("Pilot registration complete", "discord_id", discordUserID, "is_va_registered", isVARegistered)

	return response, nil
}

// findRecentFlightRoute searches through recent flight pages to find the most recent valid route
// This is adapted from the old implementation in registration_service_v2.go
func (s *RegistrationService) findRecentFlightRoute(ctx context.Context, userID string) (string, error) {
	const maxPages = 3

	for page := 1; page <= maxPages; page++ {
		flightsResp, statusCode, err := s.liveAPIProvider.GetUserFlights(ctx, userID, page)
		if err != nil {
			logging.Error("Failed to fetch flight page", "error", err, "status", statusCode, "page", page, "user_id", userID)
			return "", fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		if flightsResp == nil {
			continue
		}

		// Search for first flight with valid origin and destination
		for _, flight := range flightsResp.Flights {
			if flight.OriginAirport != "" && flight.DestinationAirport != "" {
				route := fmt.Sprintf("%s-%s", flight.OriginAirport, flight.DestinationAirport)
				logging.Info("Found recent flight route", "route", route, "user_id", userID)
				return route, nil
			}
		}
	}

	logging.Warn("No recent flights with valid routes found", "max_pages", maxPages, "user_id", userID)
	return "", fmt.Errorf("no recent flights with valid routes found in %d pages", maxPages)
}
