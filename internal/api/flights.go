package api

// DEPRECATED: All flight-related handlers have been migrated to flight_handlers.go
// This file is kept for backward compatibility but may be removed in the future.
// Use FlightHandlers methods instead:
//   - FlightHandlers.GetVALiveFlights() replaces VaFlightsHandler()
//   - FlightHandlers.GetLiveSessions() replaces LiveServers()
//   - FlightHandlers.GetUserFlights() replaces FetchFlights()
//   - FlightHandlers.GetFlightFromCache() replaces UserFlightMapHandler() (from maps.go)
//   - FlightHandlers.GetUserFlightsFromCache() replaces UserFlightsCacheHandler() (from maps.go)
