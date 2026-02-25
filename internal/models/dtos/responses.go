package dtos

import "time"

// dtos/user_stats.go
type UserStatsResponse struct {
	ErrorCode int         `json:"errorCode"`
	Result    []UserStats `json:"result"`
}

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

type ViolationCountByLevel struct {
	Level1 int `json:"level1"`
	Level2 int `json:"level2"`
	Level3 int `json:"level3"`
}

// ---- USER GRADE ----
type UserGradeResponse struct {
	UserID string `json:"UserID"`
	Grade  int    `json:"Grade"`
}

// ---- ATC ----
type ATCResponse struct {
	ATC []ATCEntry `json:"ATC"`
}
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

// ---- FLIGHTS ----
type FlightsResponse struct {
	Flights []FlightEntry `json:"result"`
}
type FlightEntry struct {
	Username            string  `json:"username"`
	Callsign            string  `json:"callsign"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	Altitude            float64 `json:"altitude"` // docs show float
	Speed               float64 `json:"speed"`
	VerticalSpeed       float64 `json:"verticalSpeed"`
	Track               float64 `json:"track"`      // a.k.a. heading/course
	LastReport          string  `json:"lastReport"` // RFC3339 / ISO-8601
	FlightID            string  `json:"flightId"`
	UserID              string  `json:"userId"`
	AircraftID          string  `json:"aircraftId"`
	LiveryID            string  `json:"liveryId"`
	VirtualOrganization string  `json:"virtualOrganization"`
	PilotState          int     `json:"pilotState"`
	IsConnected         bool    `json:"isConnected"`
}

// ---- AIRCRAFT LIVERIES ----
type AircraftLiveriesResponse struct {
	Liveries  []AircraftLivery `json:"result"`
	ErrorCode int              `json:"errorCode"`
}
type AircraftLivery struct {
	LiveryId     string `json:"id"`
	AircraftID   string `json:"aircraftID"`
	LiveryName   string `json:"liveryName"`
	AircraftName string `json:"aircraftName"`
}

// ---- USER FLIGHTS ----

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
	Violations         []any     `json:"violations"` // If you know violation type, replace 'any'
}

// ---- WORLD STATUS ----
type WorldStatusResponse struct {
	Status           string         `json:"Status"`
	Servers          []WorldServer  `json:"Servers"`
	OnlineFlights    int            `json:"OnlineFlights"`
	ActiveATC        int            `json:"ActiveATC"`
	OnlineUsers      int            `json:"OnlineUsers"`
	RecentViolations map[string]int `json:"RecentViolations"`
}

type WorldServer struct {
	ID     int    `json:"Id"`
	Name   string `json:"Name"`
	Status int    `json:"Status"`
}

// ---- ATIS ----
type ATISResponse struct {
	ATIS []ATISEntry `json:"ATIS"`
}
type ATISEntry struct {
	Airport   string `json:"Airport"`
	Frequency string `json:"Frequency"`
	Text      string `json:"Text"`
	Updated   string `json:"Updated"`
}

// --- Controller endpoints ----

type APIResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	ResponseTime string `json:"response_time"`
	Data         any    `json:"data,omitempty"`
}

type InitApiResponse struct {
	IfcId   string             `json:"ifc_id"`
	Status  bool               `json:"status"`
	Message string             `json:"message"`
	Steps   []RegistrationStep `json:"steps"`
}

type RegistrationStep struct {
	Name    string `json:"name"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

type InitServerResponse struct {
	VACode  string             `json:"va_code"`
	Status  bool               `json:"status"`
	Message string             `json:"message,omitempty"`
	Steps   []RegistrationStep `json:"steps"`
}

type UserFlightsResponse struct {
	PageIndex   int               `json:"pageIndex"`
	TotalPages  int               `json:"totalPages"`
	TotalCount  int               `json:"totalCount"`
	HasPrevious bool              `json:"hasPreviousPage"`
	HasNext     bool              `json:"hasNextPage"`
	Flights     []UserFlightEntry `json:"data"`
}

type UserFlightsRawResponse struct {
	ErrorCode int                 `json:"errorCode"`
	Result    UserFlightsResponse `json:"result"`
}

type HistoryRecord struct {
	FlightID   string    `json:"flightId"`
	Origin     string    `json:"origin"`
	Dest       string    `json:"dest"`
	TimeStamp  time.Time `json:"timestamp"`
	EndTime    time.Time `json:"endtime"`
	Landings   int       `json:"landings"`
	Server     string    `json:"server"`
	SessionID  string    `json:"sessionId"` // Session ID for Live API route endpoint
	Aircraft   string    `json:"aircraft"`
	Livery     string    `json:"livery"`
	MapUrl     string    `json:"mapUrl"`
	Callsign   string    `json:"callsign"`
	Violations int       `json:"violations"`
	Equipment  string    `json:"equipment"`
	Duration   string    `json:"duration"`
	DayTime    float32   `json:"dayTime"`    // Day flight time in minutes
	NightTime  float32   `json:"nightTime"`  // Night flight time in minutes
	XP         int       `json:"xp"`         // Experience points earned
	WorldType  int       `json:"worldType"`  // 1=Casual, 2=Training, 3=Expert
	Username   string    `json:"username"`
}

type FlightHistoryDto struct {
	PageNo      int             `json:"page"`
	Records     []HistoryRecord `json:"records"`
	Error       string          `json:"error"`
	HasNext     bool            `json:"hasNext"`
	HasPrevious bool            `json:"hasPrevious"`
	TotalPages  int             `json:"totalPages"`
	TotalCount  int             `json:"totalCount"`
}

type SessionsResponse struct {
	ErrorCode int       `json:"errorCode"`
	Result    []Session `json:"result"`
}

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

type FlightRouteResponse struct {
	ErrorCode int              `json:"errorCode"`
	Result    []FlightPosition `json:"result"`
}

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

type LiveFlight struct {
	Callsign       string `json:"callsign"`
	CallsignVar    string `json:"callsignVar"`
	CallsignPrefix string `json:"callsignPrefix"`
	CallsignSuffix string `json:"callsignSuffix"`

	SessionID  string `json:"sessionID"`
	FlightID   string `json:"flightID"`
	AircraftId string `json:"aircraftID"`
	LiveryId   string `json:"liveryID"`
	Username   string `json:"username"`
	UserID     string `json:"userID"`

	Aircraft string `json:"aircraft"`
	Livery   string `json:"livery"`

	AltitudeFt  int    `json:"altitude"`
	SpeedKts    int    `json:"speed"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`

	ReportTime  time.Time `json:"lastReport"`
	IsConnected bool      `json:"isConnected"`

	// Flight phase tracking (from cache jobs)
	FlightPhase string `json:"flightPhase,omitempty"` // "on_ground", "takeoff", "climb", "cruise", "descent", "landed"
	TakeoffTime string `json:"takeoffTime,omitempty"` // ISO8601 timestamp
	LandingTime string `json:"landingTime,omitempty"` // ISO8601 timestamp
	LastUpdated string `json:"lastUpdated,omitempty"` // Cache freshness indicator
}
