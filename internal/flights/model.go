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
