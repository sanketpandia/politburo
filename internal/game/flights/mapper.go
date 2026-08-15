package flights

import (
	"fmt"
	"math"
	"time"

	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	"infinite-experiment/politburo/internal/infiniteflight"
)

func MapFlight(upstream infiniteflight.Flight, session infiniteflight.Session, livery *gameliveries.Livery, existing *Flight, fallbackReport time.Time) Flight {
	speed := int(math.Round(upstream.Speed))
	verticalSpeed := math.Round(upstream.VerticalSpeed*10) / 10
	connected := "disconnected"
	if upstream.IsConnected {
		connected = "connected"
	}
	aircraftName := ""
	liveryName := ""
	if livery != nil {
		aircraftName = livery.AircraftName
		liveryName = livery.LiveryName
	}
	if existing != nil {
		if aircraftName == "" {
			aircraftName = existing.AircraftName
		}
		if liveryName == "" {
			liveryName = existing.LiveryName
		}
	}

	flight := Flight{
		FlightID:            upstream.FlightID,
		UserID:              upstream.UserID,
		AircraftID:          upstream.AircraftID,
		LiveryID:            upstream.LiveryID,
		Username:            upstream.Username,
		VirtualOrganization: upstream.VirtualOrganization,
		Callsign:            upstream.Callsign,
		Latitude:            math.Round(upstream.Latitude*10000) / 10000,
		Longitude:           math.Round(upstream.Longitude*10000) / 10000,
		Altitude:            int(math.Round(upstream.Altitude)),
		Speed:               speed,
		VerticalSpeed:       verticalSpeed,
		Track:               math.Round(upstream.Track*10) / 10,
		Heading:             math.Round(upstream.Heading*10) / 10,
		LastReport:          parseLastReport(upstream.LastReport, fallbackReport),
		PilotState:          upstream.PilotState,
		IsConnected:         upstream.IsConnected,
		AircraftName:        aircraftName,
		LiveryName:          liveryName,
		SessionID:           session.ID,
		NormalizedName:      session.NormalizedName,
		Normalized: Normalized{
			Speed:         fmt.Sprintf("%d kts", speed),
			VerticalSpeed: fmt.Sprintf("%.1f ft/min", verticalSpeed),
			PilotState:    NameForPilotState(upstream.PilotState),
			IsConnected:   connected,
		},
		PathSync: &PathSync{FPLSyncRequired: false},
	}
	return flight
}

func UpsertFlights(existing []Flight, mapped []Flight) []Flight {
	index := make(map[string]Flight, len(existing))
	for _, flight := range existing {
		index[flight.FlightID] = flight
	}
	result := make([]Flight, 0, len(mapped))
	seen := make(map[string]int, len(mapped))
	for _, current := range mapped {
		if prior, hadPrior := index[current.FlightID]; hadPrior {
			current = mergeNames(current, prior)
		}
		current.History = nil
		if previousIndex, duplicate := seen[current.FlightID]; duplicate {
			result[previousIndex] = current
			continue
		}
		seen[current.FlightID] = len(result)
		result = append(result, current)
	}
	return result
}

func mergeNames(current, prior Flight) Flight {
	if current.AircraftName == "" {
		current.AircraftName = prior.AircraftName
	}
	if current.LiveryName == "" {
		current.LiveryName = prior.LiveryName
	}
	return current
}

func NextHistory(existing []Flight, prior Flight) []Flight {
	historical := prior
	historical.History = nil
	historical.PathSync = nil
	history := make([]Flight, 0, len(existing)+1)
	history = append(history, existing...)
	history = append(history, historical)
	if len(history) > MaxHistory {
		history = history[len(history)-MaxHistory:]
	}
	return history
}

func parseLastReport(raw string, fallback time.Time) time.Time {
	parsed, err := time.Parse(lastReportLayout, raw)
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}
