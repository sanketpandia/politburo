package va

import (
	"context"
	"encoding/json"
	"strings"
)

type SetupReadiness struct {
	BootstrapCreated    bool
	FlightMatchingReady bool
	CallsignPrefix      string
	CallsignSuffix      string
	EnabledServerIDs    []string
}

func (s *ConfigService) ComputeSetupReadiness(ctx context.Context, vaID string) (*SetupReadiness, error) {
	configs, err := s.GetAllConfigValues(ctx, vaID)
	if err != nil {
		return nil, err
	}

	prefix := strings.TrimSpace(configs[ConfigKeyCallsignPrefix])
	suffix := strings.TrimSpace(configs[ConfigKeyCallsignSuffix])
	enabledServerIDs := parseEnabledServerIDs(configs[ConfigKeyEnabledServerIDs])

	return &SetupReadiness{
		BootstrapCreated:    vaID != "",
		FlightMatchingReady: prefix != "" || suffix != "",
		CallsignPrefix:      prefix,
		CallsignSuffix:      suffix,
		EnabledServerIDs:    enabledServerIDs,
	}, nil
}

func parseEnabledServerIDs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err == nil {
		return compactStrings(ids)
	}
	return compactStrings(strings.Split(value, "|"))
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
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
		result = append(result, value)
	}
	return result
}

func CallsignMatches(sample string, prefix string, suffix string) bool {
	sample = strings.ToUpper(strings.TrimSpace(sample))
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	suffix = strings.ToUpper(strings.TrimSpace(suffix))

	if sample == "" || (prefix == "" && suffix == "") {
		return false
	}
	if prefix != "" && !strings.HasPrefix(sample, prefix) {
		return false
	}
	if suffix != "" && !strings.HasSuffix(sample, suffix) {
		return false
	}
	return true
}
