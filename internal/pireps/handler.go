package pireps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

// Handler handles PIREP filing endpoints (configuration and submission)
type Handler struct {
	// Repositories
	userRepo *repositories.UserRepositoryGORM
	vaRepo   *repositories.VAGormRepository

	// Services
	cache      cache.CacheInterface
	liveAPI    *common.LiveAPIService
	flightsSvc *flights.Service
	config     *common.VAConfigService
	pirepSvc   *Service
	validator  *FlightModeValidationService
}

// NewHandler creates a new PIREP handlers instance
func NewHandler(
	userRepo *repositories.UserRepositoryGORM,
	vaRepo *repositories.VAGormRepository,
	cache cache.CacheInterface,
	liveAPI *common.LiveAPIService,
	flightsSvc *flights.Service,
	config *common.VAConfigService,
	pirepSvc *Service,
	validator *FlightModeValidationService,
) *Handler {
	return &Handler{
		userRepo:   userRepo,
		vaRepo:     vaRepo,
		cache:      cache,
		liveAPI:    liveAPI,
		flightsSvc: flightsSvc,
		config:     config,
		pirepSvc:   pirepSvc,
		validator:  validator,
	}
}

// GetConfig handles GET /api/v1/pireps/config
// Returns available flight modes and modal field configurations for the user's current flight
func (h *Handler) GetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA configuration with flight modes using Discord Server ID
		vaGorm, err := h.vaRepo.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			httpdto.WriteError(w, initTime, "VA_FETCH_FAILED", "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if vaGorm == nil {
			httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		if _, err := dtos.ParseModeRuntimeEnvelope(vaGorm.FlightModesConfig); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_MODE_CONFIG", "Flight mode configuration is invalid; contact VA admin", http.StatusUnprocessableEntity)
			return
		}

		// Get the user by Discord ID and find their VA role/callsign
		discordID := claims.DiscordUserID()
		user, err := h.userRepo.GetUserWithVAAffiliations(r.Context(), discordID)
		if err != nil || user == nil {
			httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
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
			httpdto.WriteError(w, initTime, "USER_NOT_MEMBER", "User is not a member of this virtual airline", http.StatusForbidden)
			return
		}

		// Get VA config to retrieve prefix and suffix
		prefix, _ := h.config.GetConfigVal(r.Context(), vaGorm.ID, common.ConfigKeyCallsignPrefix)
		suffix, _ := h.config.GetConfigVal(r.Context(), vaGorm.ID, common.ConfigKeyCallsignSuffix)

		logging.Debug("GetPirepConfig: user context",
			"callsign", userCallsign,
			"va_prefix", prefix,
			"va_suffix", suffix,
		)

		// Get VA live flights
		vaFlights, err := h.flightsSvc.GetVALiveFlights(r.Context(), vaGorm.ID)
		if err != nil {
			logging.Warn("GetPirepConfig: failed to fetch VA live flights, continuing with empty set",
				"va_id", vaGorm.ID,
				"error", err,
			)
			vaFlights = &[]dtos.LiveFlight{}
		}

		logging.Debug("GetPirepConfig: fetched live flights", "va_id", vaGorm.ID, "count", len(*vaFlights))

		// Find the user's current flight
		expectedCallsignPattern := prefix + userCallsign + suffix
		logging.Debug("GetPirepConfig: looking for flight",
			"pattern", expectedCallsignPattern,
			"callsign", userCallsign,
		)

		flight := &common.FlightData{
			Callsign:    userCallsign,
			IFCUsername: user.IFCommunityID,
			Aircraft:    "",
			Livery:      "",
			LiveryID:    "",
			Route:       "",
		}

		// Find the user's current flight using unified method
		currentFlight, err := h.flightsSvc.FindUserCurrentFlight(
			r.Context(),
			vaGorm.ID,
			userCallsign,
			prefix,
			suffix,
		)
		if err != nil {
			logging.Debug("GetPirepConfig: no matching live flight found", "error", err)
			httpdto.WriteError(w, initTime, "FLIGHT_NOT_FOUND", "You are not currently flying. Please join a flight before filing a PIREP.", http.StatusNotFound)
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
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// Submit handles POST /api/v1/pireps/submit
// Accepts PIREP submission data and processes it
func (h *Handler) Submit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA configuration
		va, err := h.vaRepo.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			httpdto.WriteError(w, initTime, "VA_FETCH_FAILED", "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if va == nil {
			httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Parse request body
		var submitRequest dtos.PirepSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&submitRequest); err != nil {
			httpdto.WriteError(w, initTime, "BAD_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// Log the incoming PIREP request at debug level — contains user data, not suitable for Info.
		logging.Debug("PIREP submission received",
			"mode", submitRequest.Mode,
			"discord_user_id", claims.DiscordUserID(),
			"va_discord_server_id", vaDiscordServerID,
		)

		// Get user and their current flight for livery mapping
		discordID := claims.DiscordUserID()
		user, err := h.userRepo.GetUserWithVAAffiliations(r.Context(), discordID)
		if err != nil || user == nil {
			httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
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
			httpdto.WriteError(w, initTime, "USER_NOT_MEMBER", "User is not a member of this virtual airline", http.StatusForbidden)
			return
		}

		// Submit PIREP using the injected service
		response, err := h.pirepSvc.SubmitPirep(r.Context(), &submitRequest, va, claims)
		if err != nil {
			httpdto.WriteError(w, initTime, "SUBMIT_FAILED", "Failed to submit PIREP", http.StatusInternalServerError)
			return
		}

		// Return response (success or validation error)
		if response.Success {
			httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
		} else {
			httpdto.WriteError(w, initTime, response.ErrorType, response.ErrorMessage, http.StatusBadRequest)
		}
	}
}

// buildSimplePirepConfigResponse constructs a minimal SimpleConfigResponse
func (h *Handler) buildSimplePirepConfigResponse(
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

	modeEnvelope, err := dtos.ParseModeRuntimeEnvelope(va.FlightModesConfig)
	if err != nil {
		logging.Warn("Invalid v2 flight mode config", "va_id", va.ID, "error", err)
		return response
	}

	for _, modeID := range dtos.SortedModeKeysByDisplayName(modeEnvelope.FlightModes) {
		modeConfig := modeEnvelope.FlightModes[modeID]
		if !modeConfig.Identity.Enabled {
			continue
		}

		requiresRouteSelection := modeConfig.RouteBehavior.RouteSource == dtos.RouteSourceCurrentFPL

		validationResult := &ValidationResult{Valid: true}
		if modeConfig.RouteBehavior.RouteSource == dtos.RouteSourceFixedRoute && modeConfig.RouteBehavior.FixedRoute != nil && flight.Route != "" {
			if flight.Route != modeConfig.RouteBehavior.FixedRoute.RouteName {
				validationResult = &ValidationResult{Valid: false, ErrorMsg: "Current route does not match fixed route for this mode"}
			}
		}

		fields := make([]dtos.FormField, 0, len(modeConfig.PilotInputs))
		for _, input := range modeConfig.PilotInputs {
			fields = append(fields, dtos.FormField{
				Name:     input.Key,
				Type:     input.Type,
				Label:    input.Label,
				Required: input.Required,
			})
		}

		autofillRoute := ""
		if modeConfig.RouteBehavior.RouteSource == dtos.RouteSourceFixedRoute && modeConfig.RouteBehavior.FixedRoute != nil {
			autofillRoute = modeConfig.RouteBehavior.FixedRoute.RouteName
		}

		modeResponse := dtos.SimpleModeResponse{
			ModeID:                 modeID,
			DisplayName:            modeConfig.Identity.DisplayName,
			RequiresRouteSelection: requiresRouteSelection,
			AutofillRoute:          autofillRoute,
			Fields:                 fields,
		}

		if validationResult.Valid {
			modeResponse.Status = dtos.ModeStatusValid
		} else {
			modeResponse.Status = dtos.ModeStatusInvalid
			modeResponse.ErrorReason = validationResult.ErrorMsg
		}

		response.AvailableModes = append(response.AvailableModes, modeResponse)
	}

	return response
}
