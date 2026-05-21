package flights

import (
	"encoding/json"
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

type FlightTrendPoint struct {
	Timestamp time.Time `json:"ts"`
	Altitude  int       `json:"alt"`
	Speed     int       `json:"spd"`
}

type FlightTrendQueue struct {
	Items []FlightTrendPoint `json:"i,omitempty"`
}

type FlightTrend struct {
	AltitudeRateFpm float64
	SpeedRateKpm    float64

	AltitudeRising  bool
	AltitudeFalling bool
	AltitudeStable  bool

	SpeedIncreasing bool
	SpeedDecreasing bool
	SpeedStable     bool
}

const maxFlightTrendPoints = 6

// WaypointSnapshot represents a point-in-time flight position for logbook
// All values are normalized: coordinates to 4 decimals, altitude in feet (int), speed in m/s (int), track to 1 decimal
type WaypointSnapshot struct {
	Timestamp time.Time `json:"ts"`
	Latitude  float64   `json:"lat"` // Normalized to 4 decimal places
	Longitude float64   `json:"lon"` // Normalized to 4 decimal places
	Altitude  int       `json:"alt"` // Normalized: feet rounded to int (can be negative)
	Speed     int       `json:"spd"` // Normalized: knots rounded to int
	Track     float64   `json:"trk"` // Normalized to 1 decimal place
}

// UnmarshalJSON implements custom JSON unmarshaling for WaypointSnapshot
// This handles backward compatibility with old cached data that may have float values
// for altitude and speed fields (which are now int)
func (ws *WaypointSnapshot) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with flexible types for altitude and speed
	type Alias struct {
		Timestamp time.Time       `json:"ts"`
		Latitude  float64         `json:"lat"`
		Longitude float64         `json:"lon"`
		Altitude  json.RawMessage `json:"alt"`
		Speed     json.RawMessage `json:"spd"`
		Track     float64         `json:"trk"`
	}
	type LegacyAlias struct {
		Timestamp time.Time       `json:"timestamp"`
		Latitude  float64         `json:"latitude"`
		Longitude float64         `json:"longitude"`
		Altitude  json.RawMessage `json:"altitude"`
		Speed     json.RawMessage `json:"speed"`
		Track     float64         `json:"track"`
	}

	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if alias.Timestamp.IsZero() {
		var legacy LegacyAlias
		if err := json.Unmarshal(data, &legacy); err == nil {
			alias = Alias{Timestamp: legacy.Timestamp, Latitude: legacy.Latitude, Longitude: legacy.Longitude, Altitude: legacy.Altitude, Speed: legacy.Speed, Track: legacy.Track}
		}
	}

	// Copy all fields
	ws.Timestamp = alias.Timestamp
	ws.Latitude = alias.Latitude
	ws.Longitude = alias.Longitude
	ws.Track = alias.Track

	// Handle altitude: can be int or float (or null)
	if len(alias.Altitude) > 0 && string(alias.Altitude) != "null" {
		var altFloat float64
		if err := json.Unmarshal(alias.Altitude, &altFloat); err == nil {
			// Successfully unmarshaled as float, normalize to int
			ws.Altitude = normalizeAltitude(altFloat)
		} else {
			// Try as int
			var altInt int
			if err := json.Unmarshal(alias.Altitude, &altInt); err == nil {
				ws.Altitude = altInt
			} else {
				// If both fail, leave as zero value (0)
			}
		}
	}

	// Handle speed: can be int or float (or null)
	if len(alias.Speed) > 0 && string(alias.Speed) != "null" {
		var speedFloat float64
		if err := json.Unmarshal(alias.Speed, &speedFloat); err == nil {
			// Successfully unmarshaled as float, normalize to int
			ws.Speed = normalizeSpeed(speedFloat)
		} else {
			// Try as int
			var speedInt int
			if err := json.Unmarshal(alias.Speed, &speedInt); err == nil {
				ws.Speed = speedInt
			} else {
				// If both fail, leave as zero value (0)
			}
		}
	}

	return nil
}

// CompleteFlight represents all cached flight data in ONE Redis object
// Stored at: game:live:flight:<flight_id> with 7-day TTL
type CompleteFlight struct {
	// Core flight data
	FlightID string `json:"fid"`
	Callsign string `json:"cs"`
	UserID   string `json:"uid"`
	Username string `json:"un"`

	// Session context (IMPORTANT: store session for multi-session support)
	SessionID   string `json:"sid"`
	SessionName string `json:"sn"`

	// Current position data (all normalized)
	Latitude      float64 `json:"lat"` // Normalized to 4 decimal places
	Longitude     float64 `json:"lon"` // Normalized to 4 decimal places
	Altitude      int     `json:"alt"` // Normalized: feet rounded to int (can be negative)
	Speed         int     `json:"spd"` // Normalized: knots rounded to int
	Track         float64 `json:"trk"` // Normalized to 1 decimal place
	VerticalSpeed float64 `json:"vs"`  // Normalized: ft/min rounded to 1 decimal place

	// Aircraft identifiers
	AircraftID   string `json:"aid"`
	LiveryID     string `json:"lid"`
	AircraftName string `json:"an,omitempty"` // Cached from aircraft cache job
	LiveryName   string `json:"ln,omitempty"` // Cached from aircraft cache job

	// Flight phase tracking (embedded state)
	Phase       FlightPhase `json:"ph"`
	TakeoffTime *time.Time  `json:"to,omitempty"`
	LandingTime *time.Time  `json:"ld,omitempty"`

	// VA associations (can belong to multiple VAs)
	VAIDs []string `json:"vas,omitempty"`

	// Route information (from flight plan)
	Origin      string `json:"org,omitempty"`
	Destination string `json:"dst,omitempty"`

	// Waypoints history (max 600 entries = ~20 hours)
	Waypoints           []WaypointSnapshot `json:"wps"`
	LastUpdatedWaypoint time.Time          `json:"luw"`

	// Metadata
	DetectedAt          time.Time        `json:"da"` // When we first detected/created this flight record
	LastUpdated         time.Time        `json:"lu"` // When we last updated this record in cache
	LastReport          time.Time        `json:"lr"` // When pilot last reported position to game servers
	LastFlightPlanFetch time.Time        `json:"lfp,omitempty"`
	TrendQueue          FlightTrendQueue `json:"tq,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for CompleteFlight
// This handles backward compatibility with old cached data that may have float values
// for altitude and speed fields (which are now int)
func (cf *CompleteFlight) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with flexible types for altitude and speed
	type Alias struct {
		FlightID            string             `json:"flight_id"`
		Callsign            string             `json:"callsign"`
		UserID              string             `json:"user_id"`
		Username            string             `json:"username"`
		SessionID           string             `json:"session_id"`
		SessionName         string             `json:"session_name"`
		Latitude            float64            `json:"latitude"`
		Longitude           float64            `json:"longitude"`
		Altitude            json.RawMessage    `json:"altitude"` // Use RawMessage to handle both int and float
		Speed               json.RawMessage    `json:"speed"`    // Use RawMessage to handle both int and float
		Track               float64            `json:"track"`
		VerticalSpeed       float64            `json:"vertical_speed"`
		AircraftID          string             `json:"aircraft_id"`
		LiveryID            string             `json:"livery_id"`
		AircraftName        string             `json:"aircraft_name,omitempty"`
		LiveryName          string             `json:"livery_name,omitempty"`
		Phase               FlightPhase        `json:"phase"`
		TakeoffTime         *time.Time         `json:"takeoff_time,omitempty"`
		LandingTime         *time.Time         `json:"landing_time,omitempty"`
		VAIDs               []string           `json:"va_ids,omitempty"`
		Origin              string             `json:"origin,omitempty"`
		Destination         string             `json:"destination,omitempty"`
		Waypoints           []WaypointSnapshot `json:"waypoints"`
		LastUpdatedWaypoint time.Time          `json:"last_updated_waypoint"`
		DetectedAt          time.Time          `json:"detected_at"`
		LastUpdated         time.Time          `json:"last_updated"`
		LastReport          time.Time          `json:"last_report"`
		LastFlightPlanFetch time.Time          `json:"last_flight_plan_fetch,omitempty"`
		TrendQueue          FlightTrendQueue   `json:"tq,omitempty"`
	}
	type ShortAlias struct {
		FlightID            string             `json:"fid"`
		Callsign            string             `json:"cs"`
		UserID              string             `json:"uid"`
		Username            string             `json:"un"`
		SessionID           string             `json:"sid"`
		SessionName         string             `json:"sn"`
		Latitude            float64            `json:"lat"`
		Longitude           float64            `json:"lon"`
		Altitude            json.RawMessage    `json:"alt"`
		Speed               json.RawMessage    `json:"spd"`
		Track               float64            `json:"trk"`
		VerticalSpeed       float64            `json:"vs"`
		AircraftID          string             `json:"aid"`
		LiveryID            string             `json:"lid"`
		AircraftName        string             `json:"an,omitempty"`
		LiveryName          string             `json:"ln,omitempty"`
		Phase               FlightPhase        `json:"ph"`
		TakeoffTime         *time.Time         `json:"to,omitempty"`
		LandingTime         *time.Time         `json:"ld,omitempty"`
		VAIDs               []string           `json:"vas,omitempty"`
		Origin              string             `json:"org,omitempty"`
		Destination         string             `json:"dst,omitempty"`
		Waypoints           []WaypointSnapshot `json:"wps"`
		LastUpdatedWaypoint time.Time          `json:"luw"`
		DetectedAt          time.Time          `json:"da"`
		LastUpdated         time.Time          `json:"lu"`
		LastReport          time.Time          `json:"lr"`
		LastFlightPlanFetch time.Time          `json:"lfp,omitempty"`
		TrendQueue          FlightTrendQueue   `json:"tq,omitempty"`
	}

	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if alias.FlightID == "" {
		var short ShortAlias
		if err := json.Unmarshal(data, &short); err == nil {
			alias = Alias{FlightID: short.FlightID, Callsign: short.Callsign, UserID: short.UserID, Username: short.Username, SessionID: short.SessionID, SessionName: short.SessionName, Latitude: short.Latitude, Longitude: short.Longitude, Altitude: short.Altitude, Speed: short.Speed, Track: short.Track, VerticalSpeed: short.VerticalSpeed, AircraftID: short.AircraftID, LiveryID: short.LiveryID, AircraftName: short.AircraftName, LiveryName: short.LiveryName, Phase: short.Phase, TakeoffTime: short.TakeoffTime, LandingTime: short.LandingTime, VAIDs: short.VAIDs, Origin: short.Origin, Destination: short.Destination, Waypoints: short.Waypoints, LastUpdatedWaypoint: short.LastUpdatedWaypoint, DetectedAt: short.DetectedAt, LastUpdated: short.LastUpdated, LastReport: short.LastReport, LastFlightPlanFetch: short.LastFlightPlanFetch, TrendQueue: short.TrendQueue}
		}
	}

	// Copy all fields
	cf.FlightID = alias.FlightID
	cf.Callsign = alias.Callsign
	cf.UserID = alias.UserID
	cf.Username = alias.Username
	cf.SessionID = alias.SessionID
	cf.SessionName = alias.SessionName
	cf.Latitude = alias.Latitude
	cf.Longitude = alias.Longitude
	cf.Track = alias.Track
	cf.VerticalSpeed = alias.VerticalSpeed
	cf.AircraftID = alias.AircraftID
	cf.LiveryID = alias.LiveryID
	cf.AircraftName = alias.AircraftName
	cf.LiveryName = alias.LiveryName
	cf.Phase = alias.Phase
	cf.TakeoffTime = alias.TakeoffTime
	cf.LandingTime = alias.LandingTime
	cf.VAIDs = alias.VAIDs
	cf.Origin = alias.Origin
	cf.Destination = alias.Destination
	cf.Waypoints = alias.Waypoints
	cf.LastUpdatedWaypoint = alias.LastUpdatedWaypoint
	cf.DetectedAt = alias.DetectedAt
	cf.LastUpdated = alias.LastUpdated
	cf.LastReport = alias.LastReport
	cf.LastFlightPlanFetch = alias.LastFlightPlanFetch
	cf.TrendQueue = alias.TrendQueue

	// Handle altitude: can be int or float (or null)
	if len(alias.Altitude) > 0 && string(alias.Altitude) != "null" {
		var altFloat float64
		if err := json.Unmarshal(alias.Altitude, &altFloat); err == nil {
			// Successfully unmarshaled as float, normalize to int
			cf.Altitude = normalizeAltitude(altFloat)
		} else {
			// Try as int
			var altInt int
			if err := json.Unmarshal(alias.Altitude, &altInt); err == nil {
				cf.Altitude = altInt
			} else {
				// If both fail, leave as zero value (0)
			}
		}
	}

	// Handle speed: can be int or float (or null)
	if len(alias.Speed) > 0 && string(alias.Speed) != "null" {
		var speedFloat float64
		if err := json.Unmarshal(alias.Speed, &speedFloat); err == nil {
			// Successfully unmarshaled as float, normalize to int
			cf.Speed = normalizeSpeed(speedFloat)
		} else {
			// Try as int
			var speedInt int
			if err := json.Unmarshal(alias.Speed, &speedInt); err == nil {
				cf.Speed = speedInt
			} else {
				// If both fail, leave as zero value (0)
			}
		}
	}

	return nil
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

func appendTrendPoint(q FlightTrendQueue, timestamp time.Time, altitude int, speed int) FlightTrendQueue {
	q.Items = append(q.Items, FlightTrendPoint{Timestamp: timestamp.UTC(), Altitude: altitude, Speed: speed})
	if len(q.Items) > maxFlightTrendPoints {
		q.Items = q.Items[len(q.Items)-maxFlightTrendPoints:]
	}
	return q
}

func calculateTrendFromQueue(q FlightTrendQueue) FlightTrend {
	if len(q.Items) < 2 {
		return FlightTrend{}
	}

	first := q.Items[0]
	last := q.Items[len(q.Items)-1]
	minutes := last.Timestamp.Sub(first.Timestamp).Minutes()
	if minutes <= 0 {
		minutes = float64(len(q.Items) - 1)
	}
	if minutes <= 0 {
		return FlightTrend{}
	}

	altitudeRateFpm := float64(last.Altitude-first.Altitude) / minutes
	speedRateKpm := float64(last.Speed-first.Speed) / minutes

	return FlightTrend{
		AltitudeRateFpm: altitudeRateFpm,
		SpeedRateKpm:    speedRateKpm,

		AltitudeRising:  altitudeRateFpm > 300,
		AltitudeFalling: altitudeRateFpm < -300,
		AltitudeStable:  math.Abs(altitudeRateFpm) < 150,

		SpeedIncreasing: speedRateKpm > 10,
		SpeedDecreasing: speedRateKpm < -10,
		SpeedStable:     math.Abs(speedRateKpm) < 10,
	}
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
