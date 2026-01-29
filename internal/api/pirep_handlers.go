package api

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
	"infinite-experiment/politburo/internal/providers"
	"infinite-experiment/politburo/internal/services"
	"log"
	"net/http"
	"time"
)

// PirepHandlers handles PIREP filing endpoints (configuration and submission)
type PirepHandlers struct {
	// Repositories
	userRepo        *repositories.UserRepositoryGORM
	vaRepo          *repositories.VAGormRepository
	pilotRepo       *repositories.PilotATSyncedRepo
	routeRepo       *repositories.RouteATSyncedRepo
	liveryRepo      *repositories.LiveryAirtableMappingRepository
	providerCfgRepo *repositories.DataProviderConfigRepo

	// Services
	cache           common.CacheInterface
	liveAPI         *common.LiveAPIService
	flights         *services.FlightsService
	config          *common.VAConfigService
	provider        *providers.AirtableProvider
	dataProviderCfg *services.DataProviderConfigService
}

// NewPirepHandlers creates a new PIREP handlers instance
func NewPirepHandlers(deps *Dependencies) *PirepHandlers {
	return &PirepHandlers{
		userRepo:        deps.Repo.User,
		vaRepo:          deps.Repo.Va,
		pilotRepo:       deps.Repo.PilotATSynced,
		routeRepo:       deps.Repo.RouteATSynced,
		liveryRepo:      deps.Repo.LiveryAirtableMapping,
		providerCfgRepo: deps.Repo.DataProviderCfg,

		cache:           deps.Services.Cache,
		liveAPI:         deps.Services.Live,
		flights:         deps.Services.Flights,
		config:          deps.Services.Conf,
		provider:        deps.Services.AirtableProvider,
		dataProviderCfg: deps.Services.DataProviderConfig,
	}
}

// GetConfig handles GET /api/v1/pireps/config
// Returns available flight modes and modal field configurations for the user's current flight
func (h *PirepHandlers) GetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA configuration with flight modes using Discord Server ID
		vaGorm, err := h.vaRepo.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if vaGorm == nil {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get the user by Discord ID and find their VA role/callsign
		discordID := claims.DiscordUserID()
		user, err := h.userRepo.GetUserWithVAAffiliations(r.Context(), discordID)
		if err != nil || user == nil {
			common.RespondError(w, initTime, fmt.Errorf("user not found"), "User not found", http.StatusNotFound)
			return
		}

		// Find user's callsign in their VA role
		userCallsign := ""
		for _, role := range user.UserVARoles {
			if role.VAID == vaGorm.ID {
				userCallsign = role.Callsign
				break
			}
		}

		if userCallsign == "" {
			common.RespondError(w, initTime, fmt.Errorf("user not member of va"), "User is not a member of this virtual airline", http.StatusForbidden)
			return
		}

		// Get VA config to retrieve prefix and suffix
		prefix, _ := h.config.GetConfigVal(r.Context(), vaGorm.ID, common.ConfigKeyCallsignPrefix)
		suffix, _ := h.config.GetConfigVal(r.Context(), vaGorm.ID, common.ConfigKeyCallsignSuffix)

		log.Printf("[GetPirepConfig] User callsign: %s, VA prefix: %s, VA suffix: %s", userCallsign, prefix, suffix)

		// Get VA live flights
		vaFlights, err := h.flights.GetVALiveFlights(r.Context(), vaGorm.ID)
		if err != nil {
			log.Printf("[GetPirepConfig] Error fetching VA live flights: %v", err)
			// If we can't get live flights, continue with empty flight data
			vaFlights = &[]dtos.LiveFlight{}
		}

		log.Printf("[GetPirepConfig] Fetched %d live flights for VA %s", len(*vaFlights), vaGorm.ID)

		// Find the user's current flight
		expectedCallsignPattern := prefix + userCallsign + suffix
		log.Printf("[GetPirepConfig] Looking for flight matching pattern: %s (or just number: %s)", expectedCallsignPattern, userCallsign)

		flight := &common.FlightData{
			Callsign:    userCallsign,
			IFCUsername: user.IFCommunityID,
			Aircraft:    "",
			Livery:      "",
			LiveryID:    "",
			Route:       "",
		}

		// Find the user's current flight using unified method
		currentFlight, err := h.flights.FindUserCurrentFlight(
			r.Context(),
			vaGorm.ID,
			userCallsign,
			prefix,
			suffix,
		)
		if err != nil {
			log.Printf("[GetPirepConfig] No matching flight found: %v", err)
			common.RespondError(w, initTime, fmt.Errorf("no live flight found"), "You are not currently flying. Please join a flight before filing a PIREP.", http.StatusNotFound)
			return
		}

		// Map the found flight to FlightData
		if currentFlight != nil {
			flight.Callsign = currentFlight.Callsign
			flight.Aircraft = currentFlight.Aircraft
			flight.Livery = currentFlight.Livery
			flight.LiveryID = currentFlight.LiveryId
			flight.Route = fmt.Sprintf("%s-%s", currentFlight.Origin, currentFlight.Destination)
			flight.Altitude = currentFlight.AltitudeFt
			flight.Speed = currentFlight.SpeedKts
		}

		// Build simplified response
		response := h.buildSimplePirepConfigResponse(r.Context(), vaGorm, flight)
		common.RespondSuccess(w, initTime, "PIREP configuration fetched successfully", response)
	}
}

// Submit handles POST /api/v1/pireps/submit
// Accepts PIREP submission data and processes it
func (h *PirepHandlers) Submit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA configuration
		va, err := h.vaRepo.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if va == nil {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Parse request body
		var submitRequest dtos.PirepSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&submitRequest); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get user and their current flight for livery mapping
		discordID := claims.DiscordUserID()
		user, err := h.userRepo.GetUserWithVAAffiliations(r.Context(), discordID)
		if err != nil || user == nil {
			common.RespondError(w, initTime, fmt.Errorf("user not found"), "User not found", http.StatusNotFound)
			return
		}

		// Get user's callsign for this VA
		userCallsign := ""
		for _, role := range user.UserVARoles {
			if role.VAID == va.ID {
				userCallsign = role.Callsign
				break
			}
		}

		if userCallsign == "" {
			common.RespondError(w, initTime, fmt.Errorf("user not member of va"), "User is not a member of this virtual airline", http.StatusForbidden)
			return
		}

		// Create submission service with all dependencies
		validator := services.NewFlightModeValidationService(h.liveAPI, h.cache)
		submissionService := services.NewPirepSubmissionService(
			h.userRepo,
			h.pilotRepo,
			h.routeRepo,
			h.liveryRepo,
			h.providerCfgRepo,
			h.provider,
			validator,
			h.cache,
			h.flights,
			h.config,
			h.dataProviderCfg,
		)

		// Submit PIREP
		response, err := submissionService.SubmitPirep(r.Context(), &submitRequest, va, claims)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to submit PIREP", http.StatusInternalServerError)
			return
		}

		// Return response (success or validation error)
		if response.Success {
			common.RespondSuccess(w, initTime, response.Message, response)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
		}
	}
}

// buildSimplePirepConfigResponse constructs a minimal SimpleConfigResponse
func (h *PirepHandlers) buildSimplePirepConfigResponse(
	ctx context.Context,
	va *gormModels.VA,
	flight *common.FlightData,
) *dtos.SimpleConfigResponse {
	response := &dtos.SimpleConfigResponse{
		UserInfo: dtos.UserInfo{
			Callsign:            flight.Callsign,
			IFCUsername:         flight.IFCUsername,
			CurrentAircraft:     flight.Aircraft,
			CurrentLivery:       flight.Livery,
			CurrentRoute:        flight.Route,
			CurrentFlightStatus: "in_flight",
			CurrentAltitude:     flight.Altitude,
			CurrentSpeed:        flight.Speed,
		},
		AvailableModes: []dtos.SimpleModeResponse{},
	}

	// Extract flight modes from config
	if va.FlightModesConfig == nil || len(va.FlightModesConfig) == 0 {
		return response
	}

	flightModes, ok := va.FlightModesConfig["flight_modes"].(map[string]interface{})
	if !ok {
		return response
	}

	// Create validator
	validator := services.NewFlightModeValidationService(h.liveAPI, h.cache)

	// Process each configured mode
	for modeID, modeData := range flightModes {
		modeConfig, ok := modeData.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if enabled
		enabled, _ := modeConfig["enabled"].(bool)
		if !enabled {
			continue
		}

		// Get display name
		displayName, _ := modeConfig["display_name"].(string)
		if displayName == "" {
			displayName = modeID
		}

		// Get requires_route_selection
		requiresRouteSelection, _ := modeConfig["requires_route_selection"].(bool)

		// Convert to FlightModeConfig struct for validation
		modeConfigJSON, _ := json.Marshal(modeConfig)
		var flightModeConfig dtos.FlightModeConfig
		if err := json.Unmarshal(modeConfigJSON, &flightModeConfig); err != nil {
			continue
		}

		// Validate mode
		validationResult := validator.ValidateFlightForMode(ctx, flight.Route, &flightModeConfig.Validations)

		modeResponse := dtos.SimpleModeResponse{
			ModeID:                 modeID,
			DisplayName:            displayName,
			RequiresRouteSelection: requiresRouteSelection,
			AutofillRoute:          flightModeConfig.AutofillRoute,
			Fields:                 flightModeConfig.Fields,
		}

		if validationResult.Valid {
			modeResponse.Status = "valid"
		} else {
			modeResponse.Status = "invalid"
			modeResponse.ErrorReason = validationResult.ErrorMsg
		}

		response.AvailableModes = append(response.AvailableModes, modeResponse)
	}

	return response
}
