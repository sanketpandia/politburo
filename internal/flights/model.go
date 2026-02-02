package flights

import "time"

// FlightData represents current flight information for a user
type FlightData struct {
	Callsign     string  `json:"callsign"`
	IFCUsername  string  `json:"ifc_username"`
	Aircraft     string  `json:"aircraft"`
	Livery       string  `json:"livery"`
	LiveryID     string  `json:"livery_id"`
	Route        string  `json:"route"`
	Altitude     int     `json:"altitude"`   // Altitude in feet
	Speed        int     `json:"speed"`      // Speed in knots
	Multiplier   float64 `json:"multiplier"` // Mode multiplier
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
type WaypointSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Speed     float64   `json:"speed"`
	Track     float64   `json:"track"`
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

	// Current position data
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	Speed         float64 `json:"speed"`
	Track         float64 `json:"track"`
	VerticalSpeed float64 `json:"vertical_speed"`

	// Aircraft identifiers
	AircraftID string `json:"aircraft_id"`
	LiveryID   string `json:"livery_id"`

	// Flight phase tracking (embedded state)
	Phase       FlightPhase `json:"phase"`
	TakeoffTime *time.Time  `json:"takeoff_time,omitempty"`
	LandingTime *time.Time  `json:"landing_time,omitempty"`

	// VA associations (can belong to multiple VAs)
	VAIDs []string `json:"va_ids,omitempty"`

	// Waypoints history (max 600 entries = ~20 hours)
	Waypoints           []WaypointSnapshot `json:"waypoints"`
	LastUpdatedWaypoint time.Time          `json:"last_updated_waypoint"`

	// Metadata
	LastUpdated time.Time `json:"last_updated"`
	LastReport  time.Time `json:"last_report"`
}

// VAPattern represents a cached VA callsign configuration (in-memory only)
type VAPattern struct {
	VAID   string
	Prefix string
	Suffix string
}
