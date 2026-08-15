// Package liveries defines the shared cache contract for Infinite Flight liveries.
package liveries

import "time"

const (
	RefreshSchedule = "0 0 * * * *"
	CacheTTL        = 24 * time.Hour
)

type Livery struct {
	ID           string `json:"id"`
	AircraftID   string `json:"aircraftId"`
	AircraftName string `json:"aircraftName"`
	LiveryName   string `json:"liveryName"`
}
