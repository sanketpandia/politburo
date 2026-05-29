package dtos

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	FlightModesConfigVersionV2 = 2

	DetectionModeRequired = "required"
	DetectionModeOptional = "optional"
	DetectionModeNotUsed  = "not_used"

	RouteSourceCurrentFPL = "current_fpl"
	RouteSourceFixedRoute = "fixed_route"
	RouteSourceNone       = "none"

	ModeStatusValid   = "valid"
	ModeStatusInvalid = "invalid"
)

type ModeRuntimeEnvelope struct {
	ConfigVersion int                          `json:"config_version"`
	FlightModes   map[string]ModeRuntimeConfig `json:"flight_modes"`
}

type ModeRuntimeConfig struct {
	Identity        ModeIdentity      `json:"identity"`
	FlightDetection FlightDetection   `json:"flight_detection"`
	RouteBehavior   RouteBehavior     `json:"route_behavior"`
	PilotInputs     []ModePilotInput  `json:"pilot_inputs"`
	ModeValidations map[string]any    `json:"mode_validations,omitempty"`
	AirtableMapping map[string]string `json:"airtable_mapping,omitempty"`
}

type ModeIdentity struct {
	DisplayName string `json:"display_name"`
	InternalKey string `json:"internal_key"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order,omitempty"`
}

type FlightDetection struct {
	DetectionMode       string `json:"detection_mode"`
	RequireActiveFlight bool   `json:"require_active_flight"`
}

type RouteBehavior struct {
	RouteSource string              `json:"route_source"`
	FixedRoute  *RouteBehaviorFixed `json:"fixed_route,omitempty"`
}

type RouteBehaviorFixed struct {
	RouteName  string  `json:"route_name"`
	Multiplier float64 `json:"multiplier,omitempty"`
}

type ModePilotInput struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	HelpText    string `json:"help_text,omitempty"`
}

func ParseModeRuntimeEnvelope(raw map[string]interface{}) (*ModeRuntimeEnvelope, error) {
	if raw == nil {
		return nil, fmt.Errorf("flight modes config is empty")
	}

	version, ok := raw["config_version"]
	if !ok {
		return nil, fmt.Errorf("config_version is required")
	}

	parsedVersion := 0
	switch v := version.(type) {
	case float64:
		parsedVersion = int(v)
	case int:
		parsedVersion = v
	default:
		return nil, fmt.Errorf("config_version must be numeric")
	}

	if parsedVersion != FlightModesConfigVersionV2 {
		return nil, fmt.Errorf("unsupported config_version: %d", parsedVersion)
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal flight modes config: %w", err)
	}

	var envelope ModeRuntimeEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("parse flight modes config: %w", err)
	}

	if err := ValidateModeRuntimeEnvelope(&envelope); err != nil {
		return nil, err
	}

	return &envelope, nil
}

func ValidateModeRuntimeEnvelope(envelope *ModeRuntimeEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("flight modes config is empty")
	}
	if envelope.ConfigVersion != FlightModesConfigVersionV2 {
		return fmt.Errorf("unsupported config_version: %d", envelope.ConfigVersion)
	}
	if len(envelope.FlightModes) == 0 {
		return fmt.Errorf("flight_modes must contain at least one mode")
	}

	for key, mode := range envelope.FlightModes {
		if strings.TrimSpace(mode.Identity.DisplayName) == "" {
			return fmt.Errorf("mode %q missing identity.display_name", key)
		}
		if strings.TrimSpace(mode.Identity.InternalKey) == "" {
			return fmt.Errorf("mode %q missing identity.internal_key", key)
		}
		if mode.Identity.InternalKey != key {
			return fmt.Errorf("mode %q identity.internal_key mismatch", key)
		}
		switch mode.FlightDetection.DetectionMode {
		case DetectionModeRequired, DetectionModeOptional, DetectionModeNotUsed:
		default:
			return fmt.Errorf("mode %q has invalid flight_detection.detection_mode", key)
		}
		switch mode.RouteBehavior.RouteSource {
		case RouteSourceCurrentFPL, RouteSourceFixedRoute, RouteSourceNone:
		default:
			return fmt.Errorf("mode %q has invalid route_behavior.route_source", key)
		}
		if mode.RouteBehavior.RouteSource == RouteSourceFixedRoute {
			if mode.RouteBehavior.FixedRoute == nil || strings.TrimSpace(mode.RouteBehavior.FixedRoute.RouteName) == "" {
				return fmt.Errorf("mode %q fixed_route requires route_name", key)
			}
		}
		if len(mode.PilotInputs) > 5 {
			return fmt.Errorf("mode %q exceeds Discord modal max fields (5)", key)
		}
	}

	return nil
}

func SortedModeKeysByDisplayName(modes map[string]ModeRuntimeConfig) []string {
	keys := make([]string, 0, len(modes))
	for key := range modes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(modes[keys[i]].Identity.DisplayName))
		right := strings.ToLower(strings.TrimSpace(modes[keys[j]].Identity.DisplayName))
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	return keys
}
