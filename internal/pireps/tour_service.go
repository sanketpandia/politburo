package pireps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/events"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/sync"
)

// TourPirepResult is the success payload returned by tour PIREP submission.
type TourPirepResult struct {
	PirepID   string `json:"pirep_id"`
	TourID    string `json:"tour_id"`
	LegID     string `json:"leg_id"`
	LegNumber int    `json:"leg_number"`
	Route     string `json:"route"`
}

// TourSubmitError is a structured error returned by TourPirepService.Submit.
// The handler maps Code and StatusCode to the httpdto response.
type TourSubmitError struct {
	Code       string // machine-readable, e.g. "FLIGHT_NOT_FOUND"
	Message    string // human-readable
	StatusCode int    // HTTP status code to respond with
}

func (e *TourSubmitError) Error() string { return e.Message }

// TourPirepService handles tour PIREP submission business logic.
// All dependencies are injected via the constructor (created once at startup).
type TourPirepService struct {
	vaSvc          *platformVA.Service
	vaConfigSvc    *platformVA.ConfigService
	usersRepo      *users.Repository
	eventsSvc      *events.Service
	syncRepo       *sync.Repository
	redisCache     *cache.RedisCacheService
	airtableProvider *providers.AirtableProvider
	configRepo     *repositories.DataProviderConfigRepo
}

// NewTourPirepService creates a new TourPirepService with all dependencies.
func NewTourPirepService(
	vaSvc *platformVA.Service,
	vaConfigSvc *platformVA.ConfigService,
	usersRepo *users.Repository,
	eventsSvc *events.Service,
	syncRepo *sync.Repository,
	redisCache *cache.RedisCacheService,
	airtableProvider *providers.AirtableProvider,
	configRepo *repositories.DataProviderConfigRepo,
) *TourPirepService {
	return &TourPirepService{
		vaSvc:            vaSvc,
		vaConfigSvc:      vaConfigSvc,
		usersRepo:        usersRepo,
		eventsSvc:        eventsSvc,
		syncRepo:         syncRepo,
		redisCache:       redisCache,
		airtableProvider: airtableProvider,
		configRepo:       configRepo,
	}
}

// Submit processes a tour PIREP submission. It matches the user's live flight
// against active tour legs, builds the Airtable payload, and submits it.
//
// Returns (*TourPirepResult, nil) on success or (nil, *TourSubmitError) on failure.
func (s *TourPirepService) Submit(
	ctx context.Context,
	vaDiscordServerID string,
	discordUserID string,
	submitRequest *dtos.PirepSubmitRequest,
) (*TourPirepResult, *TourSubmitError) {

	// ── 1. Resolve VA ────────────────────────────────────────────────
	va, err := s.vaSvc.GetByDiscordServerID(ctx, vaDiscordServerID)
	if err != nil || va == nil {
		logging.Warn("Tour PIREP submit: failed to get VA", "error", err)
		return nil, &TourSubmitError{"VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound}
	}

	// ── 2. Resolve user ──────────────────────────────────────────────
	user, err := s.usersRepo.GetUserWithVAAffiliations(ctx, discordUserID)
	if err != nil || user == nil {
		logging.Warn("Tour PIREP submit: user not found", "discord_id", discordUserID, "error", err)
		return nil, &TourSubmitError{"USER_NOT_FOUND", "User not found", http.StatusNotFound}
	}
	if user.IFCommunityID == "" {
		logging.Warn("Tour PIREP submit: user has no IFCommunityID", "user_id", user.ID)
		return nil, &TourSubmitError{"NO_COMMUNITY_ID", "User has no Infinite Flight community ID", http.StatusBadRequest}
	}

	// ── 3. Get tour flight mode config ───────────────────────────────
	tourFlightMode, _ := s.vaConfigSvc.GetConfigVal(ctx, va.ID, platformVA.ConfigKeyTourFlightMode)
	if tourFlightMode == "" {
		tourFlightMode = "tour"
	}
	logging.Info("Tour PIREP submit: processing",
		"user_id", user.ID, "if_community_id", user.IFCommunityID,
		"va_id", va.ID, "mode", tourFlightMode)

	// ── 4. Match user's live flight ──────────────────────────────────
	matchedFlight, matchErr := s.matchUserFlight(ctx, va.ID, user)
	if matchErr != nil {
		return nil, matchErr
	}

	// ── 5. Get active tour ───────────────────────────────────────────
	activeTour, err := s.eventsSvc.GetActiveMultiLegEvent(ctx, va.ID)
	if err != nil {
		logging.Warn("Tour PIREP submit: failed to get active tour", "error", err)
		return nil, &TourSubmitError{"TOUR_LOOKUP_ERROR", "Failed to get active tour", http.StatusInternalServerError}
	}
	if activeTour == nil {
		logging.Warn("Tour PIREP submit: no active tour found", "va_id", va.ID)
		return nil, &TourSubmitError{"NO_ACTIVE_TOUR", "No active tour found", http.StatusNotFound}
	}

	// ── 6. Match flight route against tour legs ──────────────────────
	flightRoute := ""
	if matchedFlight.Origin != "" && matchedFlight.Destination != "" {
		flightRoute = strings.ToUpper(matchedFlight.Origin + "-" + matchedFlight.Destination)
	}
	if flightRoute == "" {
		logging.Warn("Tour PIREP submit: flight has no route", "flight_id", matchedFlight.FlightID)
		return nil, &TourSubmitError{"NO_ROUTE", "Flight has no route information", http.StatusBadRequest}
	}

	var matchedLeg *events.EventLeg
	for i := range activeTour.Legs {
		legRoute := strings.ToUpper(activeTour.Legs[i].Origin + "-" + activeTour.Legs[i].Destination)
		if legRoute == flightRoute {
			matchedLeg = &activeTour.Legs[i]
			logging.Info("Tour PIREP submit: matched leg",
				"leg_id", matchedLeg.ID, "leg_number", matchedLeg.LegNumber,
				"route", legRoute, "route_at_id", matchedLeg.RouteATID)
			break
		}
	}
	if matchedLeg == nil {
		logging.Warn("Tour PIREP submit: route does not match any tour leg",
			"flight_route", flightRoute, "tour_id", activeTour.ID)
		return nil, &TourSubmitError{
			Code:       "ROUTE_NOT_MATCHED",
			Message:    "Your flight plan start and end do not denote a tour route.",
			StatusCode: http.StatusBadRequest,
		}
	}

	// ── 7. Resolve route Airtable ID ─────────────────────────────────
	routeATID := s.resolveRouteATID(ctx, va.ID, matchedLeg, flightRoute)

	// ── 8. Compute flight time & multiplier ──────────────────────────
	flightTimeSeconds := parseFlightTimeToSeconds(submitRequest.FlightTime)
	multiplier := extractMultiplier(matchedLeg)
	originalSeconds := flightTimeSeconds
	if multiplier > 0 {
		flightTimeSeconds = flightTimeSeconds * multiplier
	}
	logging.Info("Tour PIREP submit: flight duration calculation",
		"multiplier", multiplier, "original_seconds", originalSeconds,
		"adjusted_seconds", flightTimeSeconds,
		"leg_id", matchedLeg.ID, "leg_number", matchedLeg.LegNumber)

	// ── 9. Resolve aircraft / livery names ───────────────────────────
	aircraftResolved := s.vaConfigSvc.ResolveAircraftNameWithMetadata(ctx, va.ID, matchedFlight.LiveryID)
	aircraftName := aircraftResolved.Value
	if aircraftName == "" {
		aircraftName = matchedFlight.AircraftName
	}

	liveryResolved := s.vaConfigSvc.ResolveLiveryNameWithMetadata(ctx, va.ID, matchedFlight.LiveryID)
	liveryName := liveryResolved.Value
	if liveryName == "" {
		liveryName = matchedFlight.LiveryName
	}

	// ── 10. Build enriched remarks ───────────────────────────────────
	enrichedRemarks := s.buildEnrichedRemarks(
		submitRequest, matchedFlight, matchedLeg,
		flightRoute, multiplier,
		aircraftResolved, liveryResolved,
	)

	// ── 11. Load Airtable credentials + pirep schema ─────────────────
	creds, pirepSchema, schemaErr := s.loadAirtableConfig(ctx, va.ID)
	if schemaErr != nil {
		return nil, schemaErr
	}

	// ── 12. Build mapped Airtable fields ─────────────────────────────
	mappedFields := s.buildAirtablePayload(
		pirepSchema, user, va.ID, matchedLeg, activeTour,
		flightTimeSeconds, routeATID,
		aircraftName, liveryName, enrichedRemarks,
		submitRequest,
	)

	// Log payload
	payloadJSON, _ := json.MarshalIndent(map[string]interface{}{"fields": mappedFields}, "", "  ")
	logging.Info("Tour PIREP submit: Airtable payload prepared",
		"tour_id", activeTour.ID, "leg_id", matchedLeg.ID,
		"leg_number", matchedLeg.LegNumber, "flight_id", matchedFlight.FlightID,
		"route", flightRoute, "payload", string(payloadJSON))

	// ── 13. Submit to Airtable ───────────────────────────────────────
	// convertDTOsEntitySchema is reused from pireps/service.go (same package)
	vaSchema := convertDTOsEntitySchema(pirepSchema)
	if vaSchema == nil {
		logging.Error("Tour PIREP submit: failed to convert schema")
		return nil, &TourSubmitError{"SCHEMA_ERROR", "Failed to process schema", http.StatusInternalServerError}
	}

	submitCtx := context.WithValue(ctx, "provider_credentials", creds)
	logging.Info("Tour PIREP submit: submitting to Airtable", "table", vaSchema.TableName)

	pirepID, err := s.airtableProvider.SubmitRecord(submitCtx, vaSchema, mappedFields)
	if err != nil {
		logging.Error("Tour PIREP submit: Airtable submission failed",
			"error", err, "table", vaSchema.TableName)
		return nil, &TourSubmitError{"AIRTABLE_ERROR", "Failed to submit PIREP to Airtable", http.StatusInternalServerError}
	}

	logging.Info("Tour PIREP submit: successfully submitted to Airtable", "pirep_id", pirepID)
	return &TourPirepResult{
		PirepID:   pirepID,
		TourID:    activeTour.ID,
		LegID:     matchedLeg.ID,
		LegNumber: matchedLeg.LegNumber,
		Route:     flightRoute,
	}, nil
}

// ─── Private helpers ─────────────────────────────────────────────────────────

// matchUserFlight finds the user's live flight from cache by IF API ID.
func (s *TourPirepService) matchUserFlight(
	ctx context.Context,
	vaID string,
	user *users.User,
) (*flights.CompleteFlight, *TourSubmitError) {

	flightDTOs, err := flights.GetVALiveFlightsDTOs(s.redisCache, vaID)
	if err != nil {
		logging.Warn("Tour PIREP submit: failed to get VA flights", "error", err)
		return nil, &TourSubmitError{"FLIGHTS_FETCH_ERROR", "Failed to fetch live flights", http.StatusInternalServerError}
	}
	if len(flightDTOs) == 0 {
		logging.Warn("Tour PIREP submit: no flights found for VA", "va_id", vaID)
		return nil, &TourSubmitError{"NO_FLIGHTS", "No live flights found", http.StatusNotFound}
	}

	var userIFApiID string
	if user.IFApiID != nil && *user.IFApiID != "" {
		userIFApiID = *user.IFApiID
	} else {
		logging.Info("Tour PIREP submit: user has no IFApiID, will match by other criteria")
	}

	for _, flightDTO := range flightDTOs {
		flightKey := cache.LiveFlightKey(flightDTO.FlightID)
		flightVal, found := s.redisCache.Get(flightKey)
		if !found {
			continue
		}

		jsonBytes, err := json.Marshal(flightVal)
		if err != nil {
			continue
		}

		var completeFlight flights.CompleteFlight
		if err := json.Unmarshal(jsonBytes, &completeFlight); err != nil {
			continue
		}

		if userIFApiID != "" && completeFlight.UserID == userIFApiID {
			logging.Info("Tour PIREP submit: matched flight by UserID",
				"flight_id", completeFlight.FlightID, "user_id", completeFlight.UserID)
			return &completeFlight, nil
		}
	}

	logging.Warn("Tour PIREP submit: no matching flight found",
		"if_community_id", user.IFCommunityID, "if_api_id", userIFApiID)
	return nil, &TourSubmitError{
		Code:       "FLIGHT_NOT_FOUND",
		Message:    "Could not identify your flight. Please ensure you are currently in the game's server with your VA callsign.",
		StatusCode: http.StatusNotFound,
	}
}

// resolveRouteATID returns the Airtable record ID for the matched route.
func (s *TourPirepService) resolveRouteATID(
	ctx context.Context,
	vaID string,
	matchedLeg *events.EventLeg,
	flightRoute string,
) string {
	if matchedLeg.RouteATID != nil && *matchedLeg.RouteATID != "" {
		logging.Info("Tour PIREP submit: using route Airtable ID from event leg",
			"route_at_id", *matchedLeg.RouteATID, "leg_id", matchedLeg.ID,
			"leg_number", matchedLeg.LegNumber)
		return *matchedLeg.RouteATID
	}

	// Fallback: look up via sync repository
	route, err := s.syncRepo.FindRouteByName(ctx, vaID, flightRoute)
	if err == nil && route != nil {
		logging.Info("Tour PIREP submit: found route Airtable ID from sync repository",
			"route_at_id", route.ATID, "route", flightRoute)
		return route.ATID
	}

	logging.Warn("Tour PIREP submit: no route Airtable ID found",
		"route", flightRoute, "leg_id", matchedLeg.ID)
	return ""
}

// buildEnrichedRemarks generates the pilot remarks including bot-appended metadata.
func (s *TourPirepService) buildEnrichedRemarks(
	submitRequest *dtos.PirepSubmitRequest,
	matchedFlight *flights.CompleteFlight,
	matchedLeg *events.EventLeg,
	flightRoute string,
	multiplier int,
	aircraftResolved platformVA.ResolvedValue,
	liveryResolved platformVA.ResolvedValue,
) string {
	// Calculate max speed / altitude from waypoints
	var maxSpeed, maxAltitude *int
	if len(matchedFlight.Waypoints) > 0 {
		maxSpeedVal := matchedFlight.Waypoints[0].Speed
		maxAltitudeVal := matchedFlight.Waypoints[0].Altitude
		for _, wp := range matchedFlight.Waypoints {
			if wp.Speed > maxSpeedVal {
				maxSpeedVal = wp.Speed
			}
			if wp.Altitude > maxAltitudeVal {
				maxAltitudeVal = wp.Altitude
			}
		}
		maxSpeed = &maxSpeedVal
		maxAltitude = &maxAltitudeVal
	}

	var commentsParts []string
	if submitRequest.FlightTime != "" {
		commentsParts = append(commentsParts, fmt.Sprintf("Actual FT: %s", submitRequest.FlightTime))
	}
	commentsParts = append(commentsParts, fmt.Sprintf("Multiplier: %d", multiplier))
	if flightRoute != "" {
		commentsParts = append(commentsParts, fmt.Sprintf("Actual Route from FPL: %s", flightRoute))
	}
	if maxSpeed != nil {
		commentsParts = append(commentsParts, fmt.Sprintf("Max Speed: %d knots", *maxSpeed))
	}
	if maxAltitude != nil {
		commentsParts = append(commentsParts, fmt.Sprintf("Max Altitude: %d ft", *maxAltitude))
	}
	if aircraftResolved.UsedDefault && aircraftResolved.OriginalValue != "" {
		commentsParts = append(commentsParts, fmt.Sprintf("Aircraft Flown: %s", aircraftResolved.OriginalValue))
	}
	if liveryResolved.UsedDefault && liveryResolved.OriginalValue != "" {
		commentsParts = append(commentsParts, fmt.Sprintf("Livery Flown: %s", liveryResolved.OriginalValue))
	}

	remarks := submitRequest.PilotRemarks
	if len(commentsParts) > 0 {
		commentsText := strings.Join(commentsParts, "\n")
		if remarks != "" {
			remarks = remarks + "\n\n" + commentsText
		} else {
			remarks = commentsText
		}
	}
	return remarks
}

// loadAirtableConfig fetches and validates Airtable credentials and the pirep schema.
func (s *TourPirepService) loadAirtableConfig(
	ctx context.Context,
	vaID string,
) (*platformVA.ProviderCredentials, *dtos.EntitySchema, *TourSubmitError) {

	// ── Credentials ──
	credentialsConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil {
		logging.Error("Tour PIREP submit: failed to get credentials config", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to get Airtable credentials configuration", http.StatusInternalServerError}
	}
	if credentialsConfig == nil {
		logging.Error("Tour PIREP submit: no active credentials config found", "va_id", vaID)
		return nil, nil, &TourSubmitError{"NO_CREDENTIALS", "Airtable credentials are not configured. Please configure Airtable API key and Base ID in the datasource settings.", http.StatusInternalServerError}
	}

	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		logging.Error("Tour PIREP submit: failed to marshal credentials config", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to parse credentials configuration", http.StatusInternalServerError}
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		logging.Error("Tour PIREP submit: failed to parse credentials config", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to parse credentials configuration", http.StatusInternalServerError}
	}
	if credsData.APIKey == "" || credsData.BaseID == "" {
		logging.Error("Tour PIREP submit: Airtable credentials are empty", "va_id", vaID)
		return nil, nil, &TourSubmitError{"NO_CREDENTIALS", "Airtable credentials are not configured. Please configure Airtable API key and Base ID in the datasource settings.", http.StatusInternalServerError}
	}

	creds := &platformVA.ProviderCredentials{
		APIKey: credsData.APIKey,
		BaseID: credsData.BaseID,
		SyncSettings: platformVA.SyncSettings{
			BatchSize:          credsData.SyncSettings.BatchSize,
			RateLimitPerSecond: credsData.SyncSettings.RateLimitPerSecond,
			RetryAttempts:      credsData.SyncSettings.RetryAttempts,
			TimeoutSeconds:     credsData.SyncSettings.TimeoutSeconds,
		},
	}

	// ── PIREP schema ──
	pirepConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "pirep")
	if err != nil {
		logging.Error("Tour PIREP submit: failed to get pirep schema config", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to get PIREP schema configuration", http.StatusInternalServerError}
	}
	if pirepConfig == nil {
		logging.Error("Tour PIREP submit: no pirep schema configured", "va_id", vaID)
		return nil, nil, &TourSubmitError{"NO_SCHEMA", "PIREP schema is not configured. Please configure the PIREP schema in the datasource settings.", http.StatusInternalServerError}
	}

	var pirepSchema dtos.EntitySchema
	schemaBytes, err := json.Marshal(pirepConfig.ConfigData)
	if err != nil {
		logging.Error("Tour PIREP submit: failed to marshal pirep config data", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to parse PIREP schema configuration", http.StatusInternalServerError}
	}
	if err := json.Unmarshal(schemaBytes, &pirepSchema); err != nil {
		logging.Error("Tour PIREP submit: failed to parse pirep schema", "error", err, "va_id", vaID)
		return nil, nil, &TourSubmitError{"CONFIG_ERROR", "Failed to parse PIREP schema configuration", http.StatusInternalServerError}
	}
	if pirepSchema.EntityType == "" {
		pirepSchema.EntityType = "pirep"
	}
	if len(pirepSchema.Fields) == 0 {
		logging.Error("Tour PIREP submit: pirep schema has no fields configured", "va_id", vaID)
		return nil, nil, &TourSubmitError{"NO_SCHEMA", "PIREP schema has no fields configured. Please configure field mappings in the datasource settings.", http.StatusInternalServerError}
	}

	return creds, &pirepSchema, nil
}

// buildAirtablePayload maps domain values to Airtable field names using the pirep schema.
func (s *TourPirepService) buildAirtablePayload(
	pirepSchema *dtos.EntitySchema,
	user *users.User,
	vaID string,
	matchedLeg *events.EventLeg,
	activeTour *events.Event,
	flightTimeSeconds int,
	routeATID string,
	aircraftName string,
	liveryName string,
	enrichedRemarks string,
	submitRequest *dtos.PirepSubmitRequest,
) map[string]interface{} {
	mappedFields := make(map[string]interface{})

	getFieldName := func(internalName string) string {
		fm := pirepSchema.GetFieldMapping(internalName)
		if fm != nil {
			return fm.AirtableName
		}
		return ""
	}

	// Flight time
	if f := getFieldName("flight_time"); f != "" {
		mappedFields[f] = flightTimeSeconds
	}

	// Callsign (Airtable linked record)
	if f := getFieldName("callsign"); f != "" {
		var userVARole *users.UserVARole
		for i := range user.UserVARoles {
			if user.UserVARoles[i].VAID == vaID {
				userVARole = &user.UserVARoles[i]
				break
			}
		}
		if userVARole != nil && userVARole.AirtablePilotID != nil && *userVARole.AirtablePilotID != "" {
			mappedFields[f] = []string{*userVARole.AirtablePilotID}
			logging.Info("Tour PIREP submit: set callsign field",
				"callsign_field", f, "pilot_at_id", *userVARole.AirtablePilotID)
		} else {
			logging.Warn("Tour PIREP submit: user has no Airtable Pilot ID",
				"va_id", vaID, "user_id", user.ID)
		}
	}

	// Aircraft
	if f := getFieldName("aircraft"); f != "" && aircraftName != "" {
		mappedFields[f] = aircraftName
	}

	// Airline (from livery)
	if f := getFieldName("airline"); f != "" && liveryName != "" {
		mappedFields[f] = liveryName
	}

	// Flight mode
	if f := getFieldName("flight_mode"); f != "" {
		flightModeValue := "World Tour 10"
		if activeTour.FlightMode != nil && *activeTour.FlightMode != "" {
			flightModeValue = *activeTour.FlightMode
		}
		mappedFields[f] = flightModeValue
	}

	// Route (Airtable linked record)
	if f := getFieldName("route_at_id"); f != "" && routeATID != "" {
		mappedFields[f] = []string{routeATID}
		logging.Info("Tour PIREP submit: set route field",
			"route_field", f, "route_at_id", routeATID)
	}

	// IFC username
	if f := getFieldName("ifc_username"); f != "" && user.IFCommunityID != "" {
		mappedFields[f] = user.IFCommunityID
	}

	// Pilot remarks (enriched)
	if f := getFieldName("pilot_remarks"); f != "" && enrichedRemarks != "" {
		mappedFields[f] = enrichedRemarks
	}

	// Mode-specific optional fields
	if submitRequest.FuelKg != nil {
		if f := getFieldName("fuel_kg"); f != "" {
			mappedFields[f] = *submitRequest.FuelKg
		}
	}
	if submitRequest.CargoKg != nil {
		if f := getFieldName("cargo_kg"); f != "" {
			mappedFields[f] = *submitRequest.CargoKg
		}
	}
	if submitRequest.Passengers != nil {
		if f := getFieldName("passengers"); f != "" {
			mappedFields[f] = *submitRequest.Passengers
		}
	}

	return mappedFields
}

// ─── Package-private utilities ───────────────────────────────────────────────

// getAdditionalDataKeys extracts keys from AdditionalData map for logging.
func getAdditionalDataKeys(additionalData map[string]interface{}) []string {
	if additionalData == nil {
		return []string{}
	}
	keys := make([]string, 0, len(additionalData))
	for k := range additionalData {
		keys = append(keys, k)
	}
	return keys
}

// parseFlightTimeToSeconds converts "HH:MM" to total seconds.
func parseFlightTimeToSeconds(ft string) int {
	parts := strings.Split(ft, ":")
	if len(parts) != 2 {
		return 0
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	return (hours * 3600) + (minutes * 60)
}

// extractMultiplier reads the multiplier from an event leg's AdditionalData.
func extractMultiplier(leg *events.EventLeg) int {
	multiplier := 1
	if leg.AdditionalData == nil {
		logging.Info("Tour PIREP submit: additional_data is nil, using default multiplier 1",
			"leg_id", leg.ID, "leg_number", leg.LegNumber)
		return multiplier
	}

	logging.Debug("Tour PIREP submit: checking additional_data for multiplier",
		"leg_id", leg.ID, "leg_number", leg.LegNumber,
		"additional_data_keys", getAdditionalDataKeys(leg.AdditionalData))

	multValue, exists := leg.AdditionalData["multiplier"]
	if !exists {
		logging.Info("Tour PIREP submit: multiplier key not found in additional_data, using default 1",
			"leg_id", leg.ID, "leg_number", leg.LegNumber,
			"available_keys", getAdditionalDataKeys(leg.AdditionalData))
		return multiplier
	}

	logging.Debug("Tour PIREP submit: found multiplier value",
		"value", multValue, "type", fmt.Sprintf("%T", multValue))

	switch v := multValue.(type) {
	case int:
		multiplier = v
	case int64:
		multiplier = int(v)
	case float64:
		multiplier = int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			multiplier = parsed
		} else {
			logging.Warn("Tour PIREP submit: failed to parse multiplier string",
				"value", v, "error", err)
		}
	default:
		logging.Warn("Tour PIREP submit: multiplier has unexpected type",
			"value", multValue, "type", fmt.Sprintf("%T", multValue))
	}

	logging.Info("Tour PIREP submit: extracted multiplier from event leg",
		"multiplier", multiplier, "leg_id", leg.ID, "leg_number", leg.LegNumber)
	return multiplier
}
