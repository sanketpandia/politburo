package common

import (
	stdCtx "context"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"log"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// Public “enum” — just string constants
///////////////////////////////////////////////////////////////////////////////

const (
	ConfigKeyIFServerID                   = "if_server_id"
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
)

var AllowedVAConfigKeys = map[string]struct{}{
	ConfigKeyIFServerID:                   {},
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
}

func ListAllowedVAConfigKeys() []string { return GetKeysStructMap(AllowedVAConfigKeys) }

func IsValidVAConfigKey(k string) bool {
	_, ok := AllowedVAConfigKeys[k]
	return ok
}

///////////////////////////////////////////////////////////////////////////////
// Service
///////////////////////////////////////////////////////////////////////////////

type VAConfigService struct {
	repo  *repositories.VAGormRepository
	cache CacheInterface
}

func NewVAConfigService(r *repositories.VAGormRepository, c CacheInterface) *VAConfigService {
	return &VAConfigService{repo: r, cache: c}
}

func configCacheKey(vaID string) string {
	return string(constants.CachePrefixVAConfig) + vaID
}

// Expose constants to API callers
func (s *VAConfigService) ListPossibleKeys() []string { return ListAllowedVAConfigKeys() }

// ---------------------------------------------------------------------------
// Set VA config and return updated map
// ---------------------------------------------------------------------------
func (s *VAConfigService) SetVaConfig(
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

		s.cache.Delete(cKey)
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
func (s *VAConfigService) GetAllConfigValues(
	ctx stdCtx.Context,
	vaID string,
) (map[string]string, error) {

	cacheKey := configCacheKey(vaID)

	val, err := s.cache.GetOrSet(cacheKey, 10*time.Minute, func() (any, error) {
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
func (s *VAConfigService) GetConfigVal(
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

func (s *VAConfigService) GetConfigValues(
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
func (s *VAConfigService) GetAllCallsigns(ctx stdCtx.Context) ([]map[string]string, error) {
	cacheKey := string(constants.CachePrefixVAConfig) + "all_callsigns"

	val, err := s.cache.GetOrSet(cacheKey, 10*time.Minute, func() (any, error) {
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
