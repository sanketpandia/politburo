package flights

import "time"

// VALiveFlightDTO represents a live flight response for the VA flights endpoint
// This DTO excludes internal fields and uses UTC timestamps
type VALiveFlightDTO struct {
	// Core flight data
	FlightID string `json:"flight_id"`
	Callsign string `json:"callsign"`
	Username string `json:"username"`

	// Session context (only name, not ID)
	SessionName string `json:"session_name"`

	// Current position data
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      int     `json:"altitude"`
	Speed         int     `json:"speed"`
	Track         float64 `json:"track"`
	VerticalSpeed float64 `json:"vertical_speed"`

	// Aircraft names (not IDs)
	AircraftName string `json:"aircraft_name"`
	LiveryName   string `json:"livery_name"`

	// Flight phase tracking
	Phase        FlightPhase               `json:"phase"`
	PhaseHistory []FlightPhaseHistoryEntry `json:"phase_history,omitempty"`
	TakeoffTime  *time.Time                `json:"takeoff_time"` // null if not yet detected
	LandingTime  *time.Time                `json:"landing_time,omitempty"`

	// Route information
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	Route       string `json:"route,omitempty"`

	// Flight statistics (calculated from waypoints)
	MaxSpeed    *int `json:"max_speed"`    // null if no waypoints, max speed in knots from waypoints
	MaxAltitude *int `json:"max_altitude"` // null if no waypoints, max altitude in feet from waypoints

	// Metadata (UTC timestamps)
	DetectedAt          time.Time  `json:"detected_at"`            // When we first detected/created this flight record
	LastUpdated         time.Time  `json:"last_updated"`           // When we last updated this record in cache
	LastReport          time.Time  `json:"last_report"`            // When pilot last reported position to game servers
	LastFlightPlanFetch *time.Time `json:"last_flight_plan_fetch"` // null if not yet fetched
}

// ToVALiveFlightDTO converts a CompleteFlight to VALiveFlightDTO
// All time fields are already in UTC from cache
func ToVALiveFlightDTO(flight *CompleteFlight) *VALiveFlightDTO {
	dto := &VALiveFlightDTO{
		// Core flight data
		FlightID: flight.FlightID,
		Callsign: flight.Callsign,
		Username: flight.Username,

		// Session context
		SessionName: flight.SessionName,

		// Current position data
		Latitude:      flight.Latitude,
		Longitude:     flight.Longitude,
		Altitude:      flight.Altitude,
		Speed:         flight.Speed,
		Track:         flight.Track,
		VerticalSpeed: flight.VerticalSpeed,

		// Aircraft names
		AircraftName: flight.AircraftName,
		LiveryName:   flight.LiveryName,

		// Flight phase tracking
		Phase:        flight.Phase,
		PhaseHistory: flight.PhaseHistory,
		TakeoffTime:  flight.TakeoffTime,
		LandingTime:  flight.LandingTime,

		// Route information
		Origin:      flight.Origin,
		Destination: flight.Destination,

		// Metadata (already UTC from cache)
		DetectedAt:          flight.DetectedAt.UTC(),
		LastUpdated:         flight.LastUpdated.UTC(),
		LastReport:          flight.LastReport.UTC(),
		LastFlightPlanFetch: nil,
	}
	if flight.Origin != "" && flight.Destination != "" {
		dto.Route = flight.Origin + "-" + flight.Destination
	}

	// Convert LastFlightPlanFetch to UTC if present
	if !flight.LastFlightPlanFetch.IsZero() {
		utcTime := flight.LastFlightPlanFetch.UTC()
		dto.LastFlightPlanFetch = &utcTime
	}

	// Convert TakeoffTime to UTC if present
	if flight.TakeoffTime != nil {
		utcTime := flight.TakeoffTime.UTC()
		dto.TakeoffTime = &utcTime
	}
	for i := range dto.PhaseHistory {
		dto.PhaseHistory[i].ChangedAt = dto.PhaseHistory[i].ChangedAt.UTC()
	}

	// Convert LandingTime to UTC if present
	if flight.LandingTime != nil {
		utcTime := flight.LandingTime.UTC()
		dto.LandingTime = &utcTime
	}

	// Calculate max speed and max altitude from waypoints
	if len(flight.Waypoints) > 0 {
		maxSpeed := flight.Waypoints[0].Speed
		maxAltitude := flight.Waypoints[0].Altitude

		for _, wp := range flight.Waypoints {
			if wp.Speed > maxSpeed {
				maxSpeed = wp.Speed
			}
			if wp.Altitude > maxAltitude {
				maxAltitude = wp.Altitude
			}
		}

		dto.MaxSpeed = &maxSpeed
		dto.MaxAltitude = &maxAltitude
	} else {
		// No waypoints yet - set to null
		dto.MaxSpeed = nil
		dto.MaxAltitude = nil
	}

	return dto
}

// VALiveFlightsResponse represents the response for GET /api/v1/flights/va
// Contains flights array and signed link for browser access
type VALiveFlightsResponse struct {
	Code       string               `json:"code"`
	Message    string               `json:"message"`
	Flights    []VALiveFlightDTO    `json:"flights"`
	Summary    VALiveFlightsSummary `json:"summary"`
	SignedLink string               `json:"signed_link,omitempty"`
}

type VALiveFlightsSummary struct {
	TotalDetectedFlights int                  `json:"total_detected_flights"`
	TopRoute             *VALiveFlightsRoute `json:"top_route,omitempty"`
}

type VALiveFlightsRoute struct {
	Route string `json:"route"`
	Count int    `json:"count"`
}
