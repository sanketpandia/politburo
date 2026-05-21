package flights

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/platform/va"
)

const vaPatternRefreshInterval = 5 * time.Minute

// refreshVAPatterns loads active VA callsign patterns. It does not require Airtable config.
func (j *CacheJob) refreshVAPatterns(ctx context.Context) error {
	if time.Since(j.lastPatternUpdate) < vaPatternRefreshInterval {
		return nil
	}

	configs, err := j.vaRepo.GetAllActiveVACallsignConfigs(ctx)
	if err != nil {
		j.recordCacheJobFailure("va_pattern_refresh")
		return fmt.Errorf("failed to fetch VA callsign configs: %w", err)
	}

	patterns := make([]VAPattern, 0, len(configs))
	for _, config := range configs {
		vaID, ok := config["va_id"]
		if !ok {
			continue
		}
		patterns = append(patterns, VAPattern{
			VAID:   vaID,
			Prefix: config[va.ConfigKeyCallsignPrefix],
			Suffix: config[va.ConfigKeyCallsignSuffix],
		})
	}

	j.vaPatterns = patterns
	j.lastPatternUpdate = time.Now()
	logging.Info("Refreshed VA callsign patterns", "count", len(patterns))
	return nil
}

func (j *CacheJob) matchFlightToVAs(callsign string) []string {
	if len(j.vaPatterns) == 0 {
		return nil
	}

	matchedVAs := make([]string, 0, 1)
	for _, pattern := range j.vaPatterns {
		if pattern.Prefix == "" && pattern.Suffix == "" {
			continue
		}
		if va.MatchesVAPattern(callsign, pattern.Prefix, pattern.Suffix) {
			matchedVAs = append(matchedVAs, pattern.VAID)
		}
	}
	return matchedVAs
}

func (j *CacheJob) filterVAsForSession(ctx context.Context, vaIDs []string, sessionID string) []string {
	if len(vaIDs) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(vaIDs))
	for _, vaID := range vaIDs {
		if j.vaSessionEnabled(ctx, vaID, sessionID) {
			filtered = append(filtered, vaID)
		}
	}
	return filtered
}

func (j *CacheJob) vaSessionEnabled(ctx context.Context, vaID string, sessionID string) bool {
	configs, err := j.vaRepo.GetVAConfigs(ctx, vaID)
	if err != nil {
		j.recordCacheJobFailure("va_enabled_server_config")
		logging.Warn("Failed to load VA enabled server config", "error", err)
		return true
	}

	for _, cfg := range configs {
		if cfg.ConfigKey != va.ConfigKeyEnabledServerIDs {
			continue
		}
		ids := parseServerIDConfig(cfg.ConfigValue)
		if len(ids) == 0 {
			return true
		}
		for _, id := range ids {
			if id == sessionID {
				return true
			}
		}
		return false
	}
	return true
}

func parseServerIDConfig(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return nil
	}

	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err == nil {
		return compactServerIDs(ids)
	}
	return compactServerIDs(strings.Split(value, "|"))
}

func compactServerIDs(values []string) []string {
	ids := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	return ids
}
