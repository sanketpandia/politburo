package cache

// Cache key constants for Redis
// All keys use the "game:" prefix to namespace game-related data

const (
	// Session keys - Infinite Flight multiplayer sessions
	KeySessionPrefix     = "game:session:"      // game:session:<session_id> -> Full session object
	KeySessionNamePrefix = "game:session:name:" // game:session:name:<session_id> -> Session name string
	KeySessionList       = "game:sessions"      // game:sessions -> pipe-separated list of session IDs

	// Aircraft keys - Infinite Flight aircraft data
	KeyAircraftPrefix = "game:aircraft:" // game:aircraft:<aircraft_id> -> Aircraft object/name

	// Livery keys - Infinite Flight livery data
	KeyLiveryPrefix = "game:livery:" // game:livery:<livery_id> -> Livery object/name

	// Live flight tracking keys
	// All flights in a session - pipe-separated flight IDs
	// Format: game:live:flights:<session_id>
	// TTL: 5 minutes
	KeyLiveFlightsPrefix = "game:live:flights:"

	// Complete flight data (position + state + waypoints)
	// Format: game:live:flight:<flight_id>
	// TTL: 7 days
	KeyLiveFlightPrefix = "game:live:flight:"

	// VA-specific flight list - pipe-separated flight IDs
	// Format: game:live:vaflights:<va_id>
	// TTL: 5 minutes
	KeyLiveVAFlightsPrefix = "game:live:vaflights:"

	// Flight plan data (full FPL response)
	// Format: game:flightplan:<flight_id>
	// TTL: 7 days
	KeyFlightPlanPrefix = "game:flightplan:"
)

// Helper functions to build cache keys

// SessionKey returns the cache key for a specific session
func SessionKey(sessionID string) string {
	return KeySessionPrefix + sessionID
}

// SessionNameKey returns the cache key for a session name
func SessionNameKey(sessionID string) string {
	return KeySessionNamePrefix + sessionID
}

// AircraftKey returns the cache key for a specific aircraft
func AircraftKey(aircraftID string) string {
	return KeyAircraftPrefix + aircraftID
}

// LiveryKey returns the cache key for a specific livery
func LiveryKey(liveryID string) string {
	return KeyLiveryPrefix + liveryID
}

// LiveFlightsKey returns the cache key for all flights in a session
func LiveFlightsKey(sessionID string) string {
	return KeyLiveFlightsPrefix + sessionID
}

// LiveFlightKey returns the cache key for a specific flight
func LiveFlightKey(flightID string) string {
	return KeyLiveFlightPrefix + flightID
}

// LiveVAFlightsKey returns the cache key for all flights for a specific VA
func LiveVAFlightsKey(vaID string) string {
	return KeyLiveVAFlightsPrefix + vaID
}

// FlightPlanKey returns the cache key for a flight plan
func FlightPlanKey(flightID string) string {
	return KeyFlightPlanPrefix + flightID
}
