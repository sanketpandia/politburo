package flights

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	"infinite-experiment/politburo/internal/infiniteflight"
)

func TestMapFlightNormalizesAndEnriches(t *testing.T) {
	username := "Hantder_Broncano_Jar"
	org := "IFAET"
	fallback := time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC)
	flight := MapFlight(infiniteflight.Flight{
		Username:            &username,
		Callsign:            "Swiss 39 Heavy",
		Latitude:            63.042991638183594,
		Longitude:           -30.311138153076172,
		Altitude:            35999.953,
		Speed:               525.6486,
		VerticalSpeed:       0.00010479759,
		Track:               106.800766,
		Heading:             106.800766,
		LastReport:          "2026-08-15 05:09:53Z",
		FlightID:            "c34118e7-cbdd-4e22-8751-0cda93e41d75",
		UserID:              "0d85b360-92b5-4e62-ac64-41bd1c829772",
		AircraftID:          "e258f6d4-4503-4dde-b25c-1fb9067061e2",
		LiveryID:            "df597aaf-456c-4878-9d84-45201f2aae74",
		VirtualOrganization: &org,
		PilotState:          PilotStateInBackground,
		IsConnected:         false,
	}, infiniteflight.Session{ID: "session-1", NormalizedName: "casual"}, &gameliveries.Livery{
		AircraftName: "Airbus A350", LiveryName: "Swiss",
	}, nil, fallback)

	if flight.Altitude != 36000 || flight.Speed != 526 || flight.VerticalSpeed != 0 {
		t.Fatalf("numeric normalization = alt %d speed %d vs %v", flight.Altitude, flight.Speed, flight.VerticalSpeed)
	}
	if flight.Latitude != math.Round(63.042991638183594*10000)/10000 {
		t.Fatalf("latitude = %v", flight.Latitude)
	}
	if flight.Normalized.Speed != "526 kts" || flight.Normalized.VerticalSpeed != "0.0 ft/min" {
		t.Fatalf("normalized units = %#v", flight.Normalized)
	}
	if flight.Normalized.PilotState != PilotStateNameInBackground || flight.Normalized.IsConnected != "disconnected" {
		t.Fatalf("normalized enums = %#v", flight.Normalized)
	}
	if flight.AircraftName != "Airbus A350" || flight.LiveryName != "Swiss" {
		t.Fatalf("enrichment = %#v", flight)
	}
	if flight.PathSync == nil || flight.PathSync.FPLSyncRequired {
		t.Fatalf("pathSync = %#v", flight.PathSync)
	}
	if !flight.LastReport.Equal(time.Date(2026, time.August, 15, 5, 9, 53, 0, time.UTC)) {
		t.Fatalf("lastReport = %s", flight.LastReport)
	}
}

func TestMapFlightPreservesNamesOnLiveryMiss(t *testing.T) {
	existing := &Flight{AircraftName: "A320", LiveryName: "BA"}
	flight := MapFlight(infiniteflight.Flight{FlightID: "f1", LastReport: "not-a-time"}, infiniteflight.Session{NormalizedName: "expert"}, nil, existing, time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC))
	if flight.AircraftName != "A320" || flight.LiveryName != "BA" {
		t.Fatalf("preserved names = %#v", flight)
	}
	if !flight.LastReport.Equal(time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("fallback lastReport = %s", flight.LastReport)
	}
}

func TestUpsertFlightsCreatesMissingEntries(t *testing.T) {
	result := UpsertFlights(nil, []Flight{{FlightID: "f1", Callsign: "new", History: []Flight{{Callsign: "leak"}}, PathSync: &PathSync{}}})
	if len(result) != 1 || result[0].Callsign != "new" || result[0].History != nil {
		t.Fatalf("created = %#v", result)
	}
}

func TestUpsertFlightsPreservesNamesWithoutEmbeddingHistory(t *testing.T) {
	existing := []Flight{{FlightID: "f1", Callsign: "prior", AircraftName: "A320", LiveryName: "BA", History: []Flight{{Callsign: "older"}}, PathSync: &PathSync{FPLSyncRequired: false}}}
	mapped := []Flight{{FlightID: "f1", Callsign: "current", PathSync: &PathSync{FPLSyncRequired: false}}}
	result := UpsertFlights(existing, mapped)
	if len(result) != 1 || result[0].AircraftName != "A320" || result[0].LiveryName != "BA" || result[0].History != nil {
		t.Fatalf("upserted = %#v", result[0])
	}
}

func TestNextHistoryCapsAtTwentyFive(t *testing.T) {
	history := make([]Flight, MaxHistory)
	for i := range history {
		history[i].Callsign = "old"
		history[i].Speed = i
	}
	prior := Flight{FlightID: "f1", Callsign: "prior", Speed: 400, History: history, PathSync: &PathSync{FPLSyncRequired: false}}
	result := NextHistory(history, prior)
	if len(result) != MaxHistory {
		t.Fatalf("history length = %d", len(result))
	}
	if result[MaxHistory-1].Callsign != "prior" || result[MaxHistory-1].History != nil || result[MaxHistory-1].PathSync != nil {
		t.Fatalf("newest history = %#v", result[MaxHistory-1])
	}
	if result[0].Speed != 1 {
		t.Fatalf("oldest retained speed = %d, want 1", result[0].Speed)
	}
}

func TestFlightHistoryOmitsNestedPathSync(t *testing.T) {
	result := NextHistory(nil, Flight{FlightID: "f1", Callsign: "prior", PathSync: &PathSync{FPLSyncRequired: false}})
	encoded, err := json.Marshal(result[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(encoded); strings.Contains(got, `"history"`) || strings.Contains(got, `"pathSync"`) {
		t.Fatalf("historical JSON = %s", encoded)
	}
}
