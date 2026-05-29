package vaadmin

import (
	"testing"

	"infinite-experiment/politburo/internal/models/dtos"
)

func TestBuildModeSectionStatus_FixedRouteMissingNameNeedsSetup(t *testing.T) {
	mode := dtos.ModeRuntimeConfig{
		Identity:        dtos.ModeIdentity{DisplayName: "Mode", InternalKey: "mode", Enabled: true},
		FlightDetection: dtos.FlightDetection{DetectionMode: dtos.DetectionModeRequired, RequireActiveFlight: true},
		RouteBehavior: dtos.RouteBehavior{
			RouteSource: dtos.RouteSourceFixedRoute,
			FixedRoute:  &dtos.RouteBehaviorFixed{RouteName: ""},
		},
		PilotInputs: []dtos.ModePilotInput{{Key: "flight_time", Label: "Flight Time", Type: "text", Required: true}},
	}

	status := buildModeSectionStatus(mode)
	if status["route_behavior"] != "Needs setup" {
		t.Fatalf("expected route_behavior Needs setup, got %q", status["route_behavior"])
	}
}

func TestBuildModeCards_ParsesAndOrdersByDisplayName(t *testing.T) {
	config := map[string]interface{}{
		"config_version": float64(2),
		"flight_modes": map[string]interface{}{
			"b": map[string]interface{}{
				"identity": map[string]interface{}{"display_name": "Zulu", "internal_key": "b", "enabled": true},
				"flight_detection": map[string]interface{}{"detection_mode": "required", "require_active_flight": true},
				"route_behavior":   map[string]interface{}{"route_source": "none"},
				"pilot_inputs":     []interface{}{},
			},
			"a": map[string]interface{}{
				"identity": map[string]interface{}{"display_name": "Alpha", "internal_key": "a", "enabled": true},
				"flight_detection": map[string]interface{}{"detection_mode": "required", "require_active_flight": true},
				"route_behavior":   map[string]interface{}{"route_source": "none"},
				"pilot_inputs":     []interface{}{},
			},
		},
	}

	cards, err := buildModeCards(config)
	if err != nil {
		t.Fatalf("unexpected buildModeCards error: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	if cards[0]["mode_id"] != "a" {
		t.Fatalf("expected first card mode_id 'a', got %#v", cards[0]["mode_id"])
	}
}
