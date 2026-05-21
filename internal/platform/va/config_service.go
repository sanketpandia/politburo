package va

import (
	stdCtx "context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"log"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// Public "enum" — just string constants
///////////////////////////////////////////////////////////////////////////////

const (
	ConfigKeyIFServerID                   = "if_server_id"
	ConfigKeyEnabledServerIDs             = "enabled_server_ids"
	ConfigKeyTest                         = "test"
	ConfigKeyCallsignPrefix               = "callsign_prefix"
	ConfigKeyCallsignSuffix               = "callsign_suffix"
	ConfigKeyAirtableAPIKey               = "airtable_api_key"
	ConfigKeyAirtableVABase               = "airtable_va_base"
	ConfigKeyAirtableCallsignColumnPrefix = "airtable_callsign_col_prefix"

	// New table keys
	ConfigKeyATTablePilots = "at_table_pilots"
	ConfigKeyATTableRoutes = "at_table_routes"
	ConfigKeyATTablePIREPs = "at_table_pireps"

	// Field mapping keys
	ConfigKeyATFieldPilotsCallsign   = "at_field_pilots_callsign"
	ConfigKeyATFieldRoutesOrigin     = "at_field_routes_origin"
	ConfigKeyATFieldRoutesDest       = "at_field_routes_dest"
	ConfigKeyATFieldRoutesRoute      = "at_field_routes_route"
	ConfigKeyATFieldPIREPsCallsign   = "at_field_pireps_callsign"
	ConfigKeyATFieldPIREPsRoute      = "at_field_pireps_route"
	ConfigKeyATFieldPIREPsFlightTime = "at_field_pireps_ft"

	ConfigKeyATFieldLastModified = "at_field_last_modified"

	ConfigKeyTourFlightMode = "tour_flight_mode"

	// Default values for livery mappings
	ConfigKeyDefaultAircraft = "default_aircraft"
	ConfigKeyDefaultAirline  = "default_airline"
)

var AllowedVAConfigKeys = map[string]struct{}{
	ConfigKeyIFServerID:                   {},
	ConfigKeyEnabledServerIDs:             {},
	ConfigKeyTest:                         {},
	ConfigKeyCallsignPrefix:               {},
	ConfigKeyCallsignSuffix:               {},
	ConfigKeyAirtableAPIKey:               {},
	ConfigKeyAirtableVABase:               {},
	ConfigKeyATTablePilots:                {},
	ConfigKeyATTableRoutes:                {},
	ConfigKeyATTablePIREPs:                {},
	ConfigKeyATFieldPilotsCallsign:        {},
	ConfigKeyATFieldRoutesOrigin:          {},
	ConfigKeyATFieldRoutesDest:            {},
	ConfigKeyATFieldPIREPsCallsign:        {},
	ConfigKeyATFieldPIREPsRoute:           {},
	ConfigKeyATFieldPIREPsFlightTime:      {},
	ConfigKeyATFieldLastModified:          {},
	ConfigKeyATFieldRoutesRoute:           {},
	ConfigKeyAirtableCallsignColumnPrefix: {},
	ConfigKeyTourFlightMode:               {},
	ConfigKeyDefaultAircraft:              {},
	ConfigKeyDefaultAirline:               {},
}

func ListAllowedVAConfigKeys() []string {
	keys := make([]string, 0, len(AllowedVAConfigKeys))
	for k := range AllowedVAConfigKeys {
		keys = append(keys, k)
	}
	return keys
}

func IsValidVAConfigKey(k string) bool {
	_, ok := AllowedVAConfigKeys[k]
	return ok
}

func (s *ConfigService) SetConfigValue(ctx stdCtx.Context, vaID string, key string, value string) error {
	if !IsValidVAConfigKey(key) {
		return fmt.Errorf("%q is not a valid key", key)
	}
	if err := s.repo.UpsertVAConfig(ctx, vaID, key, value); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	s.cacheStore.Delete(configCacheKey(vaID))
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Service
///////////////////////////////////////////////////////////////////////////////

// ConfigService manages Virtual Airline configuration key-value pairs with caching support
type ConfigService struct {
	repo         *Repository
	cacheStore   cache.CacheInterface
	aircraftRepo *aircraft.Repository
	aircraftSvc  *aircraft.Service
}

// NewConfigService creates a new VA config service
func NewConfigService(r *Repository, c cache.CacheInterface, aircraftRepo *aircraft.Repository, aircraftSvc *aircraft.Service) *ConfigService {
	return &ConfigService{
		repo:         r,
		cacheStore:   c,
		aircraftRepo: aircraftRepo,
		aircraftSvc:  aircraftSvc,
	}
}

func configCacheKey(vaID string) string {
	return string(constants.CachePrefixVAConfig) + vaID
}

// Expose constants to API callers
func (s *ConfigService) ListPossibleKeys() []string { return ListAllowedVAConfigKeys() }

func (s *ConfigService) CacheStore() cache.CacheInterface { return s.cacheStore }

// ---------------------------------------------------------------------------
// Set VA config and return updated map
// ---------------------------------------------------------------------------
func (s *ConfigService) SetVaConfig(
	ctx stdCtx.Context,
	cfgs map[string]string,
) (*map[string]string, error) {

	claims := auth.GetUserClaims(ctx)
	fmt.Printf("Request Map: \n %v", cfgs)
	for key, value := range cfgs {

		if !IsValidVAConfigKey(key) {
			return nil, fmt.Errorf("%q is not a valid key", key)
		}

		va_id := claims.ServerID()

		// upsert
		if err := s.repo.UpsertVAConfig(ctx, va_id, key, value); err != nil {
			return nil, fmt.Errorf("failed to set config: %w", err)
		}
		cKey := configCacheKey(va_id)
		fmt.Printf("Evicting: %s", cKey)

		s.cacheStore.Delete(cKey)
	}

	cfgs, err := s.GetAllConfigValues(ctx, claims.ServerID())
	if err != nil {
		return nil, err
	}
	return &cfgs, nil
}

// ---------------------------------------------------------------------------
// Get *all* values (cached)             map[string]string
// ---------------------------------------------------------------------------
func (s *ConfigService) GetAllConfigValues(
	ctx stdCtx.Context,
	vaID string,
) (map[string]string, error) {

	cacheKey := configCacheKey(vaID)

	val, err := s.cacheStore.GetOrSet(cacheKey, 10*time.Minute, func() (any, error) {
		rows, err := s.repo.GetVAConfigs(ctx, vaID)
		if err != nil {
			return nil, err
		}
		m := make(map[string]string, len(rows))
		for _, r := range rows {
			m[r.ConfigKey] = r.ConfigValue
		}

		return m, nil
	})
	if err != nil {
		return nil, err
	}

	// Handle both map[string]string (from loader) and map[string]any (from JSON unmarshal)
	switch v := val.(type) {
	case map[string]string:
		return v, nil
	case map[string]any:
		// Convert map[string]any to map[string]string
		cfgs := make(map[string]string, len(v))
		for key, value := range v {
			if strVal, ok := value.(string); ok {
				cfgs[key] = strVal
			} else {
				return nil, fmt.Errorf("value for key %q is not a string", key)
			}
		}
		return cfgs, nil
	default:
		return nil, fmt.Errorf("cache type assertion failed: expected map[string]string or map[string]any, got %T", val)
	}
}

// ---------------------------------------------------------------------------
// Get single value
// ---------------------------------------------------------------------------
func (s *ConfigService) GetConfigVal(
	ctx stdCtx.Context,
	vaID string,
	key string, // callers import ConfigKeyIFServerID etc.
) (string, bool) {

	if !IsValidVAConfigKey(key) {
		log.Printf("\nUnable to fetch VA Config: %s", key)
		return "", false
	}

	cfgs, err := s.GetAllConfigValues(ctx, vaID)
	if err != nil {
		log.Printf("\nError: %v", err)

		return "", false
	}
	log.Printf("\nConfigs: %v", cfgs)
	return cfgs[key], true
}

func (s *ConfigService) GetConfigValues(
	ctx stdCtx.Context,
	vaID string,
	keys []string, // callers import ConfigKeyIFServerID etc.
) (map[string]string, bool) {

	conf := make(map[string]string, len(keys))
	cfgs, err := s.GetAllConfigValues(ctx, vaID)

	if err != nil {
		return conf, false
	}

	for _, key := range keys {
		if !IsValidVAConfigKey(key) {
			return conf, false
		}
		val, ok := cfgs[key]
		if ok {
			conf[key] = val
		} else {
			conf[key] = ""
		}
	}

	return conf, true
}

// GetAllCallsigns retrieves callsign prefix/suffix configuration for all active VAs
func (s *ConfigService) GetAllCallsigns(ctx stdCtx.Context) ([]map[string]string, error) {
	cacheKey := string(constants.CachePrefixVAConfig) + "all_callsigns"

	val, err := s.cacheStore.GetOrSet(cacheKey, 10*time.Minute, func() (any, error) {
		return s.repo.GetAllActiveVACallsignConfigs(ctx)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get all callsigns: %w", err)
	}

	// Handle type conversion from cache
	switch v := val.(type) {
	case []map[string]string:
		// Direct type match (from loader function)
		return v, nil
	case []interface{}:
		// Cache deserialized as []interface{}, convert each element
		result := make([]map[string]string, 0, len(v))
		for i, item := range v {
			switch m := item.(type) {
			case map[string]string:
				result = append(result, m)
			case map[string]interface{}:
				// Convert map[string]interface{} to map[string]string
				converted := make(map[string]string)
				for key, value := range m {
					if strVal, ok := value.(string); ok {
						converted[key] = strVal
					} else {
						return nil, fmt.Errorf("value for key %q at index %d is not a string", key, i)
					}
				}
				result = append(result, converted)
			default:
				return nil, fmt.Errorf("unexpected map type at index %d: %T", i, item)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("cache type assertion failed: expected []map[string]string or []interface{}, got %T", val)
	}
}

// In-game suffixes that should be stripped before VA suffix matching
var gameCallsignSuffixes = []string{
	"HEAVY",
	"SUPER",
	"FLIGHT OF 1",
	"FLIGHT OF 2",
	"FLIGHT OF 3",
	"FLIGHT OF 4",
	"FLIGHT OF 5",
	"FLIGHT OF 6",
	"FLIGHT OF 7",
	"FLIGHT OF 8",
	"FLIGHT OF 9",
	"FLIGHT OF 10",
}

// MatchesVAPattern checks if a callsign matches a specific VA prefix/suffix pattern
func MatchesVAPattern(callsign string, prefix string, suffix string) bool {
	callsignUpper := strings.ToUpper(strings.TrimSpace(callsign))

	// Check prefix match
	if prefix != "" {
		prefixUpper := strings.ToUpper(strings.TrimSpace(prefix))
		if !strings.HasPrefix(callsignUpper, prefixUpper) {
			return false
		}
	}

	// Check suffix match
	if suffix != "" {
		// Strip in-game suffixes before checking VA suffix
		strippedCallsign := stripGameSuffixes(callsignUpper)

		suffixUpper := strings.ToUpper(strings.TrimSpace(suffix))
		// Use HasSuffix for accurate matching
		if !strings.HasSuffix(strippedCallsign, suffixUpper) {
			return false
		}
	}

	// If we got here, all specified patterns match
	return true
}

// stripGameSuffixes removes in-game callsign suffixes (HEAVY, SUPER, FLIGHT OF X)
func stripGameSuffixes(callsign string) string {
	trimmed := strings.TrimSpace(callsign)

	for _, gameSuffix := range gameCallsignSuffixes {
		if strings.HasSuffix(trimmed, gameSuffix) {
			// Remove the game suffix and trim any trailing spaces
			trimmed = strings.TrimSpace(trimmed[:len(trimmed)-len(gameSuffix)])
			break // Only remove one game suffix
		}
	}

	return trimmed
}

// ResolveAircraftName resolves the aircraft name for Airtable using livery mappings
// Returns the mapped value if a mapping exists, otherwise returns the original aircraft name
func (s *ConfigService) ResolveAircraftName(ctx stdCtx.Context, vaID, liveryID string) string {
	if liveryID == "" {
		return ""
	}

	// Get aircraft name from livery
	liveryData := s.aircraftSvc.GetAircraftLivery(ctx, liveryID)
	if liveryData == nil {
		return ""
	}

	aircraftName := liveryData.AircraftName
	if aircraftName == "" {
		return ""
	}

	// Check for mapping
	mappings, err := s.aircraftRepo.GetMappingsByLivery(ctx, vaID, liveryID)
	if err != nil {
		// Log but don't fail - return original name
		log.Printf("[ConfigService] Error fetching aircraft mapping for va_id=%s, livery_id=%s: %v", vaID, liveryID, err)
		return aircraftName
	}

	// If mapping exists for aircraft, use it
	if mappedAircraft, ok := mappings["aircraft"]; ok && mappedAircraft != "" {
		log.Printf("[ConfigService] Using mapped aircraft for va_id=%s, livery_id=%s: %s", vaID, liveryID, mappedAircraft)
		return mappedAircraft
	}

	// No mapping found - check for default aircraft value
	if defaultAircraft, ok := s.GetConfigVal(ctx, vaID, ConfigKeyDefaultAircraft); ok && defaultAircraft != "" {
		log.Printf("[ConfigService] No mapping found, using default aircraft for va_id=%s, livery_id=%s: %s (original: %s)", vaID, liveryID, defaultAircraft, aircraftName)
		return defaultAircraft
	}

	// Fallback to original aircraft name
	log.Printf("[ConfigService] No mapping or default found, using original aircraft name for va_id=%s, livery_id=%s: %s", vaID, liveryID, aircraftName)
	return aircraftName
}

// ResolveLiveryName resolves the livery/airline name for Airtable using livery mappings
// Returns the mapped value if a mapping exists, otherwise returns the original livery name
func (s *ConfigService) ResolveLiveryName(ctx stdCtx.Context, vaID, liveryID string) string {
	if liveryID == "" {
		return ""
	}

	// Get livery name from livery
	liveryData := s.aircraftSvc.GetAircraftLivery(ctx, liveryID)
	if liveryData == nil {
		return ""
	}

	liveryName := liveryData.LiveryName
	if liveryName == "" {
		return ""
	}

	// Check for mapping
	mappings, err := s.aircraftRepo.GetMappingsByLivery(ctx, vaID, liveryID)
	if err != nil {
		// Log but don't fail - return original name
		log.Printf("[ConfigService] Error fetching livery mapping for va_id=%s, livery_id=%s: %v", vaID, liveryID, err)
		return liveryName
	}

	// If mapping exists for airline, use it
	if mappedAirline, ok := mappings["airline"]; ok && mappedAirline != "" {
		log.Printf("[ConfigService] Using mapped airline for va_id=%s, livery_id=%s: %s", vaID, liveryID, mappedAirline)
		return mappedAirline
	}

	// No mapping found - check for default airline value
	if defaultAirline, ok := s.GetConfigVal(ctx, vaID, ConfigKeyDefaultAirline); ok && defaultAirline != "" {
		log.Printf("[ConfigService] No mapping found, using default airline for va_id=%s, livery_id=%s: %s (original: %s)", vaID, liveryID, defaultAirline, liveryName)
		return defaultAirline
	}

	// Fallback to original livery name
	log.Printf("[ConfigService] No mapping or default found, using original livery name for va_id=%s, livery_id=%s: %s", vaID, liveryID, liveryName)
	return liveryName
}

// ResolvedValue represents a resolved value with metadata about how it was resolved
type ResolvedValue struct {
	Value         string
	UsedDefault   bool
	OriginalValue string
}

// ResolveAircraftNameWithMetadata resolves aircraft name and returns metadata about resolution
func (s *ConfigService) ResolveAircraftNameWithMetadata(ctx stdCtx.Context, vaID, liveryID string) ResolvedValue {
	if liveryID == "" {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	// Get aircraft name from livery
	liveryData := s.aircraftSvc.GetAircraftLivery(ctx, liveryID)
	if liveryData == nil {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	aircraftName := liveryData.AircraftName
	if aircraftName == "" {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	// Check for mapping
	mappings, err := s.aircraftRepo.GetMappingsByLivery(ctx, vaID, liveryID)
	if err != nil {
		log.Printf("[ConfigService] Error fetching aircraft mapping for va_id=%s, livery_id=%s: %v", vaID, liveryID, err)
		return ResolvedValue{Value: aircraftName, UsedDefault: false, OriginalValue: aircraftName}
	}

	// If mapping exists for aircraft, use it
	if mappedAircraft, ok := mappings["aircraft"]; ok && mappedAircraft != "" {
		log.Printf("[ConfigService] Using mapped aircraft for va_id=%s, livery_id=%s: %s", vaID, liveryID, mappedAircraft)
		return ResolvedValue{Value: mappedAircraft, UsedDefault: false, OriginalValue: aircraftName}
	}

	// No mapping found - check for default aircraft value
	if defaultAircraft, ok := s.GetConfigVal(ctx, vaID, ConfigKeyDefaultAircraft); ok && defaultAircraft != "" {
		log.Printf("[ConfigService] No mapping found, using default aircraft for va_id=%s, livery_id=%s: %s (original: %s)", vaID, liveryID, defaultAircraft, aircraftName)
		return ResolvedValue{Value: defaultAircraft, UsedDefault: true, OriginalValue: aircraftName}
	}

	// Fallback to original aircraft name
	log.Printf("[ConfigService] No mapping or default found, using original aircraft name for va_id=%s, livery_id=%s: %s", vaID, liveryID, aircraftName)
	return ResolvedValue{Value: aircraftName, UsedDefault: false, OriginalValue: aircraftName}
}

// ResolveLiveryNameWithMetadata resolves livery/airline name and returns metadata about resolution
func (s *ConfigService) ResolveLiveryNameWithMetadata(ctx stdCtx.Context, vaID, liveryID string) ResolvedValue {
	if liveryID == "" {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	// Get livery name from livery
	liveryData := s.aircraftSvc.GetAircraftLivery(ctx, liveryID)
	if liveryData == nil {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	liveryName := liveryData.LiveryName
	if liveryName == "" {
		return ResolvedValue{Value: "", UsedDefault: false, OriginalValue: ""}
	}

	// Check for mapping
	mappings, err := s.aircraftRepo.GetMappingsByLivery(ctx, vaID, liveryID)
	if err != nil {
		log.Printf("[ConfigService] Error fetching livery mapping for va_id=%s, livery_id=%s: %v", vaID, liveryID, err)
		return ResolvedValue{Value: liveryName, UsedDefault: false, OriginalValue: liveryName}
	}

	// If mapping exists for airline, use it
	if mappedAirline, ok := mappings["airline"]; ok && mappedAirline != "" {
		log.Printf("[ConfigService] Using mapped airline for va_id=%s, livery_id=%s: %s", vaID, liveryID, mappedAirline)
		return ResolvedValue{Value: mappedAirline, UsedDefault: false, OriginalValue: liveryName}
	}

	// No mapping found - check for default airline value
	if defaultAirline, ok := s.GetConfigVal(ctx, vaID, ConfigKeyDefaultAirline); ok && defaultAirline != "" {
		log.Printf("[ConfigService] No mapping found, using default airline for va_id=%s, livery_id=%s: %s (original: %s)", vaID, liveryID, defaultAirline, liveryName)
		return ResolvedValue{Value: defaultAirline, UsedDefault: true, OriginalValue: liveryName}
	}

	// Fallback to original livery name
	log.Printf("[ConfigService] No mapping or default found, using original livery name for va_id=%s, livery_id=%s: %s", vaID, liveryID, liveryName)
	return ResolvedValue{Value: liveryName, UsedDefault: false, OriginalValue: liveryName}
}
