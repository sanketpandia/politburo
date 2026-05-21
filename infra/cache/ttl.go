package cache

import "time"

const (
	// SessionTTL is the operational cache lifetime for Infinite Flight session metadata.
	SessionTTL = 24 * time.Hour

	// AircraftTTL is the operational cache lifetime for Infinite Flight aircraft/livery metadata.
	AircraftTTL = 24 * time.Hour

	// LiveFlightListTTL is the short-lived cache lifetime for session and VA live-flight ID lists.
	LiveFlightListTTL = 5 * time.Minute

	// LiveFlightTTL is the operational cache lifetime for LiveAPI-derived CompleteFlight objects.
	LiveFlightTTL = 48 * time.Hour

	// FlightPlanTTL is the operational cache lifetime for LiveAPI-derived flight plan data.
	FlightPlanTTL = 48 * time.Hour

	// WorldDetailsTTL is the operational cache lifetime for derived world/session details.
	WorldDetailsTTL = 48 * time.Hour
)
