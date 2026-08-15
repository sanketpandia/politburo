// Package flights defines the shared cache contract for active Infinite Flight flights.
package flights

import "time"

const (
	RefreshSchedule      = "0 * * * * *"
	RefreshInterval      = time.Minute
	GameActiveFlightTTL  = 3 * 24 * time.Hour
	MaxHistory           = 25
	MaxFlightsPerRequest = 5000
	DefaultPageLength    = 50
	DefaultPageNumber    = 1
	lastReportLayout     = "2006-01-02 15:04:05Z07:00"
)

type PathSync struct {
	FPLSyncRequired bool `json:"fplSyncRequired"`
}

type Normalized struct {
	Speed         string `json:"speed"`
	VerticalSpeed string `json:"verticalSpeed"`
	PilotState    string `json:"pilotState"`
	IsConnected   string `json:"isConnected"`
}

type Flight struct {
	FlightID            string     `json:"flightId"`
	UserID              string     `json:"userId"`
	AircraftID          string     `json:"aircraftId"`
	LiveryID            string     `json:"liveryId"`
	Username            *string    `json:"username"`
	VirtualOrganization *string    `json:"virtualOrganization"`
	Callsign            string     `json:"callsign"`
	Latitude            float64    `json:"latitude"`
	Longitude           float64    `json:"longitude"`
	Altitude            int        `json:"altitude"`
	Speed               int        `json:"speed"`
	VerticalSpeed       float64    `json:"verticalSpeed"`
	Track               float64    `json:"track"`
	Heading             float64    `json:"heading"`
	LastReport          time.Time  `json:"lastReport"`
	PilotState          int        `json:"pilotState"`
	IsConnected         bool       `json:"isConnected"`
	AircraftName        string     `json:"aircraftName,omitempty"`
	LiveryName          string     `json:"liveryName,omitempty"`
	SessionID           string     `json:"sessionId"`
	NormalizedName      string     `json:"normalizedName"`
	Normalized          Normalized `json:"normalized"`
	PathSync            *PathSync  `json:"pathSync,omitempty"`
	// History is only used when reading legacy nested snapshots; live cache
	// entries store history at cache.KeyFlightHistory instead.
	History []Flight `json:"history,omitempty"`
}

type HistorySnapshot struct {
	Result []Flight `json:"result"`
}

type Snapshot struct {
	Result     []Flight  `json:"result"`
	LastCached time.Time `json:"lastCached"`
}
