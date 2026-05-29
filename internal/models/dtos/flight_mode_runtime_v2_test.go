package dtos

import "testing"

func TestParseModeRuntimeEnvelope_RejectsLegacyPayload(t *testing.T) {
	raw := map[string]interface{}{
		"flight_modes": map[string]interface{}{},
	}

	if _, err := ParseModeRuntimeEnvelope(raw); err == nil {
		t.Fatal("expected error for missing config_version")
	}
}

func TestParseModeRuntimeEnvelope_AcceptsStrictV2(t *testing.T) {
	raw := map[string]interface{}{
		"config_version": float64(2),
		"flight_modes": map[string]interface{}{
			"classic": map[string]interface{}{
				"identity": map[string]interface{}{
					"display_name": "Classic",
					"internal_key": "classic",
					"enabled":      true,
				},
				"flight_detection": map[string]interface{}{
					"detection_mode":        DetectionModeRequired,
					"require_active_flight": true,
				},
				"route_behavior": map[string]interface{}{
					"route_source": RouteSourceCurrentFPL,
				},
				"pilot_inputs": []interface{}{
					map[string]interface{}{"key": "flight_time", "label": "Flight Time", "type": "text", "required": true},
				},
			},
		},
	}

	env, err := ParseModeRuntimeEnvelope(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if env.ConfigVersion != FlightModesConfigVersionV2 {
		t.Fatalf("expected config version 2, got %d", env.ConfigVersion)
	}
}

func TestValidateModeRuntimeEnvelope_RejectsTooManyModalInputs(t *testing.T) {
	mode := ModeRuntimeConfig{
		Identity: ModeIdentity{DisplayName: "Mode", InternalKey: "mode", Enabled: true},
		FlightDetection: FlightDetection{DetectionMode: DetectionModeRequired, RequireActiveFlight: true},
		RouteBehavior: RouteBehavior{RouteSource: RouteSourceCurrentFPL},
		PilotInputs: []ModePilotInput{
			{Key: "a", Label: "a", Type: "text", Required: true},
			{Key: "b", Label: "b", Type: "text", Required: true},
			{Key: "c", Label: "c", Type: "text", Required: true},
			{Key: "d", Label: "d", Type: "text", Required: true},
			{Key: "e", Label: "e", Type: "text", Required: true},
			{Key: "f", Label: "f", Type: "text", Required: true},
		},
	}

	err := ValidateModeRuntimeEnvelope(&ModeRuntimeEnvelope{
		ConfigVersion: FlightModesConfigVersionV2,
		FlightModes:   map[string]ModeRuntimeConfig{"mode": mode},
	})

	if err == nil {
		t.Fatal("expected validation error for modal field overflow")
	}
}
