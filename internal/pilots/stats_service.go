package pilots

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/constants"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/sync"
	stdsync "sync"
	"time"

	"gorm.io/gorm"
)

type StatsService struct {
	gormDB           *gorm.DB
	cache            *cache.CacheService
	configAccessor   *platformVA.ProviderConfigAccessor
	airtableProvider *providers.AirtableProvider
	subjectReader    *statsSubjectReader
	liveAPIService   *statsLiveAPIService
	fieldMapper      *statsFieldMapper
	syncRepo         *sync.Repository
}

func NewStatsService(
	gormDB *gorm.DB,
	cache *cache.CacheService,
	membershipsSvc *platformMemberships.Service,
	configAccessor *platformVA.ProviderConfigAccessor,
	syncRepo *sync.Repository,
	liveAPIProvider *providers.LiveAPIProvider,
) *StatsService {
	return &StatsService{
		gormDB:           gormDB,
		cache:            cache,
		configAccessor:   configAccessor,
		airtableProvider: providers.NewAirtableProvider(cache),
		subjectReader:    newStatsSubjectReader(membershipsSvc),
		liveAPIService:   newStatsLiveAPIService(liveAPIProvider),
		fieldMapper:      newStatsFieldMapper(),
		syncRepo:         syncRepo,
	}
}

func (s *StatsService) getStatsSubject(ctx context.Context, userDiscordID, vaID string) (*platformMemberships.PilotStatsSubject, error) {
	return s.subjectReader.GetSubject(ctx, userDiscordID, vaID)
}

func (s *StatsService) getAirtableSchema(ctx context.Context, vaID, schemaType string) (*platformVA.SchemaConfig, error) {
	schema, err := s.configAccessor.GetAirtableSchema(ctx, vaID, schemaType)
	if err != nil {
		return nil, &StatsError{Code: constants.ErrCodeConfigNotFound, Message: "Failed to get schema config", Err: err}
	}
	return schema, nil
}

func (s *StatsService) getAirtableCredentials(ctx context.Context, vaID string) (*platformVA.ProviderCredentials, error) {
	creds, err := s.configAccessor.GetAirtableCredentials(ctx, vaID)
	if err != nil {
		return nil, &StatsError{Code: constants.ErrCodeConfigNotFound, Message: "Failed to get credentials config", Err: err}
	}
	return creds, nil
}

// GetPilotStatusByCallsign fetches pilot data from Airtable by searching for the callsign
// This method constructs the full callsign using the configured prefix
func (s *StatsService) GetPilotStatusByCallsign(ctx context.Context, userDiscordID, vaID string) (*PilotStatusResponse, error) {
	// Step 1: Get user's VA membership to check role and get callsign
	subject, err := s.getStatsSubject(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: constants.GetErrorMessage(constants.ErrCodePilotNotSynced),
			Err:     err,
		}
	}

	// Step 2: Check if user has a role (is a member)
	if subject.Role == "" {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: "User is not a member of this VA",
		}
	}

	// Step 3: Get callsign prefix from VA config
	callsignPrefix, ok := s.configAccessor.GetBasicConfigValue(ctx, vaID, platformVA.ConfigKeyAirtableCallsignColumnPrefix)
	if !ok {
		callsignPrefix = "" // Default to no prefix if not configured
	}

	// Step 4: Construct full callsign
	fullCallsign := callsignPrefix + subject.Callsign
	logging.Debug("Searching for pilot by callsign", "full_callsign", fullCallsign, "prefix", callsignPrefix, "base", subject.Callsign)

	pilotSchemaConfig, err := s.getAirtableSchema(ctx, vaID, platformVA.ConfigTypePilot)
	if err != nil {
		return nil, err
	}

	if pilotSchemaConfig == nil {
		return nil, &StatsError{
			Code:    constants.ErrCodeConfigNotFound,
			Message: "Pilot schema not found in configuration",
		}
	}
	pilotSchema := pilotSchemaConfig.ToEntitySchema(platformVA.ConfigTypePilot)

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
	logging.Debug("Airtable filter formula", "formula", filterFormula)

	// Step 9: Get credentials config separately
	creds, err := s.getAirtableCredentials(ctx, vaID)
	if err != nil || creds == nil {
		return nil, err
	}

	// Step 10: Fetch from Airtable using filter
	// Convert dtos.EntitySchema to platformVA.EntitySchema
	ctx = context.WithValue(ctx, "provider_credentials", creds)
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
	logging.Debug("Pilot status response from Airtable", "record_id", record.ID)

	// Step 12: Build response
	response := &PilotStatusResponse{
		AirtablePilotID: record.ID,
		Callsign:        subject.Callsign,
		FullCallsign:    fullCallsign,
		Role:            subject.Role,
		RawFields:       record.Fields,
		Metadata: PilotStatusMetadata{
			SchemaVersion: "typed-config",
			FetchedAt:     time.Now().UTC().Format(time.RFC3339),
			VAName:        subject.VAName,
			ConfigActive:  pilotSchemaConfig.Enabled,
		},
	}

	return response, nil
}

// GetPilotStats fetches comprehensive pilot statistics (game stats + provider data)
// This is the main entry point for the GET /api/v1/pilot/stats endpoint
func (s *StatsService) GetPilotStats(ctx context.Context, userDiscordID, vaID string) (*StatsResponse, error) {
	return s.GetPilotStatsWithOptions(ctx, userDiscordID, vaID, false)
}

func (s *StatsService) GetPilotStatsWithOptions(ctx context.Context, userDiscordID, vaID string, forceRefresh bool) (*StatsResponse, error) {
	profileCacheKey := statsProfileCachePrefix + vaID + ":" + userDiscordID
	refreshCooldownKey := statsRefreshCachePrefix + vaID + ":" + userDiscordID

	if forceRefresh {
		if _, inCooldown := s.cache.Get(refreshCooldownKey); inCooldown {
			return nil, &StatsError{
				Code:    constants.ErrCodeRateLimited,
				Message: "Pilot stats refresh is on cooldown; try again in a minute",
			}
		}
		s.cache.Set(refreshCooldownKey, true, statsRefreshCooldown)
	} else if cached, found := s.cache.Get(profileCacheKey); found {
		if cachedResp, ok := cached.(*StatsResponse); ok {
			cachedCopy := *cachedResp
			cachedCopy.Metadata.Cached = true
			return &cachedCopy, nil
		}
	}

	response := &StatsResponse{
		Metadata: StatsMetadata{
			LastFetched:        time.Now().UTC().Format(time.RFC3339),
			Cached:             false,
			ProviderConfigured: false,
		},
	}

	// Get user membership to get VA name
	subject, err := s.getStatsSubject(ctx, userDiscordID, vaID)
	if err != nil {
		return nil, &StatsError{
			Code:    constants.ErrCodePilotNotSynced,
			Message: "User is not a member of this VA",
			Err:     err,
		}
	}

	response.Metadata.VAName = subject.VAName
	if featureCfg, err := s.configAccessor.GetFeaturePilotStatsConfig(ctx, vaID); err != nil {
		logging.Warn("Feature pilot stats config unavailable", "va_id", vaID, "err", err)
	} else if featureCfg != nil && featureCfg.Enabled {
		response.Metadata.SchemaVersion = platformVA.ConfigTypeFeaturePilotStats
	}

	var (
		gameStats      *IFGameStats
		gameErr        error
		providerData   *ProviderPilotData
		rawFields      map[string]interface{}
		providerCached bool
		providerErr    error
		careerModeData *CareerModeData
		cmCached       bool
		careerErr      error
	)

	var wg stdsync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		gameStats, gameErr = s.liveAPIService.Fetch(ctx, subject)
	}()

	go func() {
		defer wg.Done()
		providerData, rawFields, providerCached, providerErr = s.fetchProviderData(ctx, subject)
	}()

	go func() {
		defer wg.Done()
		careerModeData, cmCached, careerErr = s.fetchCareerModeData(ctx, subject)
	}()

	wg.Wait()

	if gameErr == nil && gameStats != nil {
		response.GameStats = gameStats
	} else if gameErr != nil {
		logging.Warn("Failed to fetch IF game stats", "discord_id", userDiscordID, "va_id", vaID, "err", gameErr)
	}

	if providerErr != nil {
		logging.Warn("Provider data unavailable", "discord_id", userDiscordID, "va_id", vaID, "err", providerErr)
	} else {
		response.ProviderData = providerData
		response.Metadata.ProviderConfigured = true
		response.Metadata.Cached = providerCached
		if rawFields != nil {
			recentPIREPs, err := s.fetchRecentPIREPs(ctx, vaID, rawFields)
			if err != nil {
				logging.Warn("Failed to fetch recent PIREPs", "va_id", vaID, "err", err)
			} else if len(recentPIREPs) > 0 {
				response.RecentPIREPs = recentPIREPs
			}
		}
	}

	if careerErr != nil {
		logging.Warn("Career mode data unavailable", "discord_id", userDiscordID, "va_id", vaID, "err", careerErr)
	} else {
		response.CareerModeData = careerModeData
		response.Metadata.Cached = response.Metadata.Cached && cmCached
	}

	response.Insights = s.buildInsights(response)

	s.cache.Set(profileCacheKey, response, statsProfileTTL)

	return response, nil
}

func (s *StatsService) buildInsights(response *StatsResponse) *PilotStatsInsights {
	if response == nil {
		return nil
	}

	insights := &PilotStatsInsights{
		ProviderFreshness: &ProviderFreshness{
			LastFetchedAt: response.Metadata.LastFetched,
			Cached:        response.Metadata.Cached,
		},
	}

	if response.ProviderData != nil && response.ProviderData.LastActivity != nil {
		insights.ProviderFreshness.ProviderLastActivity = response.ProviderData.LastActivity
	}

	if len(response.RecentPIREPs) > 0 {
		recent := make([]RecentFlightCard, 0, len(response.RecentPIREPs))
		for _, p := range response.RecentPIREPs {
			recent = append(recent, RecentFlightCard{
				Route:      p.Route,
				FlightMode: p.FlightMode,
				FlightTime: p.FlightTime,
				Aircraft:   p.Aircraft,
				Livery:     p.Livery,
				OccurredAt: p.ATCreatedTime,
			})
			if len(recent) >= 5 {
				break
			}
		}
		insights.RecentFlights = recent
	}

	if response.CareerModeData != nil {
		career := &CareerProgressCard{LastRoute: response.CareerModeData.LastCareerModeFlight}
		if response.CareerModeData.AssignedRoutes != nil {
			switch v := (*response.CareerModeData.AssignedRoutes).(type) {
			case []interface{}:
				career.AssignedRouteCount = len(v)
			case []string:
				career.AssignedRouteCount = len(v)
			}
		}
		insights.Career = career
	}

	return insights
}

// fetchProviderData fetches and transforms data from the configured provider
// Returns: (providerData, rawFields, cached, error)
func (s *StatsService) fetchProviderData(ctx context.Context, subject *platformMemberships.PilotStatsSubject) (*ProviderPilotData, map[string]interface{}, bool, error) {
	vaID := subject.VAID
	// Get pilot schema config by type (separate config row)
	pilotSchemaConfig, err := s.getAirtableSchema(ctx, vaID, platformVA.ConfigTypePilot)
	if err != nil {
		return nil, nil, false, err
	}

	if pilotSchemaConfig == nil {
		return nil, nil, false, nil // No pilot config - optional data
	}
	pilotSchema := pilotSchemaConfig.ToEntitySchema(platformVA.ConfigTypePilot)

	// Check if enabled
	if !pilotSchema.Enabled {
		return nil, nil, false, nil // Schema disabled - optional data
	}

	// Get user's airtable_pilot_id
	// Check if airtable_pilot_id exists - if not, return nil (optional data)
	if subject.AirtablePilotID == nil || *subject.AirtablePilotID == "" {
		return nil, nil, false, nil // No error, just no data yet
	}

	airtablePilotID := *subject.AirtablePilotID

	// Check cache
	cacheKey := fmt.Sprintf("pilot_stats:%s:%s", vaID, airtablePilotID)
	if cachedData, found := s.cache.Get(cacheKey); found {
		if data, ok := cachedData.(*ProviderPilotData); ok {
			return data, nil, true, nil
		}
	}

	logging.Debug("Fetching provider data from Airtable", "at_pilot_id", airtablePilotID, "va_id", vaID)

	// Get credentials config separately
	creds, err := s.getAirtableCredentials(ctx, vaID)
	if err != nil || creds == nil {
		return nil, nil, false, err
	}

	ctx = context.WithValue(ctx, "provider_credentials", creds)
	pilotRecord, err := s.airtableProvider.FetchPilotRecord(ctx, airtablePilotID, pilotSchema)
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
			logging.Warn("Failed to fetch pilot from Airtable", "at_pilot_id", airtablePilotID, "va_id", vaID, "err", err)
			return nil, nil, false, nil
		}
		// Non-provider error - return nil (optional data)
		logging.Warn("Error fetching pilot from Airtable", "at_pilot_id", airtablePilotID, "va_id", vaID, "err", err)
		return nil, nil, false, nil
	}

	// Transform to standardized response
	providerData := s.fieldMapper.TransformProviderFields(pilotRecord.RawFields, pilotSchema)

	// Cache the result (10 minutes)
	s.cache.Set(cacheKey, providerData, 10*time.Minute)

	return providerData, pilotRecord.RawFields, false, nil
}

// fetchCareerModeData fetches career mode data using stored ID (no fallback to callsign)
func (s *StatsService) fetchCareerModeData(ctx context.Context, subject *platformMemberships.PilotStatsSubject) (*CareerModeData, bool, error) {
	vaID := subject.VAID
	// Get career mode schema config by type (separate config row)
	careerModeSchemaConfig, err := s.getAirtableSchema(ctx, vaID, platformVA.ConfigTypeCareerMode)
	if err != nil {
		return nil, false, err
	}

	if careerModeSchemaConfig == nil {
		// Career mode not configured - this is not an error
		return nil, false, nil
	}
	careerModeSchema := careerModeSchemaConfig.ToEntitySchema(platformVA.ConfigTypeCareerMode)

	// Check if enabled
	if !careerModeSchema.Enabled {
		return nil, false, nil // Schema disabled - optional data
	}

	// Get credentials config separately
	creds, err := s.getAirtableCredentials(ctx, vaID)
	if err != nil || creds == nil {
		return nil, false, err
	}

	ctx = context.WithValue(ctx, "provider_credentials", creds)

	var recordFields map[string]interface{}
	var recordID string

	// Check if we have a stored career mode pilot ID
	if subject.CareerModePilotID != nil && *subject.CareerModePilotID != "" {
		logging.Debug("Fetching career mode data by stored pilot ID", "at_pilot_id", *subject.CareerModePilotID, "va_id", vaID)
		pilotRecord, err := s.airtableProvider.FetchPilotRecord(ctx, *subject.CareerModePilotID, careerModeSchema)
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
				logging.Warn("Failed to fetch career mode by ID, falling back to callsign match", "va_id", vaID, "err_code", provErr.Code, "err", err)
				// Fall through to callsign matching below
			} else {
				// Non-provider error - fallback to callsign matching
				logging.Warn("Error fetching career mode by ID, falling back to callsign match", "va_id", vaID, "err", err)
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
		logging.Debug("Career mode: using callsign matching", "va_id", vaID)

		// Get callsign prefix from VA config
		callsignPrefix, ok := s.configAccessor.GetBasicConfigValue(ctx, vaID, platformVA.ConfigKeyAirtableCallsignColumnPrefix)
		if !ok {
			callsignPrefix = ""
		}

		// Construct full callsign
		fullCallsign := callsignPrefix + subject.Callsign

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
		logging.Debug("Career mode filter formula", "formula", filterFormula, "va_id", vaID)

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
		recordFields = record.Fields
		recordID = record.ID
	}

	logging.Debug("Career mode record fetched from Airtable", "record_id", recordID, "va_id", vaID)

	// Transform to standardized response
	careerModeData := s.fieldMapper.TransformCareerModeFields(recordFields, careerModeSchema)

	// Resolve last_flown_route if it contains a PIREP ID
	// If LastCareerModePIREP is set but LastCareerModeFlight is not, it means we need to fetch the route
	if careerModeData.LastCareerModePIREP != nil && careerModeData.LastCareerModeFlight == nil {
		logging.Debug("Resolving last_flown_route from PIREP ID", "va_id", vaID)
		route := s.fetchRouteFromPIREPID(ctx, vaID, *careerModeData.LastCareerModePIREP)
		if route != "" {
			careerModeData.LastCareerModeFlight = &route
		} else {
			// Fallback: Query by flight_mode="Career Mode" and callsign if available
			if subject != nil {
				callsign := subject.Callsign
				if callsign == "" && careerModeData.AdditionalFields != nil {
					if callsignVal, ok := careerModeData.AdditionalFields["Callsign"]; ok {
						if cs, ok := callsignVal.(string); ok {
							callsign = cs
						}
					}
				}

				if callsign != "" {
					route = s.fetchLastCareerModeRouteByCallsign(ctx, vaID, callsign, careerModeSchema)
					if route != "" {
						careerModeData.LastCareerModeFlight = &route
					}
				}
			}
		}
	}

	return careerModeData, false, nil
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
func (s *StatsService) fetchLastCareerModeRouteByCallsign(ctx context.Context, vaID string, callsign string, careerModeSchema *platformVA.EntitySchema) string {
	logging.Debug("Fetching last career mode route by callsign", "callsign", callsign, "va_id", vaID)

	// Get flight mode from schema config (if configured)
	flightModeFilter := "Career Mode" // Default to "Career Mode"
	if careerModeSchema.CareerModeFlightMode != nil && *careerModeSchema.CareerModeFlightMode != "" {
		flightModeFilter = *careerModeSchema.CareerModeFlightMode
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
		if err != gorm.ErrRecordNotFound {
			logging.Warn("Error fetching career mode route by callsign", "callsign", callsign, "va_id", vaID, "err", err)
		}
		return ""
	}

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
		return ""
	}

	// If no AT ID found, return empty
	if pirepATID == "" {
		return ""
	}

	logging.Debug("Fetching route from PIREP AT ID", "at_id", pirepATID, "va_id", vaID)

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
			// Fallback: Fetch directly from Airtable
			return s.fetchRouteFromAirtablePIREP(ctx, vaID, pirepATID)
		}
		logging.Warn("Error fetching PIREP route from DB", "at_id", pirepATID, "va_id", vaID, "err", err)
		return ""
	}

	return pirep.Route
}

// fetchRouteFromAirtablePIREP fetches a PIREP record directly from Airtable and extracts the route
func (s *StatsService) fetchRouteFromAirtablePIREP(ctx context.Context, vaID string, pirepATID string) string {
	logging.Debug("Fetching PIREP route directly from Airtable", "at_id", pirepATID, "va_id", vaID)

	// Get PIREP schema config
	pirepSchemaConfig, err := s.getAirtableSchema(ctx, vaID, platformVA.ConfigTypePirep)
	if err != nil || pirepSchemaConfig == nil {
		return ""
	}
	pirepSchema := pirepSchemaConfig.ToEntitySchema(platformVA.ConfigTypePirep)
	creds, err := s.getAirtableCredentials(ctx, vaID)
	if err != nil || creds == nil {
		return ""
	}

	// Set credentials in context
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Fetch PIREP record from Airtable (using FetchPilotRecord which works for any record type)
	pirepRecord, err := s.airtableProvider.FetchPilotRecord(ctx, pirepATID, pirepSchema)
	if err != nil {
		logging.Warn("Failed to fetch PIREP from Airtable", "at_id", pirepATID, "va_id", vaID, "err", err)
		return ""
	}

	// Extract route field from schema
	routeField := pirepSchema.GetFieldMapping("route")
	var routeValue interface{}
	var exists bool

	if routeField != nil {
		routeValue, exists = pirepRecord.RawFields[routeField.AirtableName]
	}

	// Fallback: Try common route field names directly
	if !exists {
		commonRouteNames := []string{"Route", "route", "Route Name", "Route Name (from Route)"}
		for _, fieldName := range commonRouteNames {
			if val, ok := pirepRecord.RawFields[fieldName]; ok {
				routeValue = val
				exists = true
				break
			}
		}
	}

	if !exists {
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
		return pirepATIDData // Return original data if we can't parse it
	}

	// If no AT ID found, return original data
	if pirepATID == "" {
		return pirepATIDData
	}

	logging.Debug("Fetching last career mode PIREP route", "at_id", pirepATID)

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
		logging.Warn("Error fetching career mode PIREP route from DB", "at_id", pirepATID, "err", err)
		return pirepATIDData // Return original data on error
	}

	if pirep.Route == "" {
		return pirepATIDData // Return original data if route is empty
	}

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
		return nil, nil
	}

	if len(atIDs) == 0 {
		return nil, nil
	}

	logging.Debug("Fetching recent PIREPs", "count", len(atIDs), "va_id", vaID)

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
		logging.Error("Failed to fetch recent PIREPs", "va_id", vaID, "err", err)
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

	logging.Debug("Fetched recent PIREPs", "count", len(recentPIREPs), "va_id", vaID)
	return recentPIREPs, nil
}
