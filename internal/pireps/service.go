package pireps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/sync"
	"log"
)

// DataProviderConfigServiceInterface interface to avoid import cycle with services package
type DataProviderConfigServiceInterface interface {
	GetActiveConfigCached(ctx context.Context, vaID string, providerType string) (*dtos.ProviderConfigData, error)
}

// Service handles PIREP submission logic
type Service struct {
	userRepo                  *repositories.UserRepositoryGORM
	pilotRepo                 *pilots.Repository
	syncRepo                  *sync.Repository
	liveryMappingRepo         *aircraft.Repository
	dataProviderConfigRepo    *repositories.DataProviderConfigRepo
	airtableProvider          providers.DataProvider
	validator                 *FlightModeValidationService
	cache                     cache.CacheInterface
	flightsService            *flights.Service
	configService             *common.VAConfigService
	dataProviderConfigService DataProviderConfigServiceInterface
}

// NewService creates a new PIREP service with dependencies
func NewService(
	userRepo *repositories.UserRepositoryGORM,
	pilotRepo *pilots.Repository,
	syncRepo *sync.Repository,
	liveryMappingRepo *aircraft.Repository,
	dataProviderConfigRepo *repositories.DataProviderConfigRepo,
	airtableProvider providers.DataProvider,
	validator *FlightModeValidationService,
	cache cache.CacheInterface,
	flightsService *flights.Service,
	configService *common.VAConfigService,
	dataProviderConfigService DataProviderConfigServiceInterface,
) *Service {
	return &Service{
		userRepo:                  userRepo,
		pilotRepo:                 pilotRepo,
		syncRepo:                  syncRepo,
		liveryMappingRepo:         liveryMappingRepo,
		dataProviderConfigRepo:    dataProviderConfigRepo,
		airtableProvider:          airtableProvider,
		validator:                 validator,
		cache:                     cache,
		flightsService:            flightsService,
		configService:             configService,
		dataProviderConfigService: dataProviderConfigService,
	}
}

// FlightData represents the current flight information for enrichment
type FlightData struct {
	FlightID string
	LiveryID string
	Aircraft string
	Livery   string
	Route    string
	Altitude int
	Speed    int
}

// SubmitPirep processes an incoming PIREP submission request
func (s *Service) SubmitPirep(
	ctx context.Context,
	request *dtos.PirepSubmitRequest,
	vaConfig *gormModels.VA,
	userClaims auth.UserClaims,
) (*dtos.PirepSubmitResponse, error) {
	// Log the incoming request
	requestJSON, _ := json.MarshalIndent(request, "", "  ")
	log.Printf("[PirepService] Received PIREP submission:\n%s\n", string(requestJSON))

	// STEP 1: VALIDATE MODE EXISTS AND IS ENABLED
	modeConfig, err := s.getModeConfig(vaConfig, request.Mode)
	if err != nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: fmt.Sprintf("Mode not found or not enabled: %s", request.Mode),
		}, nil
	}

	// STEP 2: VALIDATE REQUIRED FIELDS
	if err := s.validateRequiredFields(request, modeConfig); err != nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: err.Error(),
		}, nil
	}

	// STEP 3: RE-VALIDATE MODE (defensive check)
	// Note: For now, we skip this as it would require Live API call
	// This can be enhanced later to fetch current flight and re-validate

	// STEP 4: RESOLVE ROUTE
	var route *sync.RouteATSynced
	if modeConfig.AutoRoute != nil {
		// Auto-route mode: lookup by route name
		var err error
		route, err = s.resolveAutoRoute(ctx, vaConfig.ID, modeConfig.AutoRoute.RouteName)
		if err != nil {
			return &dtos.PirepSubmitResponse{
				Success:      false,
				ErrorType:    "validation_error",
				ErrorMessage: fmt.Sprintf("Auto-route not found: %s", modeConfig.AutoRoute.RouteName),
			}, nil
		}
	} else {
		// Manual route selection: use provided route string (e.g., "LFPG-EGLL")
		if request.RouteID == "" {
			return &dtos.PirepSubmitResponse{
				Success:      false,
				ErrorType:    "validation_error",
				ErrorMessage: "Route selection required but no route_id provided",
			}, nil
		}

		// Resolve route by name (the route_id is actually the route string)
		var err error
		route, err = s.syncRepo.FindRouteByName(ctx, vaConfig.ID, request.RouteID)
		if err != nil || route == nil {
			return &dtos.PirepSubmitResponse{
				Success:      false,
				ErrorType:    "validation_error",
				ErrorMessage: fmt.Sprintf("Route not found in system: %s", request.RouteID),
			}, nil
		}
	}

	// STEP 5: RESOLVE PILOT
	userDiscordID := userClaims.DiscordUserID()
	user, err := s.getUserWithVAAffiliations(ctx, userDiscordID)
	if err != nil || user == nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: "User not found",
		}, nil
	}

	// Get user's VA role for this VA (includes airtable_pilot_id)
	var userVARole *gormModels.UserVARole
	for i, role := range user.UserVARoles {
		if role.VAID == vaConfig.ID {
			userVARole = &user.UserVARoles[i]
			break
		}
	}

	if userVARole == nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: "User is not a member of this virtual airline",
		}, nil
	}

	// Verify airtable_pilot_id is set
	if userVARole.AirtablePilotID == nil || *userVARole.AirtablePilotID == "" {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: "User's pilot record not linked to Airtable",
		}, nil
	}

	// STEP 5.5: FETCH CURRENT FLIGHT DATA (for enrichment)
	// Get user's callsign and current flight from Live API
	flightData := &FlightData{}

	// Try to get prefix and suffix for better callsign matching
	prefix := s.getCallsignPrefix(ctx, vaConfig.ID)
	suffix := s.getCallsignSuffix(ctx, vaConfig.ID)

	currentFlight, err := s.flightsService.FindUserCurrentFlight(
		ctx,
		vaConfig.ID,
		userVARole.Callsign,
		prefix,
		suffix,
	)
	if err != nil {
		log.Printf("[PirepService] Warning: Could not fetch current flight data: %v", err)
		// Not a hard failure - we can still submit PIREP with provided liveryID
	} else if currentFlight != nil {
		flightData = &FlightData{
			FlightID: currentFlight.FlightID,
			LiveryID: currentFlight.LiveryId,
			Aircraft: currentFlight.Aircraft,
			Livery:   currentFlight.Livery,
			Route:    fmt.Sprintf("%s-%s", currentFlight.Origin, currentFlight.Destination),
			Altitude: currentFlight.AltitudeFt,
			Speed:    currentFlight.SpeedKts,
		}
	}

	// STEP 6: RESOLVE LIVERY MAPPING (aircraft/airline standardization)
	// Livery mappings standardize aircraft and airline names from Infinite Flight API to Airtable values
	// Flow: livery_id -> aircraft_livery table (get aircraft_name) -> livery_airtable_mappings (get target_value)
	// Uses Redis cache with 24-hour TTL for frequent mappings
	aircraft := "Unknown Aircraft"
	airline := "Unknown Airline"

	// Resolve livery mapping if livery_id is available from current flight
	if flightData.LiveryID != "" {
		if mappings, err := s.resolveLiveryMapping(ctx, vaConfig.ID, flightData.LiveryID); err == nil && mappings != nil {
			if a, ok := mappings["aircraft"]; ok && a != "" {
				aircraft = a
				log.Printf("[PirepService] Resolved aircraft from livery mapping: %s", aircraft)
			}
			if al, ok := mappings["airline"]; ok && al != "" {
				airline = al
				log.Printf("[PirepService] Resolved airline from livery mapping: %s", airline)
			}
		} else {
			log.Printf("[PirepService] Could not resolve livery mapping for livery_id %s: %v", flightData.LiveryID, err)
		}
	}

	// STEP 6.5: LOAD PROVIDER CONFIG AND GET PIREP SCHEMA (with caching)
	configData, err := s.dataProviderConfigService.GetActiveConfigCached(ctx, vaConfig.ID, "airtable")
	if err != nil || configData == nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: "Airtable provider configuration not available",
		}, nil
	}

	pirepSchema := configData.GetSchemaByType("pirep")
	if pirepSchema == nil {
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "validation_error",
			ErrorMessage: "PIREP schema not configured in provider settings",
		}, nil
	}

	// STEP 7: BUILD PIREP OBJECT FOR AIRTABLE
	pirepObj := s.buildPirepObject(
		request,
		modeConfig,
		user,
		userVARole,
		route,
		aircraft,
		airline,
		pirepSchema,
		flightData,
	)

	// STEP 8: SUBMIT TO AIRTABLE
	// Log the complete PIREP object before submission
	pirepJSON, _ := json.MarshalIndent(pirepObj, "", "  ")
	log.Printf("[PirepService] Submitting PIREP to Airtable:\n%s\n", string(pirepJSON))

	// Set provider config in context for provider to use
	ctx = context.WithValue(ctx, "provider_config", configData)

	pirepID, err := s.submitToAirtable(ctx, pirepObj, pirepSchema)
	if err != nil {
		log.Printf("[PirepService] Airtable submission error: %v", err)
		return &dtos.PirepSubmitResponse{
			Success:      false,
			ErrorType:    "airtable_error",
			ErrorMessage: "Failed to file PIREP with Airtable",
		}, nil
	}

	log.Printf("[PirepService] PIREP filed successfully: %s", pirepID)
	return &dtos.PirepSubmitResponse{
		Success: true,
		Message: "PIREP filed successfully",
		PirepID: pirepID,
	}, nil
}

// getModeConfig extracts and validates a flight mode configuration
func (s *Service) getModeConfig(va *gormModels.VA, modeID string) (*dtos.FlightModeConfig, error) {
	if va.FlightModesConfig == nil || len(va.FlightModesConfig) == 0 {
		return nil, fmt.Errorf("no flight modes configured")
	}

	flightModes, ok := va.FlightModesConfig["flight_modes"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid flight modes structure")
	}

	modeData, ok := flightModes[modeID].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("mode not found: %s", modeID)
	}

	// Check if enabled
	enabled, _ := modeData["enabled"].(bool)
	if !enabled {
		return nil, fmt.Errorf("mode not enabled: %s", modeID)
	}

	// Convert to FlightModeConfig struct
	modeConfigJSON, _ := json.Marshal(modeData)
	var config dtos.FlightModeConfig
	if err := json.Unmarshal(modeConfigJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse mode config: %w", err)
	}

	return &config, nil
}

// validateRequiredFields checks that all required fields in the request are present
func (s *Service) validateRequiredFields(
	request *dtos.PirepSubmitRequest,
	modeConfig *dtos.FlightModeConfig,
) error {
	if request.Mode == "" {
		return fmt.Errorf("mode is required")
	}

	if request.FlightTime == "" {
		return fmt.Errorf("flight_time is required")
	}

	// Check for mode-specific required fields
	for _, field := range modeConfig.Fields {
		if field.Required {
			switch field.Name {
			case "flight_time":
				// Already checked above
			case "fuel_kg":
				if request.FuelKg == nil {
					return fmt.Errorf("missing required field: fuel_kg")
				}
			case "cargo_kg":
				if request.CargoKg == nil {
					return fmt.Errorf("missing required field: cargo_kg")
				}
			case "passengers":
				if request.Passengers == nil {
					return fmt.Errorf("missing required field: passengers")
				}
			}
		}
	}

	return nil
}

// resolveAutoRoute finds an auto-route by name (normalized: trimmed, case-insensitive)
func (s *Service) resolveAutoRoute(ctx context.Context, vaID string, routeName string) (*sync.RouteATSynced, error) {
	// Normalize: trim spaces
	normalizedRouteName := strings.TrimSpace(routeName)

	route, err := s.syncRepo.FindRouteByName(ctx, vaID, normalizedRouteName)
	if err != nil || route == nil {
		return nil, fmt.Errorf("auto-route not found: %s", routeName)
	}
	return route, nil
}

// getUserWithVAAffiliations fetches a user with their VA affiliations
func (s *Service) getUserWithVAAffiliations(ctx context.Context, discordID string) (*gormModels.User, error) {
	user, err := s.userRepo.GetUserWithVAAffiliations(ctx, discordID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	return user, nil
}

// buildPirepObject constructs the PIREP object for Airtable submission using schema field mappings
func (s *Service) buildPirepObject(
	request *dtos.PirepSubmitRequest,
	modeConfig *dtos.FlightModeConfig,
	user *gormModels.User,
	userVARole *gormModels.UserVARole,
	route *sync.RouteATSynced,
	aircraft string,
	airline string,
	pirepSchema *dtos.EntitySchema,
	flightData *FlightData,
) map[string]interface{} {
	pirepObj := make(map[string]interface{})

	// Helper function to get Airtable field name from schema
	getFieldName := func(internalName string) string {
		fieldMapping := pirepSchema.GetFieldMapping(internalName)
		if fieldMapping != nil {
			return fieldMapping.AirtableName
		}
		return "" // Return empty if field not found in schema
	}

	// Find the bot metadata field (if configured)
	var botMetadataFieldName string
	for _, field := range pirepSchema.Fields {
		if field.BotMetadata {
			botMetadataFieldName = field.AirtableName
			log.Printf("\nField name: %s", botMetadataFieldName)
			break
		}
	}

	// Core fields (using schema mappings)
	if callsignField := getFieldName("callsign"); callsignField != "" {
		pirepObj[callsignField] = []string{*userVARole.AirtablePilotID}
	}

	if ifcUsernameField := getFieldName("ifc_username"); ifcUsernameField != "" && user.IFCommunityID != "" {
		pirepObj[ifcUsernameField] = user.IFCommunityID
	}

	if aircraftField := getFieldName("aircraft"); aircraftField != "" && aircraft != "" {
		pirepObj[aircraftField] = aircraft
	}

	if airlineField := getFieldName("airline"); airlineField != "" && airline != "" {
		pirepObj[airlineField] = airline
	}

	if route != nil {
		if routeField := getFieldName("route_at_id"); routeField != "" {
			pirepObj[routeField] = []string{route.ATID}
		}
	}

	if flightModeField := getFieldName("flight_mode"); flightModeField != "" && modeConfig.DisplayName != "" {
		pirepObj[flightModeField] = modeConfig.DisplayName
	}

	// Flight time with multiplier
	if flightTimeField := getFieldName("flight_time"); flightTimeField != "" {
		flightTimeSeconds := s.parseFlightTime(request.FlightTime)
		multiplier := s.getMultiplier(modeConfig)
		pirepObj[flightTimeField] = int(float64(flightTimeSeconds) * multiplier)
	}

	// Date completed
	if dateCompletedField := getFieldName("date_completed"); dateCompletedField != "" {
		today := time.Now().UTC()
		pirepObj[dateCompletedField] = today.Format("2006-01-02")
	}

	// Mode-specific fields
	if request.FuelKg != nil {
		if fuelField := getFieldName("fuel_kg"); fuelField != "" {
			pirepObj[fuelField] = *request.FuelKg
		}
	}
	if request.CargoKg != nil {
		if cargoField := getFieldName("cargo_kg"); cargoField != "" {
			pirepObj[cargoField] = *request.CargoKg
		}
	}
	if request.Passengers != nil {
		if paxField := getFieldName("passengers"); paxField != "" {
			pirepObj[paxField] = *request.Passengers
		}
	}

	// Pilot remarks with bot metadata enrichment
	remarksField := getFieldName("pilot_remarks")
	remarksValue := request.PilotRemarks

	// Append bot metadata if configured
	if botMetadataFieldName != "" && botMetadataFieldName == remarksField && flightData != nil {
		botMetadata := s.buildBotMetadataSection(request, modeConfig, flightData)
		if botMetadata != "" {
			if remarksValue != "" {
				remarksValue = remarksValue + "\n\n" + botMetadata
			} else {
				remarksValue = botMetadata
			}
		}
	}

	if remarksValue != "" && remarksField != "" {
		pirepObj[remarksField] = remarksValue
	}

	return pirepObj
}

// resolveLiveryMapping resolves aircraft and airline names from livery mappings
// Checks cache first, then queries database with caching
func (s *Service) resolveLiveryMapping(ctx context.Context, vaID string, liveryID string) (map[string]string, error) {
	// Check cache first with 24-hour TTL
	cacheKey := fmt.Sprintf("livery_mapping:%s:%s", vaID, liveryID)
	if cached, found := s.cache.Get(cacheKey); found {
		if mappingData, ok := cached.(map[string]interface{}); ok {
			result := make(map[string]string)
			if aircraft, ok := mappingData["aircraft"].(string); ok {
				result["aircraft"] = aircraft
			}
			if airline, ok := mappingData["airline"].(string); ok {
				result["airline"] = airline
			}
			log.Printf("[PirepService] Livery mapping cache hit for %s:%s", vaID, liveryID)
			return result, nil
		}
	}

	// Not in cache, query database
	mappings, err := s.liveryMappingRepo.GetMappingsByLivery(ctx, vaID, liveryID)
	if err != nil {
		log.Printf("[PirepService] Error fetching livery mappings: %v", err)
		return nil, err
	}

	// Cache the result with 24-hour TTL
	s.cache.Set(cacheKey, mappings, 24*time.Hour)
	log.Printf("[PirepService] Livery mapping cached for %s:%s", vaID, liveryID)

	return mappings, nil
}

// parseFlightTime converts "HH:MM" format to seconds
func (s *Service) parseFlightTime(flightTime string) int {
	parts := strings.Split(flightTime, ":")
	if len(parts) != 2 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	return (hours * 3600) + (minutes * 60)
}

// getMultiplier extracts the multiplier from mode config metadata
func (s *Service) getMultiplier(modeConfig *dtos.FlightModeConfig) float64 {
	if modeConfig.Metadata != nil {
		if m, ok := modeConfig.Metadata["multiplier"].(float64); ok {
			return m
		}
	}
	if modeConfig.AutoRoute != nil {
		return modeConfig.AutoRoute.Multiplier
	}
	return 1.0
}

// buildBotMetadataSection constructs the bot enriched metadata section for pilot remarks
func (s *Service) buildBotMetadataSection(
	request *dtos.PirepSubmitRequest,
	modeConfig *dtos.FlightModeConfig,
	flightData *FlightData,
) string {
	var metadata []string
	metadata = append(metadata, "--- BOT APPENDED SECTION ---")

	// Add flight time
	if request.FlightTime != "" {
		metadata = append(metadata, fmt.Sprintf("Actual FT: %s", request.FlightTime))
	}

	// Add multiplier
	multiplier := s.getMultiplier(modeConfig)
	metadata = append(metadata, fmt.Sprintf("Multiplier: %.1f", multiplier))

	// Add route
	if flightData.Route != "" {
		metadata = append(metadata, fmt.Sprintf("Actual Route from FPL: %s", flightData.Route))
	}

	// Add aircraft
	if flightData.Aircraft != "" {
		metadata = append(metadata, fmt.Sprintf("Aircraft: %s", flightData.Aircraft))
	}

	// Add livery
	if flightData.Livery != "" {
		metadata = append(metadata, fmt.Sprintf("Livery: %s", flightData.Livery))
	}

	// Add altitude
	if flightData.Altitude > 0 {
		metadata = append(metadata, fmt.Sprintf("Altitude: %d ft", flightData.Altitude))
	}

	// Add speed
	if flightData.Speed > 0 {
		metadata = append(metadata, fmt.Sprintf("Speed: %d knots", flightData.Speed))
	}

	return strings.Join(metadata, "\n")
}

// getCallsignPrefix retrieves the VA callsign prefix from config
func (s *Service) getCallsignPrefix(ctx context.Context, vaID string) string {
	if s.configService == nil {
		return ""
	}
	prefix, _ := s.configService.GetConfigVal(ctx, vaID, common.ConfigKeyCallsignPrefix)
	return prefix
}

// getCallsignSuffix retrieves the VA callsign suffix from config
func (s *Service) getCallsignSuffix(ctx context.Context, vaID string) string {
	if s.configService == nil {
		return ""
	}
	suffix, _ := s.configService.GetConfigVal(ctx, vaID, common.ConfigKeyCallsignSuffix)
	return suffix
}

// submitToAirtable creates a new PIREP record in Airtable using the provider
func (s *Service) submitToAirtable(
	ctx context.Context,
	pirepObj map[string]interface{},
	pirepSchema *dtos.EntitySchema,
) (string, error) {
	log.Printf("[PirepService] Submitting PIREP to Airtable via provider")
	log.Printf("[PirepService] Table: %s, Fields: %v", pirepSchema.TableName, pirepObj)

	// Use the DataProvider to submit the record
	recordID, err := s.airtableProvider.SubmitRecord(ctx, pirepSchema, pirepObj)
	if err != nil {
		return "", fmt.Errorf("failed to submit record via provider: %w", err)
	}

	log.Printf("[PirepService] Record submitted successfully: %s", recordID)
	return recordID, nil
}
