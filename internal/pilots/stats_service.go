package pilots

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/sync"
	"log"
	"time"

	"gorm.io/gorm"
)

type StatsService struct {
	gormDB           *gorm.DB
	cache            *cache.CacheService
	configRepo       *repositories.DataProviderConfigRepo
	userRepo         *users.Repository
	vaConfigService  *common.VAConfigService
	syncRepo         *sync.Repository
	airtableProvider *providers.AirtableProvider
	liveAPIProvider  *providers.LiveAPIProvider
}

func NewStatsService(
	gormDB *gorm.DB,
	cache *cache.CacheService,
	configRepo *repositories.DataProviderConfigRepo,
	userRepo *users.Repository,
	vaConfigService *common.VAConfigService,
	syncRepo *sync.Repository,
) *StatsService {
	return &StatsService{
		gormDB:           gormDB,
		cache:            cache,
		configRepo:       configRepo,
		userRepo:         userRepo,
		vaConfigService:  vaConfigService,
		syncRepo:         syncRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
		liveAPIProvider:  providers.NewLiveAPIProvider(),
	}
}

// getUserMembership fetches the user's membership in the VA
func (s *StatsService) getUserMembership(ctx context.Context, userDiscordID, vaID string) (*MembershipWithAirtable, error) {
	query := `
		SELECT
			u.id as user_id,
			u.discord_id,
			u.if_community_id,
			vur.airtable_pilot_id,
			vur.callsign,
			vur.role,
			va.name as va_name
		FROM users u
		JOIN va_user_roles vur ON u.id = vur.user_id
		JOIN virtual_airlines va ON vur.va_id = va.id
		WHERE u.discord_id = $1 AND vur.va_id = $2 AND vur.is_active = true
		LIMIT 1
	`

	var membership MembershipWithAirtable
	err := s.gormDB.WithContext(ctx).Raw(query, userDiscordID, vaID).Scan(&membership).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user membership: %w", err)
	}

	return &membership, nil
}

// getActiveAirtableConfig fetches and parses the active Airtable config for a VA
func (s *StatsService) getActiveAirtableConfig(ctx context.Context, vaID string) (*models.DataProviderConfig, *dtos.ProviderConfigData, error) {
	// Get config entity from database
	config, err := s.configRepo.GetActiveConfig(ctx, vaID, "airtable")
	if err != nil {
		return nil, nil, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: constants.GetErrorMessage(constants.ErrCodeConfigNotFound),
			Err:     err,
		}
	}

	if config == nil {
		return nil, nil, &StatsError{
			Code:    constants.ErrCodeVAAirtableNotEnabled,
			Message: constants.GetErrorMessage(constants.ErrCodeVAAirtableNotEnabled),
		}
	}

	// Parse JSONB config_data
	configData, err := repositories.ParseConfigData(config.ConfigData)
	if err != nil {
		return nil, nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: constants.GetErrorMessage(constants.ErrCodeConfigMalformed),
			Err:     err,
		}
	}

	return config, configData, nil
}

// GetPilotStatusByCallsign fetches pilot data from Airtable by searching for the callsign
// This method constructs the full callsign using the configured prefix
func (s *StatsService) GetPilotStatusByCallsign(ctx context.Context, userDiscordID, vaID string) (*PilotStatusResponse, error) {
	// Step 1: Get user's VA membership to check role and get callsign
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: constants.GetErrorMessage(constants.ErrCodePilotNotSynced),
			Err:     err,
		}
	}

	// Step 2: Check if user has a role (is a member)
	if membership.Role == "" {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: "User is not a member of this VA",
		}
	}

	// Step 3: Get callsign prefix from VA config
	callsignPrefix, ok := s.vaConfigService.GetConfigVal(ctx, vaID, common.ConfigKeyAirtableCallsignColumnPrefix)
	if !ok {
		callsignPrefix = "" // Default to no prefix if not configured
	}

	// Step 4: Construct full callsign
	fullCallsign := callsignPrefix + membership.Callsign
	fmt.Printf("Searching for pilot with callsign: %s (prefix: %s, base: %s)\n", fullCallsign, callsignPrefix, membership.Callsign)

	// Step 5: Get active Airtable config for VA
	config, configData, err := s.getActiveAirtableConfig(ctx, vaID)
	if err != nil {
		return nil, err
	}

	// Step 6: Get pilot schema from config
	pilotSchema := configData.GetSchemaByType("pilot")
	if pilotSchema == nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Pilot schema not found in configuration",
		}
	}

	// Step 7: Get the callsign field name from schema
	var callsignFieldName string
	for _, field := range pilotSchema.Fields {
		if field.InternalName == "callsign" {
			callsignFieldName = field.AirtableName
			break
		}
	}

	if callsignFieldName == "" {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Callsign field not found in pilot schema",
		}
	}

	// Step 8: Build Airtable filter formula
	// Airtable formula: {Callsign} = 'TEST012'
	filterFormula := fmt.Sprintf("{%s} = '%s'", callsignFieldName, fullCallsign)
	fmt.Printf("Airtable filter formula: %s\n", filterFormula)

	// Step 9: Fetch from Airtable using filter
	ctx = context.WithValue(ctx, "provider_config", configData)
	filters := &providers.SyncFilters{
		FilterFormula: filterFormula,
		Limit:         1, // We only expect one record
	}

	recordSet, err := s.airtableProvider.FetchRecords(ctx, pilotSchema, filters)
	if err != nil {
		if provErr, ok := err.(*providers.ProviderError); ok {
			return nil, &StatsError{
				Code:    provErr.Code,
				Message: provErr.Message,
				Err:     err,
			}
		}
		return nil, &StatsError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}

	// Step 10: Check if we found a record
	if len(recordSet.Records) == 0 {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotFoundInAirtable,
			Message: fmt.Sprintf("No pilot found with callsign: %s", fullCallsign),
		}
	}

	// Step 11: Log the raw response data
	record := recordSet.Records[0]
	fmt.Printf("\n=== PILOT STATUS RESPONSE FROM AIRTABLE ===\n")
	fmt.Printf("Record ID: %s\n", record.ID)

	// Pretty print the fields as JSON
	fieldsJSON, err := json.MarshalIndent(record.Fields, "", "  ")
	if err != nil {
		fmt.Printf("Raw Fields (error formatting): %+v\n", record.Fields)
	} else {
		fmt.Printf("Raw Fields (JSON):\n%s\n", string(fieldsJSON))
	}
	fmt.Printf("===========================================\n\n")

	// Step 12: Build response
	response := &PilotStatusResponse{
		AirtablePilotID: record.ID,
		Callsign:        membership.Callsign,
		FullCallsign:    fullCallsign,
		Role:            membership.Role,
		RawFields:       record.Fields,
		Metadata: PilotStatusMetadata{
			SchemaVersion: configData.Version,
			FetchedAt:     time.Now().Format(time.RFC3339),
			VAName:        membership.VAName,
			ConfigActive:  config.IsActive,
		},
	}

	return response, nil
}

// fetchIFGameStats fetches Infinite Flight game statistics from the Live API
// Returns nil if the user's IFC ID is not available or API call fails
func (s *StatsService) fetchIFGameStats(ctx context.Context, ifcID string) (*IFGameStats, error) {
	// Validate IFC ID
	if ifcID == "" {
		log.Printf("[fetchIFGameStats] IFC ID is empty, skipping game stats fetch")
		return nil, nil
	}

	log.Printf("[fetchIFGameStats] Fetching game stats for IFC ID: %s", ifcID)

	// Call Live API provider to get user stats
	userStatsResp, statusCode, err := s.liveAPIProvider.GetUserByIfcId(ctx, ifcID)
	if err != nil {
		log.Printf("[fetchIFGameStats] Error fetching user stats from Live API (status: %d): %v", statusCode, err)
		// Game stats are optional, so we log the error but don't fail
		return nil, nil
	}

	// Check if we have results
	if userStatsResp == nil || len(userStatsResp.Result) == 0 {
		log.Printf("[fetchIFGameStats] No user stats found in Live API response")
		return nil, nil
	}

	// Get the first result (should be only one)
	userStats := userStatsResp.Result[0]

	discourseUsername := ""
	if userStats.DiscourseUsername != nil {
		discourseUsername = *userStats.DiscourseUsername
	}
	log.Printf("[fetchIFGameStats] Successfully fetched game stats for user %s", discourseUsername)

	// Transform to IFGameStats DTO
	// Note: FlightTime from Live API is in minutes, convert to seconds for consistency
	gameStats := &IFGameStats{
		FlightTime:    userStats.FlightTime * 60, // Convert minutes to seconds
		OnlineFlights: userStats.OnlineFlights,
		LandingCount:  userStats.LandingCount,
		XP:            userStats.XP,
		Grade:         userStats.Grade,
		Violations:    userStats.Violations,
	}

	return gameStats, nil
}

// GetPilotStats fetches comprehensive pilot statistics (game stats + provider data)
// This is the main entry point for the GET /api/v1/pilot/stats endpoint
func (s *StatsService) GetPilotStats(ctx context.Context, userDiscordID, vaID string) (*StatsResponse, error) {
	response := &StatsResponse{
		Metadata: StatsMetadata{
			LastFetched:        time.Now().Format(time.RFC3339),
			Cached:             false,
			ProviderConfigured: false,
		},
	}

	// Get user membership to get VA name
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: "User is not a member of this VA",
			Err:     err,
		}
	}

	response.Metadata.VAName = membership.VAName

	// Fetch IF game stats from Live API using user's IFC Community ID
	gameStats, err := s.fetchIFGameStats(ctx, membership.IFCommunityID)
	if err == nil && gameStats != nil {
		response.GameStats = gameStats
	} else if err != nil {
		log.Printf("[GetPilotStats] Error fetching game stats: %v", err)
		// Game stats are optional - don't fail the entire request
	}

	// Fetch provider data (Airtable, etc.)
	providerData, rawFields, cached, err := s.fetchProviderData(ctx, userDiscordID, vaID)
	if err != nil {
		// Provider data is optional - log but don't fail
		log.Printf("[GetPilotStats] Provider data unavailable for user %s in VA %s: %v", userDiscordID, vaID, err)
	} else {
		response.ProviderData = providerData
		response.Metadata.ProviderConfigured = true
		response.Metadata.Cached = cached

		// Fetch recent PIREPs using raw fields from Airtable
		if rawFields != nil {
			recentPIREPs, err := s.fetchRecentPIREPs(ctx, vaID, rawFields)
			if err != nil {
				log.Printf("[GetPilotStats] Error fetching recent PIREPs: %v", err)
			} else if len(recentPIREPs) > 0 {
				response.RecentPIREPs = recentPIREPs
			}
		}
	}

	// Fetch career mode data if configured
	careerModeData, cmCached, err := s.fetchCareerModeData(ctx, userDiscordID, vaID)
	if err != nil {
		// Career mode data is optional - log but don't fail
		log.Printf("[GetPilotStats] Career mode data unavailable for user %s in VA %s: %v", userDiscordID, vaID, err)
	} else {
		response.CareerModeData = careerModeData
		// Update cached flag if career mode was also cached
		response.Metadata.Cached = response.Metadata.Cached && cmCached
	}

	return response, nil
}

// fetchProviderData fetches and transforms data from the configured provider
// Returns: (providerData, rawFields, cached, error)
func (s *StatsService) fetchProviderData(ctx context.Context, userDiscordID, vaID string) (*ProviderPilotData, map[string]interface{}, bool, error) {
	// Get active config
	_, configData, err := s.getActiveAirtableConfig(ctx, vaID)
	if err != nil {
		return nil, nil, false, err
	}

	// Get pilot schema
	pilotSchema := configData.GetSchemaByType("pilot")
	if pilotSchema == nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Pilot schema not found in configuration",
		}
	}

	// Get user's airtable_pilot_id
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, nil, false, err
	}

	if membership.AirtablePilotID == nil || *membership.AirtablePilotID == "" {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodePilotAirtableIDMissing,
			Message: "Pilot not synced with Airtable",
		}
	}

	airtablePilotID := *membership.AirtablePilotID

	// Check cache
	cacheKey := fmt.Sprintf("pilot_stats:%s:%s", vaID, airtablePilotID)
	if cachedData, found := s.cache.Get(cacheKey); found {
		if data, ok := cachedData.(*ProviderPilotData); ok {
			log.Printf("[fetchProviderData] Cache hit for pilot %s in VA %s", airtablePilotID, vaID)
			_ = data // Temporarily ignoring cache
			// return data, nil, true, nil
		}
	}

	log.Printf("[fetchProviderData] Fetching from Airtable for pilot %s in VA %s", airtablePilotID, vaID)
	// Fetch from provider
	ctx = context.WithValue(ctx, "provider_config", configData)
	pilotRecord, err := s.airtableProvider.FetchPilotRecord(ctx, airtablePilotID, pilotSchema)
	if err != nil {
		// Check if it's a provider error
		if provErr, ok := err.(*providers.ProviderError); ok {
			return nil, nil, false, &StatsError{
				Code:    provErr.Code,
				Message: provErr.Message,
				Err:     err,
			}
		}
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}

	// Log raw fields from Airtable
	log.Printf("[fetchProviderData] Raw fields from Airtable:")
	rawJSON, _ := json.MarshalIndent(pilotRecord.RawFields, "", "  ")
	log.Printf("%s", string(rawJSON))

	// Log schema fields for debugging
	log.Printf("[fetchProviderData] Schema fields configuration:")
	for _, field := range pilotSchema.Fields {
		log.Printf("  - AirtableName: %s, InternalName: %s, DisplayName: %s, IsUserVisible: %v",
			field.AirtableName, field.InternalName, field.DisplayName, field.IsUserVisible)
	}

	// Transform to standardized response
	providerData := s.transformToStandardizedFields(pilotRecord.RawFields, pilotSchema)

	// Log transformed data
	log.Printf("[fetchProviderData] Transformed provider data:")
	transformedJSON, _ := json.MarshalIndent(providerData, "", "  ")
	log.Printf("%s", string(transformedJSON))

	// Cache the result (10 minutes)
	s.cache.Set(cacheKey, providerData, 10*time.Minute)

	return providerData, pilotRecord.RawFields, false, nil
}

// transformToStandardizedFields maps raw provider data to standardized API response
// It uses the DisplayName field in the schema to map to standard field names
func (s *StatsService) transformToStandardizedFields(
	rawFields map[string]interface{},
	schema *dtos.EntitySchema,
) *ProviderPilotData {
	data := &ProviderPilotData{
		AdditionalFields: make(map[string]interface{}),
	}

	for _, field := range schema.Fields {
		// TODO: Re-enable this check once schema is properly configured
		// Skip non-visible fields
		// if !field.IsUserVisible {
		// 	continue
		// }

		// Get value from raw data using the provider field name
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}

		// Map to standardized field based on display_name or internal_name
		displayOrInternalName := field.DisplayName
		if displayOrInternalName == "" {
			displayOrInternalName = field.InternalName
		}

		switch displayOrInternalName {
		case "flight_hours":
			data.FlightHours = &value

		case "rank":
			if v, ok := value.(string); ok {
				data.Rank = &v
			}

		case "join_date":
			if v, ok := value.(string); ok {
				data.JoinDate = &v
			}

		case "last_activity":
			if v, ok := value.(string); ok {
				data.LastActivity = &v
			}

		case "last_flight":
			if v, ok := value.(string); ok {
				data.LastFlight = &v
			}

		case "region":
			if v, ok := value.(string); ok {
				data.Region = &v
			}

		case "total_flights":
			// Handle both float and int types
			if v, ok := value.(float64); ok {
				intVal := int(v)
				data.TotalFlights = &intVal
			} else if v, ok := value.(int); ok {
				data.TotalFlights = &v
			}

		case "status":
			if v, ok := value.(string); ok {
				data.Status = &v
			}

		default:
			// Non-standard field - add to additional_fields
			if field.DisplayName != "" {
				// Use the display_name as the key
				data.AdditionalFields[field.DisplayName] = value
			} else {
				// If no display_name, use internal_name
				data.AdditionalFields[field.InternalName] = value
			}
		}
	}

	return data
}

// fetchCareerModeData fetches career mode data using callsign matching
func (s *StatsService) fetchCareerModeData(ctx context.Context, userDiscordID, vaID string) (*CareerModeData, bool, error) {
	// Get active config
	_, configData, err := s.getActiveAirtableConfig(ctx, vaID)
	if err != nil {
		return nil, false, err
	}

	// Get career mode schema
	careerModeSchema := configData.GetSchemaByType("career_mode")
	if careerModeSchema == nil {
		// Career mode not configured - this is not an error
		return nil, false, fmt.Errorf("career mode schema not configured")
	}

	// Get user's callsign for matching
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, false, err
	}

	// Get callsign prefix from VA config
	callsignPrefix, ok := s.vaConfigService.GetConfigVal(ctx, vaID, common.ConfigKeyAirtableCallsignColumnPrefix)
	if !ok {
		callsignPrefix = ""
	}

	// Construct full callsign
	fullCallsign := callsignPrefix + membership.Callsign

	// Get the callsign field name from schema
	var callsignFieldName string
	for _, field := range careerModeSchema.Fields {
		if field.InternalName == "callsign" {
			callsignFieldName = field.AirtableName
			break
		}
	}

	if callsignFieldName == "" {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Callsign field not found in career mode schema",
		}
	}

	// Build Airtable filter formula
	filterFormula := fmt.Sprintf("{%s} = '%s'", callsignFieldName, fullCallsign)
	log.Printf("[fetchCareerModeData] Filter formula: %s", filterFormula)

	// Fetch from Airtable using filter
	ctx = context.WithValue(ctx, "provider_config", configData)
	filters := &providers.SyncFilters{
		FilterFormula: filterFormula,
		Limit:         1,
	}

	recordSet, err := s.airtableProvider.FetchRecords(ctx, careerModeSchema, filters)
	if err != nil {
		if provErr, ok := err.(*providers.ProviderError); ok {
			return nil, false, &StatsError{
				Code:    provErr.Code,
				Message: provErr.Message,
				Err:     err,
			}
		}
		return nil, false, &StatsError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}

	// Check if we found a record
	if len(recordSet.Records) == 0 {
		return nil, false, &StatsError{
			Code:    constants.ErrCodePilotNotFoundInAirtable,
			Message: fmt.Sprintf("No career mode record found for callsign: %s", fullCallsign),
		}
	}

	record := recordSet.Records[0]

	// Log raw fields from Airtable
	log.Printf("[fetchCareerModeData] Raw fields from Airtable:")
	rawJSON, _ := json.MarshalIndent(record.Fields, "", "  ")
	log.Printf("%s", string(rawJSON))

	// Transform to standardized response
	careerModeData := s.transformCareerModeFields(record.Fields, careerModeSchema)

	// Fetch and enrich last_career_mode_flight with route data from route_at_synced
	if careerModeData.LastCareerModePIREP != nil {
		routeName := s.fetchAndTransformLastCareerModePIREP(ctx, vaID, *careerModeData.LastCareerModePIREP)
		if routeStr, ok := routeName.(string); ok {
			careerModeData.LastCareerModeFlight = &routeStr
		}
	}

	// Log transformed data
	log.Printf("[fetchCareerModeData] Transformed career mode data:")
	transformedJSON, _ := json.MarshalIndent(careerModeData, "", "  ")
	log.Printf("%s", string(transformedJSON))

	return careerModeData, false, nil
}

// transformCareerModeFields maps raw provider data to career mode response
func (s *StatsService) transformCareerModeFields(
	rawFields map[string]interface{},
	schema *dtos.EntitySchema,
) *CareerModeData {
	data := &CareerModeData{
		AdditionalFields: make(map[string]interface{}),
	}

	for _, field := range schema.Fields {
		// Get value from raw data using the provider field name
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}

		// Map to standardized field based on display_name or internal_name
		displayOrInternalName := field.DisplayName
		if displayOrInternalName == "" {
			displayOrInternalName = field.InternalName
		}

		switch displayOrInternalName {
		case "total_cm_hours":
			data.TotalCMHours = &value

		case "required_hours_to_next":
			data.RequiredHoursToNext = &value

		case "last_activity_cm":
			if v, ok := value.(string); ok {
				data.LastActivityCM = &v
			}

		case "assigned_routes":
			data.AssignedRoutes = &value

		case "aircraft":
			if v, ok := value.(string); ok {
				data.Aircraft = &v
			}

		case "airline":
			if v, ok := value.(string); ok {
				data.Airline = &v
			}

		case "last_career_mode_pirep":
			// This will be populated by fetchAndTransformLastCareerModePIREP
			// which is called after all fields are processed in fetchCareerModeData
			data.LastCareerModePIREP = &value

		case "last_flown_route":
			if v, ok := value.(string); ok {
				data.LastFlownRoute = &v
			}

		default:
			// Non-standard field - add to additional_fields
			if field.DisplayName != "" {
				data.AdditionalFields[field.DisplayName] = value
			} else {
				data.AdditionalFields[field.InternalName] = value
			}
		}
	}

	return data
}

// fetchAndTransformLastCareerModePIREP fetches the route name from the route_at_synced table
// The last_career_mode_pirep contains AT IDs that are direct references to routes
// Returns just the route name as a string
func (s *StatsService) fetchAndTransformLastCareerModePIREP(
	ctx context.Context,
	vaID string,
	pirepATIDData interface{},
) interface{} {
	// Extract the first AT ID from the data (can be array or single value)
	var routeATID string

	switch v := pirepATIDData.(type) {
	case []interface{}:
		if len(v) > 0 {
			if id, ok := v[0].(string); ok {
				routeATID = id
			}
		}
	case []string:
		if len(v) > 0 {
			routeATID = v[0]
		}
	case string:
		// In case it's a single string
		routeATID = v
	default:
		log.Printf("[fetchAndTransformLastCareerModePIREP] Unexpected type for route AT ID: %T", pirepATIDData)
		return pirepATIDData // Return original data if we can't parse it
	}

	// If no AT ID found, return original data
	if routeATID == "" {
		log.Printf("[fetchAndTransformLastCareerModePIREP] No route AT ID found")
		return pirepATIDData
	}

	log.Printf("[fetchAndTransformLastCareerModePIREP] Fetching route with AT ID: %s", routeATID)

	// Fetch the route record from route_at_synced table
	route, err := s.syncRepo.FindRouteByATID(ctx, vaID, routeATID)
	if err != nil {
		log.Printf("[fetchAndTransformLastCareerModePIREP] Error fetching route: %v", err)
		return pirepATIDData // Return original data on error
	}

	if route == nil {
		log.Printf("[fetchAndTransformLastCareerModePIREP] Route not found for AT ID: %s", routeATID)
		return pirepATIDData // Return original data if not found
	}

	log.Printf("[fetchAndTransformLastCareerModePIREP] Successfully fetched route: %s", route.Route)
	return route.Route
}

// fetchRecentPIREPs fetches the last 5 recent PIREPs from the synced data
func (s *StatsService) fetchRecentPIREPs(ctx context.Context, vaID string, rawFields map[string]interface{}) ([]RecentPIREP, error) {
	// Extract "Recent Logs" field from raw Airtable data
	recentLogsRaw, exists := rawFields["Recent Logs"]
	if !exists {
		// Try alternative field name "Recent Logged Flights"
		recentLogsRaw, exists = rawFields["Recent Logged Flights"]
		if !exists {
			log.Printf("[fetchRecentPIREPs] No recent logs field found in Airtable data")
			return nil, nil
		}
	}

	// Convert to string slice
	var atIDs []string
	switch v := recentLogsRaw.(type) {
	case []interface{}:
		for _, id := range v {
			if strID, ok := id.(string); ok {
				atIDs = append(atIDs, strID)
			}
		}
	case []string:
		atIDs = v
	default:
		log.Printf("[fetchRecentPIREPs] Unexpected type for Recent Logs: %T", recentLogsRaw)
		return nil, nil
	}

	if len(atIDs) == 0 {
		log.Printf("[fetchRecentPIREPs] No recent log IDs found")
		return nil, nil
	}

	log.Printf("[fetchRecentPIREPs] Fetching %d recent PIREPs from synced data", len(atIDs))

	// Fetch PIREPs from database directly using GORM (to avoid circular dependency)
	var pireps []struct {
		ATID          string
		Route         string
		FlightMode    string
		FlightTime    *float64
		PilotCallsign string
		Aircraft      string
		Livery        string
		ATCreatedTime *time.Time
	}

	err := s.gormDB.WithContext(ctx).
		Table("pirep_at_synced").
		Where("server_id = ? AND at_id IN ?", vaID, atIDs).
		Order("at_created_time DESC").
		Limit(5).
		Find(&pireps).Error

	if err != nil {
		log.Printf("[fetchRecentPIREPs] Error fetching PIREPs: %v", err)
		return nil, err
	}

	// Transform to response DTOs
	var recentPIREPs []RecentPIREP
	for _, pirep := range pireps {
		dto := RecentPIREP{
			ATID:          pirep.ATID,
			Route:         pirep.Route,
			FlightMode:    pirep.FlightMode,
			FlightTime:    pirep.FlightTime,
			PilotCallsign: pirep.PilotCallsign,
			Aircraft:      pirep.Aircraft,
			Livery:        pirep.Livery,
		}

		// Format ATCreatedTime if present
		if pirep.ATCreatedTime != nil {
			formattedTime := pirep.ATCreatedTime.Format(time.RFC3339)
			dto.ATCreatedTime = &formattedTime
		}

		recentPIREPs = append(recentPIREPs, dto)
	}

	log.Printf("[fetchRecentPIREPs] Successfully fetched %d PIREPs", len(recentPIREPs))
	return recentPIREPs, nil
}
