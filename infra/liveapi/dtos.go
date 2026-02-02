package liveapi

import (
	"fmt"
	"time"
)

// Request DTOs

// LiveApiUserStatsReq is the request body for fetching user stats
type LiveApiUserStatsReq struct {
	DiscourseNames []string `json:"discourseNames"`
}

// Response DTOs

// UserGradeResponse contains user grade information
type UserGradeResponse struct {
	UserID string `json:"UserID"`
	Grade  int    `json:"Grade"`
}

// UserStatsResponse contains detailed user statistics
type UserStatsResponse struct {
	ErrorCode int         `json:"errorCode"`
	Result    []UserStats `json:"result"`
}

// UserStats represents individual user statistics
type UserStats struct {
	OnlineFlights         int                   `json:"onlineFlights"`
	Violations            int                   `json:"violations"`
	XP                    int                   `json:"xp"`
	LandingCount          int                   `json:"landingCount"`
	FlightTime            int                   `json:"flightTime"`
	ATCOperations         int                   `json:"atcOperations"`
	ATCRank               *int                  `json:"atcRank"` // nullable
	Grade                 int                   `json:"grade"`
	Hash                  string                `json:"hash"`
	ViolationCountByLevel ViolationCountByLevel `json:"violationCountByLevel"`
	Roles                 []int                 `json:"roles"`
	UserID                string                `json:"userId"`
	VirtualOrganization   *string               `json:"virtualOrganization"` // nullable
	DiscourseUsername     *string               `json:"discourseUsername"`   // nullable
	Groups                []string              `json:"groups"`
	ErrorCode             int                   `json:"errorCode"`
}

// ViolationCountByLevel breaks down violations by severity
type ViolationCountByLevel struct {
	Level1 int `json:"level1"`
	Level2 int `json:"level2"`
	Level3 int `json:"level3"`
}

// SessionsResponse contains all active multiplayer sessions
type SessionsResponse struct {
	ErrorCode int       `json:"errorCode"`
	Result    []Session `json:"result"`
}

// Session represents an Infinite Flight multiplayer session/server
type Session struct {
	MaxUsers          int     `json:"maxUsers"`
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	UserCount         int     `json:"userCount"`
	Type              int     `json:"type"`
	WorldType         int     `json:"worldType"`
	MinimumGradeLevel int     `json:"minimumGradeLevel"`
	MinimumAppVersion string  `json:"minimumAppVersion"`
	MaximumAppVersion *string `json:"maximumAppVersion"` // nullable
}

// FlightRouteResponse contains waypoints for a flight route
type FlightRouteResponse struct {
	ErrorCode int              `json:"errorCode"`
	Result    []FlightPosition `json:"result"`
}

// FlightPosition represents a single waypoint in a flight route
type FlightPosition struct {
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Altitude    float64   `json:"altitude"`
	Track       float64   `json:"track"`
	GroundSpeed float64   `json:"groundSpeed"`
	Date        time.Time `json:"date"`
	ID          *string   `json:"id"`
	FID         string    `json:"fid"`
}

// ATCResponse contains all active ATC sessions
type ATCResponse struct {
	ATC []ATCEntry `json:"ATC"`
}

// ATCEntry represents an active ATC controller
type ATCEntry struct {
	ID        string  `json:"Id"`
	Type      int     `json:"Type"`
	Frequency string  `json:"Frequency"`
	Facility  int     `json:"Facility"`
	Latitude  float64 `json:"Latitude"`
	Longitude float64 `json:"Longitude"`
	Altitude  int     `json:"Altitude"`
	Airport   string  `json:"Airport"`
	Active    bool    `json:"Active"`
	Username  string  `json:"Username"`
	UserID    string  `json:"UserID"`
}

// FlightsResponse contains all flights in a session
type FlightsResponse struct {
	Flights []FlightEntry `json:"result"`
}

// FlightEntry represents an active flight
// Note: Raw API values are stored here, normalization happens during cache processing
type FlightEntry struct {
	Username            string  `json:"username"`
	Callsign            string  `json:"callsign"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	Altitude            float64 `json:"altitude"`      // Raw feet from API
	Speed               float64 `json:"speed"`         // Raw knots from API
	VerticalSpeed       float64 `json:"verticalSpeed"` // Raw ft/min from API
	Track               float64 `json:"track"`         // Raw degrees from API
	LastReport          string  `json:"lastReport"`
	FlightID            string  `json:"flightId"`
	UserID              string  `json:"userId"`
	AircraftID          string  `json:"aircraftId"`
	LiveryID            string  `json:"liveryId"`
	VirtualOrganization string  `json:"virtualOrganization"`
	PilotState          int     `json:"pilotState"`
	IsConnected         bool    `json:"isConnected"`
}

// AircraftLiveriesResponse contains all aircraft and liveries
type AircraftLiveriesResponse struct {
	Liveries  []AircraftLivery `json:"result"`
	ErrorCode int              `json:"errorCode"`
}

// AircraftLivery represents an aircraft livery combination
type AircraftLivery struct {
	LiveryId     string `json:"id"`
	AircraftID   string `json:"aircraftID"`
	LiveryName   string `json:"liveryName"`
	AircraftName string `json:"aircraftName"`
}

// UserFlightsRawResponse wraps the paginated user flights response
type UserFlightsRawResponse struct {
	ErrorCode int                 `json:"errorCode"`
	Result    UserFlightsResponse `json:"result"`
}

// UserFlightsResponse contains paginated user flight history
type UserFlightsResponse struct {
	PageIndex   int               `json:"pageIndex"`
	TotalPages  int               `json:"totalPages"`
	TotalCount  int               `json:"totalCount"`
	HasPrevious bool              `json:"hasPreviousPage"`
	HasNext     bool              `json:"hasNextPage"`
	Flights     []UserFlightEntry `json:"data"`
}

// UserFlightEntry represents a historical flight
type UserFlightEntry struct {
	ID                 string    `json:"id"`
	Created            time.Time `json:"created"`
	UserID             string    `json:"userId"`
	AircraftID         string    `json:"aircraftId"`
	LiveryID           string    `json:"liveryId"`
	Callsign           string    `json:"callsign"`
	Server             string    `json:"server"`
	DayTime            float32   `json:"dayTime"`
	NightTime          float32   `json:"nightTime"`
	TotalTime          float32   `json:"totalTime"`
	LandingCount       int       `json:"landingCount"`
	OriginAirport      string    `json:"originAirport"`
	DestinationAirport string    `json:"destinationAirport"`
	XP                 int       `json:"xp"`
	WorldType          int       `json:"worldType"`
	Violations         []any     `json:"violations"`
}

// WorldStatusResponse contains global world status
type WorldStatusResponse struct {
	Status           string         `json:"Status"`
	Servers          []WorldServer  `json:"Servers"`
	OnlineFlights    int            `json:"OnlineFlights"`
	ActiveATC        int            `json:"ActiveATC"`
	OnlineUsers      int            `json:"OnlineUsers"`
	RecentViolations map[string]int `json:"RecentViolations"`
}

// WorldServer represents a world server status
type WorldServer struct {
	ID     int    `json:"Id"`
	Name   string `json:"Name"`
	Status int    `json:"Status"`
}

// ATISResponse contains ATIS information
type ATISResponse struct {
	ATIS []ATISEntry `json:"ATIS"`
}

// ATISEntry represents airport ATIS information
type ATISEntry struct {
	Airport   string `json:"Airport"`
	Frequency string `json:"Frequency"`
	Text      string `json:"Text"`
	Updated   string `json:"Updated"`
}

// FlightPlanWrapper wraps the flight plan response
type FlightPlanWrapper struct {
	ErrorCode int                `json:"errorCode"`
	Result    FlightPlanResponse `json:"result"`
}

// FlightPlanResponse contains flight plan details
type FlightPlanResponse struct {
	FlightPlanID    string           `json:"flightPlanId"`
	FlightID        string           `json:"flightId"`
	Waypoints       []string         `json:"waypoints"`
	LastUpdate      APITime          `json:"lastUpdate"`
	FlightPlanItems []FlightPlanItem `json:"flightPlanItems"`
}

// FlightPlanItem represents a flight plan waypoint or segment
type FlightPlanItem struct {
	Name       string           `json:"name"`
	Type       int              `json:"type"`
	Children   []FlightPlanItem `json:"children"`
	Identifier *string          `json:"identifier"`
	Altitude   float64          `json:"altitude"`
	Location   Location         `json:"location"`
}

// Location represents a geographic coordinate
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

// APITime handles custom time format from Infinite Flight API
type APITime struct{ time.Time }

const apiLayout = "2006-01-02 15:04:05Z07:00"

// UnmarshalJSON parses the custom API time format
// Also handles ISO 8601 format (RFC3339) that comes from cached JSON
func (t *APITime) UnmarshalJSON(b []byte) error {
	s := string(b)
	s = s[1 : len(s)-1] // strip quotes
	if s == "" || s == "null" {
		return nil
	}
	
	// Try the original API format first
	tt, err := time.Parse(apiLayout, s)
	if err == nil {
		t.Time = tt
		return nil
	}
	
	// Try ISO 8601 / RFC3339 format (used by Go's standard JSON encoding)
	tt, err = time.Parse(time.RFC3339, s)
	if err == nil {
		t.Time = tt
		return nil
	}
	
	// Try ISO 8601 with nanoseconds
	tt, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		t.Time = tt
		return nil
	}
	
	// If all formats fail, return the original error
	return fmt.Errorf("failed to parse time %q: %w", s, err)
}

// MarshalJSON ensures consistent JSON encoding
func (t APITime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Time.Format(time.RFC3339) + `"`), nil
}
