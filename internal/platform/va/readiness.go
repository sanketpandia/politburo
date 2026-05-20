package va

import (
	"context"
	"strings"
)

type SetupReadiness struct {
	BootstrapCreated    bool
	FlightMatchingReady bool
	CallsignPrefix      string
	CallsignSuffix      string
}

func (s *ConfigService) ComputeSetupReadiness(ctx context.Context, vaID string) (*SetupReadiness, error) {
	configs, err := s.GetAllConfigValues(ctx, vaID)
	if err != nil {
		return nil, err
	}

	prefix := strings.TrimSpace(configs[ConfigKeyCallsignPrefix])
	suffix := strings.TrimSpace(configs[ConfigKeyCallsignSuffix])

	return &SetupReadiness{
		BootstrapCreated:    vaID != "",
		FlightMatchingReady: prefix != "" || suffix != "",
		CallsignPrefix:      prefix,
		CallsignSuffix:      suffix,
	}, nil
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
