package flights

import (
	"math"
	"time"
)

// FlightData represents current flight information for a user
type FlightData struct {
	Callsign    string  `json:"callsign"`
	IFCUsername string  `json:"ifc_username"`
	Aircraft    string  `json:"aircraft"`
	Livery      string  `json:"livery"`
	LiveryID    string  `json:"livery_id"`
	Route       string  `json:"route"`
	Altitude    int     `json:"altitude"`   // Altitude in feet
	Speed       int     `json:"speed"`      // Speed in knots
	Multiplier  float64 `json:"multiplier"` // Mode multiplier
}

// FlightPhase represents the current phase of a flight
type FlightPhase string

const (
	PhaseOnGround FlightPhase = "on_ground"
	PhaseTakeoff  FlightPhase = "takeoff"
	PhaseClimb    FlightPhase = "climb"
	PhaseCruise   FlightPhase = "cruise"
	PhaseDescent  FlightPhase = "descent"
	PhaseLanded   FlightPhase = "landed"
	PhaseUnknown  FlightPhase = "unknown"
)

// FlightState tracks the state and metadata of a flight for intelligent caching
type FlightState struct {
	FlightID     string      `json:"flight_id"`
	Callsign     string      `json:"callsign"`
	Phase        FlightPhase `json:"phase"`
	LastSpeed    float64     `json:"last_speed"`
	LastAltitude float64     `json:"last_altitude"`
	TakeoffTime  *time.Time  `json:"takeoff_time,omitempty"`
	LandingTime  *time.Time  `json:"landing_time,omitempty"`
	LastUpdated  time.Time   `json:"last_updated"`
	NextPollTime time.Time   `json:"next_poll_time"`
	LastPhase    FlightPhase `json:"last_phase,omitempty"`
}

// WaypointSnapshot represents a point-in-time flight position for logbook
// All values are normalized: coordinates to 4 decimals, altitude in feet (int), speed in m/s (int), track to 1 decimal
type WaypointSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Latitude  float64   `json:"latitude"`  // Normalized to 4 decimal places
	Longitude float64   `json:"longitude"` // Normalized to 4 decimal places
	Altitude  int       `json:"altitude"`  // Normalized: feet rounded to int (can be negative)
	Speed     int       `json:"speed"`     // Normalized: knots rounded to int
	Track     float64   `json:"track"`     // Normalized to 1 decimal place
}

// CompleteFlight represents all cached flight data in ONE Redis object
// Stored at: game:live:flight:<flight_id> with 7-day TTL
type CompleteFlight struct {
	// Core flight data
	FlightID string `json:"flight_id"`
	Callsign string `json:"callsign"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`

	// Session context (IMPORTANT: store session for multi-session support)
	SessionID   string `json:"session_id"`
	SessionName string `json:"session_name"`

	// Current position data (all normalized)
	Latitude      float64 `json:"latitude"`       // Normalized to 4 decimal places
	Longitude     float64 `json:"longitude"`      // Normalized to 4 decimal places
	Altitude      int     `json:"altitude"`       // Normalized: feet rounded to int (can be negative)
	Speed         int     `json:"speed"`          // Normalized: knots rounded to int
	Track         float64 `json:"track"`          // Normalized to 1 decimal place
	VerticalSpeed float64 `json:"vertical_speed"` // Normalized: ft/min rounded to 1 decimal place

	// Aircraft identifiers
	AircraftID   string `json:"aircraft_id"`
	LiveryID     string `json:"livery_id"`
	AircraftName string `json:"aircraft_name,omitempty"` // Cached from aircraft cache job
	LiveryName   string `json:"livery_name,omitempty"`   // Cached from aircraft cache job

	// Flight phase tracking (embedded state)
	Phase       FlightPhase `json:"phase"`
	TakeoffTime *time.Time  `json:"takeoff_time,omitempty"`
	LandingTime *time.Time  `json:"landing_time,omitempty"`

	// VA associations (can belong to multiple VAs)
	VAIDs []string `json:"va_ids,omitempty"`

	// Route information (from flight plan)
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`

	// Waypoints history (max 600 entries = ~20 hours)
	Waypoints           []WaypointSnapshot `json:"waypoints"`
	LastUpdatedWaypoint time.Time          `json:"last_updated_waypoint"`

	// Metadata
	DetectedAt          time.Time `json:"detected_at"`  // When we first detected/created this flight record
	LastUpdated         time.Time `json:"last_updated"` // When we last updated this record in cache
	LastReport          time.Time `json:"last_report"`  // When pilot last reported position to game servers
	LastFlightPlanFetch time.Time `json:"last_flight_plan_fetch,omitempty"`
}

// VAPattern represents a cached VA callsign configuration (in-memory only)
type VAPattern struct {
	VAID   string
	Prefix string
	Suffix string
}

// ShouldFetchFlightPlan determines if we should fetch the flight plan based on phase and timing
// Returns (shouldFetch, delay)
// This is used to prevent enqueueing flight plan requests too frequently
func ShouldFetchFlightPlan(flight *CompleteFlight) (bool, time.Duration) {
	now := time.Now()

	// Use LastFlightPlanFetch if available, otherwise use zero time (will always fetch on first run)
	var timeSinceLastFetch time.Duration
	if flight.LastFlightPlanFetch.IsZero() {
		// First time fetching for this flight - always fetch
		timeSinceLastFetch = time.Hour // Set to a large value to ensure fetch
	} else {
		timeSinceLastFetch = now.Sub(flight.LastFlightPlanFetch)
	}

	// Determine fetch interval based on phase
	var fetchInterval time.Duration
	switch flight.Phase {
	case PhaseCruise:
		fetchInterval = 10 * time.Minute
	case PhaseOnGround:
		fetchInterval = 2 * time.Minute
	case PhaseTakeoff:
		fetchInterval = 5 * time.Minute
	case PhaseClimb, PhaseDescent:
		// Use takeoff interval for climb/descent
		fetchInterval = 5 * time.Minute
	default:
		// Unknown or landed - use default 5 minutes
		fetchInterval = 5 * time.Minute
	}

	// Check if enough time has passed since last flight plan fetch
	if timeSinceLastFetch < fetchInterval {
		return false, 0
	}

	// Calculate delay for next fetch (spacing out API calls)
	// Use a small delay to prevent hammering the API
	return true, 200 * time.Millisecond
}

// normalizeAltitude rounds altitude from feet to int
// API already returns altitude in feet, so we just round to int
// Can be negative (e.g., Amsterdam is below sea level)
func normalizeAltitude(feet float64) int {
	return int(math.Round(feet))
}

// normalizeSpeed rounds speed from knots to int
// API already returns speed in knots, so we just round to int
func normalizeSpeed(knots float64) int {
	return int(math.Round(knots))
}

// normalizeCoordinate rounds latitude/longitude to 4 decimal places
// 4 decimal places = ~11 meters precision, sufficient for flight tracking
func normalizeCoordinate(coord float64) float64 {
	return math.Round(coord*10000) / 10000
}

// normalizeTrack rounds track/heading to 1 decimal place (degrees)
func normalizeTrack(track float64) float64 {
	return math.Round(track*10) / 10
}

// normalizeVerticalSpeed rounds vertical speed to 1 decimal place (ft/min)
// API already returns vertical speed in ft/min, so we just round to 1 decimal
func normalizeVerticalSpeed(ftPerMin float64) float64 {
	return math.Round(ftPerMin*10) / 10
}

// parseLiveAPITime converts strings like
// "2025-07-27 09:57:51Z"  →  time.Time (UTC)
func parseLiveAPITime(s string) (time.Time, error) {
	const layout = "2006-01-02 15:04:05Z07:00" // space-separated, UTC suffix
	return time.Parse(layout, s)
}
