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
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/sync"
	"log"
	"math"
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
			vur.career_mode_pilot_id,
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

	// Step 5: Get pilot schema config by type (separate config row)
	pilotConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "pilot")
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get pilot config",
			Err:     err,
		}
	}

	if pilotConfig == nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Pilot schema not found in configuration",
		}
	}

	// Step 6: Parse pilot schema directly from config_data
	var pilotSchema dtos.EntitySchema
	bytes, err := json.Marshal(pilotConfig.ConfigData)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal pilot config data",
			Err:     err,
		}
	}
	if err := json.Unmarshal(bytes, &pilotSchema); err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse pilot schema",
			Err:     err,
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

	// Step 9: Get credentials config separately
	credentialsConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil || credentialsConfig == nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get credentials config",
			Err:     err,
		}
	}

	// Parse credentials from config_data
	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal credentials config",
			Err:     err,
		}
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse credentials config",
			Err:     err,
		}
	}

	// Build credentials
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

	// Step 10: Fetch from Airtable using filter
	// Convert dtos.EntitySchema to platformVA.EntitySchema
	var vaPilotSchema *platformVA.EntitySchema = convertDTOsEntitySchema(&pilotSchema)
	ctx = context.WithValue(ctx, "provider_credentials", creds)
	filters := &providers.SyncFilters{
		FilterFormula: filterFormula,
		Limit:         1, // We only expect one record
	}

	recordSet, err := s.airtableProvider.FetchRecords(ctx, vaPilotSchema, filters)
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
			SchemaVersion: fmt.Sprintf("%d", pilotConfig.ConfigVersion),
			FetchedAt:     time.Now().Format(time.RFC3339),
			VAName:        membership.VAName,
			ConfigActive:  pilotConfig.IsActive,
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
	// Get pilot schema config by type (separate config row)
	pilotConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "pilot")
	if err != nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get pilot config",
			Err:     err,
		}
	}

	if pilotConfig == nil {
		return nil, nil, false, nil // No pilot config - optional data
	}

	// Parse pilot schema directly from config_data (it's stored as EntitySchema, not wrapped)
	var pilotSchema dtos.EntitySchema
	bytes, err := json.Marshal(pilotConfig.ConfigData)
	if err != nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal pilot config data",
			Err:     err,
		}
	}
	if err := json.Unmarshal(bytes, &pilotSchema); err != nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse pilot schema",
			Err:     err,
		}
	}

	// Check if enabled
	if !pilotSchema.Enabled {
		return nil, nil, false, nil // Schema disabled - optional data
	}

	// Get user's airtable_pilot_id
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, nil, false, err
	}

	// Check if airtable_pilot_id exists - if not, return nil (optional data)
	if membership.AirtablePilotID == nil || *membership.AirtablePilotID == "" {
		log.Printf("[fetchProviderData] No airtable_pilot_id for user %s in VA %s - skipping provider data fetch", userDiscordID, vaID)
		return nil, nil, false, nil // Return nil, nil, false, nil - no error, just no data
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
	
	// Get credentials config separately
	credentialsConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil || credentialsConfig == nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get credentials config",
			Err:     err,
		}
	}

	// Parse credentials from config_data
	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal credentials config",
			Err:     err,
		}
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		return nil, nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse credentials config",
			Err:     err,
		}
	}

	// Build credentials
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

	// Convert dtos.EntitySchema to platformVA.EntitySchema
	vaPilotSchema := convertDTOsEntitySchema(&pilotSchema)
	ctx = context.WithValue(ctx, "provider_credentials", creds)
	pilotRecord, err := s.airtableProvider.FetchPilotRecord(ctx, airtablePilotID, vaPilotSchema)
	if err != nil {
		// Check if it's a provider error
		if provErr, ok := err.(*providers.ProviderError); ok {
			// Only fail for authentication errors - for other errors, return nil (optional data)
			if provErr.Code == constants.ErrCodeInvalidAPIKey {
				return nil, nil, false, &StatsError{
					Code:    provErr.Code,
					Message: provErr.Message,
					Err:     err,
				}
			}
			// For other errors (record not found, network issues, etc.), return nil (optional data)
			log.Printf("[fetchProviderData] Failed to fetch pilot %s: %v - treating as optional", airtablePilotID, err)
			return nil, nil, false, nil
		}
		// Non-provider error - return nil (optional data)
		log.Printf("[fetchProviderData] Error fetching pilot %s: %v - treating as optional", airtablePilotID, err)
		return nil, nil, false, nil
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
	providerData := s.transformToStandardizedFields(pilotRecord.RawFields, &pilotSchema)

	// Log transformed data
	log.Printf("[fetchProviderData] Transformed provider data:")
	transformedJSON, _ := json.MarshalIndent(providerData, "", "  ")
	log.Printf("%s", string(transformedJSON))

	// Cache the result (10 minutes)
	s.cache.Set(cacheKey, providerData, 10*time.Minute)

	return providerData, pilotRecord.RawFields, false, nil
}

// formatTimeSeconds converts seconds to HH:MM format
func formatTimeSeconds(seconds interface{}) string {
	var secs int
	switch v := seconds.(type) {
	case float64:
		secs = int(math.Round(v))
	case int:
		secs = v
	case int64:
		secs = int(v)
	default:
		return fmt.Sprintf("%v", seconds)
	}
	hours := secs / 3600
	mins := (secs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

// isTimeField checks if a field should be treated as a time field (in seconds)
func isTimeField(field dtos.FieldMapping) bool {
	return field.DataType == "time" || (field.DisplayFormat != nil && *field.DisplayFormat == "duration")
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
		// Skip non-visible fields (only show fields marked as user-visible)
		if !field.IsUserVisible {
			continue
		}

		// Get value from raw data using the provider field name
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}

		// Check if this is a time field and format it
		if isTimeField(field) {
			formattedTime := formatTimeSeconds(value)
			value = formattedTime
		}

		// Map to standardized field based on internal_name (not display_name)
		internalName := field.InternalName

		// Normalize arrays to top 6 items
		if arr, ok := value.([]interface{}); ok {
			if len(arr) > 6 {
				value = arr[:6]
			}
		} else if arr, ok := value.([]string); ok {
			if len(arr) > 6 {
				value = arr[:6]
			}
		}

		switch internalName {
		case "flight_hours":
			// For time fields, value is already formatted as string - convert to interface{}
			var iface interface{}
			if isTimeField(field) {
				iface = value // Already formatted as string
			} else {
				// Handle different number types
				if v, ok := value.(float64); ok {
					iface = v
				} else if v, ok := value.(int); ok {
					iface = float64(v)
				} else {
					iface = value
				}
			}
			data.FlightHours = &iface

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
			// Use display_name if available, otherwise internal_name
			fieldKey := field.DisplayName
			if fieldKey == "" {
				fieldKey = field.InternalName
			}
			data.AdditionalFields[fieldKey] = value
		}
	}

	return data
}

// fetchCareerModeData fetches career mode data using stored ID (no fallback to callsign)
func (s *StatsService) fetchCareerModeData(ctx context.Context, userDiscordID, vaID string) (*CareerModeData, bool, error) {
	// Get career mode schema config by type (separate config row)
	careerModeConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "career_mode")
	if err != nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get career mode config",
			Err:     err,
		}
	}

	if careerModeConfig == nil {
		// Career mode not configured - this is not an error
		return nil, false, nil
	}

	// Parse career mode schema directly from config_data (it's stored as EntitySchema, not wrapped)
	var careerModeSchema dtos.EntitySchema
	bytes, err := json.Marshal(careerModeConfig.ConfigData)
	if err != nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal career mode config data",
			Err:     err,
		}
	}
	if err := json.Unmarshal(bytes, &careerModeSchema); err != nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse career mode schema",
			Err:     err,
		}
	}

	// Check if enabled
	if !careerModeSchema.Enabled {
		return nil, false, nil // Schema disabled - optional data
	}

	// Get user's membership (includes career_mode_pilot_id)
	membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, false, err
	}

	// Get credentials config separately
	credentialsConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil || credentialsConfig == nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Failed to get credentials config",
			Err:     err,
		}
	}

	// Parse credentials from config_data
	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to marshal credentials config",
			Err:     err,
		}
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		return nil, false, &StatsError{
			Code:    constants.ErrCodeConfigMalformed,
			Message: "Failed to parse credentials config",
			Err:     err,
		}
	}

	// Build credentials
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

	// Convert dtos.EntitySchema to platformVA.EntitySchema
	vaCareerModeSchema := convertDTOsEntitySchema(&careerModeSchema)
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	var recordFields map[string]interface{}
	var recordID string

	// Check if we have a stored career mode pilot ID
	if membership.CareerModePilotID != nil && *membership.CareerModePilotID != "" {
		// Use stored ID - faster and more reliable
		log.Printf("[fetchCareerModeData] Using stored career mode pilot ID: %s", *membership.CareerModePilotID)
		pilotRecord, err := s.airtableProvider.FetchPilotRecord(ctx, *membership.CareerModePilotID, vaCareerModeSchema)
		if err != nil {
			if provErr, ok := err.(*providers.ProviderError); ok {
				// Only fail for authentication errors - fallback to callsign matching for everything else
				if provErr.Code == constants.ErrCodeInvalidAPIKey {
					return nil, false, &StatsError{
						Code:    provErr.Code,
						Message: provErr.Message,
						Err:     err,
					}
				}
				// For other errors (record not found, network issues, etc.), fallback to callsign matching
				log.Printf("[fetchCareerModeData] Failed to fetch by ID (%s), falling back to callsign matching: %v", provErr.Code, err)
				// Fall through to callsign matching below
			} else {
				// Non-provider error - fallback to callsign matching
				log.Printf("[fetchCareerModeData] Failed to fetch by ID, falling back to callsign matching: %v", err)
				// Fall through to callsign matching below
			}
		} else {
			// Successfully fetched by ID
			recordFields = pilotRecord.RawFields
			recordID = pilotRecord.ProviderID
		}
	}

	// Fallback to callsign matching if ID not set or fetch by ID failed
	if recordFields == nil {
		log.Printf("[fetchCareerModeData] Using callsign matching (fallback)")

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

		filters := &providers.SyncFilters{
			FilterFormula: filterFormula,
			Limit:         1,
		}

		recordSet, err := s.airtableProvider.FetchRecords(ctx, vaCareerModeSchema, filters)
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
		recordFields = record.Fields
		recordID = record.ID
	}

	// Log raw fields from Airtable
	log.Printf("[fetchCareerModeData] Raw fields from Airtable (Record ID: %s):", recordID)
	rawJSON, _ := json.MarshalIndent(recordFields, "", "  ")
	log.Printf("%s", string(rawJSON))

	// Transform to standardized response
	careerModeData := s.transformCareerModeFields(recordFields, &careerModeSchema)
	
	// Resolve last_flown_route if it contains a PIREP ID
	// If LastCareerModePIREP is set but LastCareerModeFlight is not, it means we need to fetch the route
	if careerModeData.LastCareerModePIREP != nil && careerModeData.LastCareerModeFlight == nil {
		log.Printf("[fetchCareerModeData] Resolving last_flown_route from PIREP ID")
		route := s.fetchRouteFromPIREPID(ctx, vaID, *careerModeData.LastCareerModePIREP)
		if route != "" {
			careerModeData.LastCareerModeFlight = &route
			log.Printf("[fetchCareerModeData] Successfully resolved route from PIREP: %s", route)
		} else {
			log.Printf("[fetchCareerModeData] Failed to resolve route from PIREP ID, trying fallback: query by flight_mode and callsign")
			// Fallback: Query by flight_mode="Career Mode" and callsign if available
			// Get callsign from membership or from career mode data
			membership, err := s.getUserMembership(ctx, userDiscordID, vaID)
			if err == nil && membership != nil {
				// Try to get callsign from membership or from career mode data
				callsign := membership.Callsign
				if callsign == "" && careerModeData.AdditionalFields != nil {
					if callsignVal, ok := careerModeData.AdditionalFields["Callsign"]; ok {
						if cs, ok := callsignVal.(string); ok {
							callsign = cs
						}
					}
				}
				
				if callsign != "" {
					route = s.fetchLastCareerModeRouteByCallsign(ctx, vaID, callsign, &careerModeSchema)
					if route != "" {
						careerModeData.LastCareerModeFlight = &route
						log.Printf("[fetchCareerModeData] Successfully resolved route from fallback query: %s", route)
					} else {
						log.Printf("[fetchCareerModeData] Failed to resolve route from fallback query")
					}
				} else {
					log.Printf("[fetchCareerModeData] Cannot use fallback: callsign not available")
				}
			} else {
				log.Printf("[fetchCareerModeData] Cannot use fallback: membership not available")
			}
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
		// Skip non-visible fields (only show fields marked as user-visible)
		if !field.IsUserVisible {
			continue
		}

		// Get value from raw data using the provider field name
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}

		// Check if this is a time field and format it
		if isTimeField(field) {
			formattedTime := formatTimeSeconds(value)
			value = formattedTime
		}

		// Map to standardized field based on internal_name (not display_name)
		internalName := field.InternalName

		// Normalize arrays to top 6 items
		if arr, ok := value.([]interface{}); ok {
			if len(arr) > 6 {
				value = arr[:6]
			}
		} else if arr, ok := value.([]string); ok {
			if len(arr) > 6 {
				value = arr[:6]
			}
		}

		switch internalName {
		case "total_cm_hours":
			// For time fields, value is already formatted as string - convert to interface{}
			var iface interface{}
			if isTimeField(field) {
				iface = value // Already formatted as string
			} else {
				// Handle different number types
				if v, ok := value.(float64); ok {
					iface = v
				} else if v, ok := value.(int); ok {
					iface = float64(v)
				} else {
					iface = value
				}
			}
			data.TotalCMHours = &iface

		case "required_hours_to_next":
			// For time fields, value is already formatted as string - convert to interface{}
			var iface interface{}
			if isTimeField(field) {
				iface = value // Already formatted as string
			} else {
				// Handle different number types
				if v, ok := value.(float64); ok {
					iface = v
				} else if v, ok := value.(int); ok {
					iface = float64(v)
				} else {
					iface = value
				}
			}
			data.RequiredHoursToNext = &iface

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
			// Map last_flown_route to LastCareerModeFlight for API response
			// If the value is a PIREP ID (array or string starting with "rec"), fetch route from pirep_at_synced
			log.Printf("[transformCareerModeFields] Mapping last_flown_route: value=%v, type=%T", value, value)
			
			// Check if this is a PIREP ID that needs to be resolved
			var pirepATID string
			switch v := value.(type) {
			case []interface{}:
				// Array of PIREP IDs (Airtable linked records)
				if len(v) > 0 {
					if id, ok := v[0].(string); ok && len(id) > 3 && id[:3] == "rec" {
						pirepATID = id
					}
				}
			case []string:
				if len(v) > 0 && len(v[0]) > 3 && v[0][:3] == "rec" {
					pirepATID = v[0]
				}
			case string:
				// Handle string representation of array like "[rectwoPzdedmaZuFE]"
				if len(v) > 2 && v[0] == '[' && v[len(v)-1] == ']' {
					// Extract ID from string array format
					id := v[1 : len(v)-1]
					if len(id) > 3 && id[:3] == "rec" {
						pirepATID = id
					}
				} else if len(v) > 3 && v[:3] == "rec" {
					// Direct PIREP ID
					pirepATID = v
				} else {
					// It's already a route string
					data.LastCareerModeFlight = &v
					log.Printf("[transformCareerModeFields] last_flown_route is already a route string: %s", v)
					continue
				}
			}
			
			// If we found a PIREP ID, fetch the route from pirep_at_synced
			if pirepATID != "" {
				log.Printf("[transformCareerModeFields] Detected PIREP ID in last_flown_route, fetching route from pirep_at_synced: %s", pirepATID)
				// Note: We need vaID and ctx to fetch from DB, but we don't have them in transformCareerModeFields
				// So we'll store the PIREP ID and resolve it after transformation
				// Store as interface{} so we can resolve it later
				data.LastCareerModePIREP = &value
				log.Printf("[transformCareerModeFields] Stored PIREP ID for later resolution: %s", pirepATID)
			} else {
				// Not a PIREP ID, treat as direct route string
				if str := fmt.Sprintf("%v", value); str != "" {
					data.LastCareerModeFlight = &str
					log.Printf("[transformCareerModeFields] Using last_flown_route as direct route string: %s", str)
				}
			}

		default:
			// Non-standard field - add to additional_fields
			// Use display_name if available, otherwise internal_name
			fieldKey := field.DisplayName
			if fieldKey == "" {
				fieldKey = field.InternalName
			}
			data.AdditionalFields[fieldKey] = value
		}
	}

	return data
}

// getKeys returns a slice of keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// fetchLastCareerModeRouteByCallsign fetches the most recent career mode route by callsign and flight_mode
func (s *StatsService) fetchLastCareerModeRouteByCallsign(ctx context.Context, vaID string, callsign string, careerModeSchema *dtos.EntitySchema) string {
	log.Printf("[fetchLastCareerModeRouteByCallsign] Fetching last career mode route for callsign: %s", callsign)

	// Get flight mode from schema config (if configured)
	flightModeFilter := "Career Mode" // Default to "Career Mode"
	if careerModeSchema.CareerModeFlightMode != nil && *careerModeSchema.CareerModeFlightMode != "" {
		flightModeFilter = *careerModeSchema.CareerModeFlightMode
		log.Printf("[fetchLastCareerModeRouteByCallsign] Using configured flight mode: %s", flightModeFilter)
	}

	type PirepSynced struct {
		Route         string     `gorm:"column:route"`
		ATCreatedTime *time.Time `gorm:"column:at_created_time"`
	}

	var pirep PirepSynced
	query := s.gormDB.WithContext(ctx).
		Table("pirep_at_synced").
		Where("server_id = ?", vaID).
		Where("flight_mode = ?", flightModeFilter).
		Where("route IS NOT NULL AND route != ''").
		Where("pilot_callsign = ?", callsign) // Only match records with matching callsign

	err := query.
		Order("at_created_time DESC NULLS LAST").
		First(&pirep).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[fetchLastCareerModeRouteByCallsign] No PIREP found for callsign: %s, flight_mode: %s", callsign, flightModeFilter)
		} else {
			log.Printf("[fetchLastCareerModeRouteByCallsign] Error fetching PIREP: %v", err)
		}
		return ""
	}

	if pirep.Route == "" {
		log.Printf("[fetchLastCareerModeRouteByCallsign] Route not found in PIREP for callsign: %s", callsign)
		return ""
	}

	log.Printf("[fetchLastCareerModeRouteByCallsign] Successfully fetched route: %s", pirep.Route)
	return pirep.Route
}

// fetchRouteFromPIREPID fetches the route from pirep_at_synced using a PIREP AT ID
// The pirepIDData can be an array, string, or other format containing the PIREP ID
func (s *StatsService) fetchRouteFromPIREPID(ctx context.Context, vaID string, pirepIDData interface{}) string {
	// Extract the first AT ID from the data (can be array or single value)
	var pirepATID string

	switch v := pirepIDData.(type) {
	case []interface{}:
		// Array of PIREP IDs (Airtable linked records)
		if len(v) > 0 {
			if id, ok := v[0].(string); ok {
				pirepATID = id
			}
		}
	case []string:
		if len(v) > 0 {
			pirepATID = v[0]
		}
	case string:
		// Handle string representation of array like "[rectwoPzdedmaZuFE]"
		if len(v) > 2 && v[0] == '[' && v[len(v)-1] == ']' {
			// Extract ID from string array format
			id := v[1 : len(v)-1]
			if len(id) > 3 && id[:3] == "rec" {
				pirepATID = id
			}
		} else if len(v) > 3 && v[:3] == "rec" {
			pirepATID = v
		}
	default:
		log.Printf("[fetchRouteFromPIREPID] Unexpected type for PIREP ID: %T", pirepIDData)
		return ""
	}

	// If no AT ID found, return empty
	if pirepATID == "" {
		log.Printf("[fetchRouteFromPIREPID] No PIREP AT ID found in data: %v", pirepIDData)
		return ""
	}

	log.Printf("[fetchRouteFromPIREPID] Fetching route from PIREP with AT ID: %s", pirepATID)

	// Fetch the PIREP record from pirep_at_synced table
	type PirepSynced struct {
		Route string `gorm:"column:route"`
	}
	var pirep PirepSynced
	err := s.gormDB.WithContext(ctx).
		Table("pirep_at_synced").
		Where("at_id = ? AND server_id = ?", pirepATID, vaID).
		First(&pirep).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[fetchRouteFromPIREPID] PIREP not found in synced table with AT ID: %s, trying Airtable", pirepATID)
			// Fallback: Fetch directly from Airtable
			return s.fetchRouteFromAirtablePIREP(ctx, vaID, pirepATID)
		} else {
			log.Printf("[fetchRouteFromPIREPID] Error fetching PIREP: %v", err)
		}
		return ""
	}

	if pirep.Route == "" {
		log.Printf("[fetchRouteFromPIREPID] Route not found in PIREP with AT ID: %s", pirepATID)
		return ""
	}

	log.Printf("[fetchRouteFromPIREPID] Successfully fetched route: %s", pirep.Route)
	return pirep.Route
}

// fetchRouteFromAirtablePIREP fetches a PIREP record directly from Airtable and extracts the route
func (s *StatsService) fetchRouteFromAirtablePIREP(ctx context.Context, vaID string, pirepATID string) string {
	log.Printf("[fetchRouteFromAirtablePIREP] Fetching PIREP from Airtable with AT ID: %s", pirepATID)

	// Get PIREP schema config
	pirepConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "pirep")
	if err != nil || pirepConfig == nil {
		log.Printf("[fetchRouteFromAirtablePIREP] PIREP schema not configured: %v", err)
		return ""
	}

	// Parse PIREP schema
	var pirepSchema dtos.EntitySchema
	bytes, err := json.Marshal(pirepConfig.ConfigData)
	if err != nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Failed to marshal PIREP config: %v", err)
		return ""
	}
	if err := json.Unmarshal(bytes, &pirepSchema); err != nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Failed to parse PIREP schema: %v", err)
		return ""
	}

	// Get credentials config
	credentialsConfig, err := s.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil || credentialsConfig == nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Credentials not found: %v", err)
		return ""
	}

	// Parse credentials
	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Failed to marshal credentials: %v", err)
		return ""
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Failed to parse credentials: %v", err)
		return ""
	}

	// Build credentials
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

	// Convert schema and set credentials in context
	vaPirepSchema := convertDTOsEntitySchema(&pirepSchema)
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Fetch PIREP record from Airtable (using FetchPilotRecord which works for any record type)
	pirepRecord, err := s.airtableProvider.FetchPilotRecord(ctx, pirepATID, vaPirepSchema)
	if err != nil {
		log.Printf("[fetchRouteFromAirtablePIREP] Error fetching PIREP from Airtable: %v", err)
		return ""
	}

	// Log the full JSON response
	log.Printf("[fetchRouteFromAirtablePIREP] PIREP record from Airtable (AT ID: %s):", pirepATID)
	pirepJSON, _ := json.MarshalIndent(pirepRecord.RawFields, "", "  ")
	log.Printf("%s", string(pirepJSON))

	// Extract route field from schema
	routeField := pirepSchema.GetFieldMapping("route")
	var routeValue interface{}
	var exists bool
	
	if routeField != nil {
		// Try using schema mapping first
		routeValue, exists = pirepRecord.RawFields[routeField.AirtableName]
		if !exists {
			log.Printf("[fetchRouteFromAirtablePIREP] Route field '%s' (from schema) not found in PIREP record, trying direct field names", routeField.AirtableName)
		}
	}
	
	// Fallback: Try common route field names directly
	if !exists {
		commonRouteNames := []string{"Route", "route", "Route Name", "Route Name (from Route)"}
		for _, fieldName := range commonRouteNames {
			if val, ok := pirepRecord.RawFields[fieldName]; ok {
				routeValue = val
				exists = true
				log.Printf("[fetchRouteFromAirtablePIREP] Found route using direct field name: %s", fieldName)
				break
			}
		}
	}
	
	if !exists {
		log.Printf("[fetchRouteFromAirtablePIREP] Route field not found in PIREP record. Available fields: %v", getKeys(pirepRecord.RawFields))
		return ""
	}

	// Convert route value to string
	var route string
	switch v := routeValue.(type) {
	case string:
		route = v
	case []interface{}:
		// Handle array (might be linked records)
		if len(v) > 0 {
			if str, ok := v[0].(string); ok {
				route = str
			}
		}
	default:
		route = fmt.Sprintf("%v", v)
	}

	if route == "" {
		log.Printf("[fetchRouteFromAirtablePIREP] Route is empty in PIREP record")
		return ""
	}

	log.Printf("[fetchRouteFromAirtablePIREP] Successfully extracted route from Airtable: %s", route)
	return route
}

// DEPRECATED: fetchLastCareerModeFlight is no longer used.
// Last flown route is now fetched directly from the career mode Airtable record
// via the "last_flown_route" field mapping in the schema configuration.

// fetchAndTransformLastCareerModePIREP fetches the route name from the pirep_at_synced table
// The last_career_mode_pirep contains PIREP AT IDs (not route AT IDs)
// Returns just the route name as a string
// DEPRECATED: Use fetchLastCareerModeFlight instead which filters by flight mode
func (s *StatsService) fetchAndTransformLastCareerModePIREP(
	ctx context.Context,
	vaID string,
	pirepATIDData interface{},
) interface{} {
	// Extract the first AT ID from the data (can be array or single value)
	var pirepATID string

	switch v := pirepATIDData.(type) {
	case []interface{}:
		if len(v) > 0 {
			if id, ok := v[0].(string); ok {
				pirepATID = id
			}
		}
	case []string:
		if len(v) > 0 {
			pirepATID = v[0]
		}
	case string:
		// In case it's a single string
		pirepATID = v
	default:
		log.Printf("[fetchAndTransformLastCareerModePIREP] Unexpected type for PIREP AT ID: %T", pirepATIDData)
		return pirepATIDData // Return original data if we can't parse it
	}

	// If no AT ID found, return original data
	if pirepATID == "" {
		log.Printf("[fetchAndTransformLastCareerModePIREP] No PIREP AT ID found")
		return pirepATIDData
	}

	log.Printf("[fetchAndTransformLastCareerModePIREP] Fetching PIREP with AT ID: %s", pirepATID)

	// Fetch the PIREP record from pirep_at_synced table
	type PirepSynced struct {
		Route string `gorm:"column:route"`
	}
	var pirep PirepSynced
	err := s.gormDB.WithContext(ctx).
		Table("pirep_at_synced").
		Where("at_id = ? AND server_id = ?", pirepATID, vaID).
		First(&pirep).Error

	if err != nil {
		log.Printf("[fetchAndTransformLastCareerModePIREP] Error fetching PIREP: %v", err)
		return pirepATIDData // Return original data on error
	}

	if pirep.Route == "" {
		log.Printf("[fetchAndTransformLastCareerModePIREP] Route not found in PIREP with AT ID: %s", pirepATID)
		return pirepATIDData // Return original data if route is empty
	}

	log.Printf("[fetchAndTransformLastCareerModePIREP] Successfully fetched route: %s", pirep.Route)
	return pirep.Route
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
