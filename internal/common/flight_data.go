package common

import "context"

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

// GetUserFlight retrieves the current flight for a user by searching sessions
// The method looks for a flight matching the user's callsign across all active sessions
// Note: Aircraft, Livery, and Route information should be enriched by the caller
func (svc *LiveAPIService) GetUserFlight(ctx context.Context, callsign string) (*FlightData, error) {
	// Retired behavior summary: this method previously fetched all active sessions,
	// fetched every session's flights, matched by callsign, and returned a partial
	// FlightData value for caller-side aircraft/livery/route enrichment. Rebuild this
	// as a feature service over infra/liveapi.Client if a current use case returns.
	return nil, ErrLiveAPIServiceNotImplemented
}
