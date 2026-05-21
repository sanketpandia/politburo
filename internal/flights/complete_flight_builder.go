package flights

import (
	"encoding/json"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
)

func (j *CacheJob) buildCompleteFlight(apiFlight liveapi.FlightEntry, sessionID string, sessionName string, matchedVAs []string) (*CompleteFlight, error) {
	existing := j.getCachedCompleteFlight(apiFlight.FlightID)
	lastReport := parseLastReport(apiFlight.LastReport)
	aircraftName, liveryName := j.resolveAircraftNames(apiFlight)
	now := time.Now().UTC()

	flight := &CompleteFlight{
		FlightID:      apiFlight.FlightID,
		Callsign:      apiFlight.Callsign,
		UserID:        apiFlight.UserID,
		Username:      apiFlight.Username,
		SessionID:     sessionID,
		SessionName:   sessionName,
		Latitude:      normalizeCoordinate(apiFlight.Latitude),
		Longitude:     normalizeCoordinate(apiFlight.Longitude),
		Altitude:      normalizeAltitude(apiFlight.Altitude),
		Speed:         normalizeSpeed(apiFlight.Speed),
		Track:         normalizeTrack(apiFlight.Track),
		VerticalSpeed: normalizeVerticalSpeed(apiFlight.VerticalSpeed),
		AircraftID:    apiFlight.AircraftID,
		LiveryID:      apiFlight.LiveryID,
		AircraftName:  aircraftName,
		LiveryName:    liveryName,
		VAIDs:         matchedVAs,
		LastUpdated:   now,
		LastReport:    lastReport,
	}

	preserveFlightState(flight, existing, now)
	flight.TrendQueue = appendTrendPoint(flight.TrendQueue, lastReport, flight.Altitude, flight.Speed)
	updateFlightPhase(flight, flight.Speed, flight.Altitude)
	j.cacheFlightRoute(apiFlight.FlightID, sessionID)

	return flight, nil
}

func (j *CacheJob) getCachedCompleteFlight(flightID string) *CompleteFlight {
	cachedVal, found := j.redisCache.Get(cache.LiveFlightKey(flightID))
	if !found {
		return nil
	}

	jsonBytes, err := json.Marshal(cachedVal)
	if err != nil {
		return nil
	}
	var flight CompleteFlight
	if err := json.Unmarshal(jsonBytes, &flight); err != nil {
		return nil
	}
	return &flight
}

func parseLastReport(raw string) time.Time {
	lastReport, err := parseLiveAPITime(raw)
	if err != nil {
		return time.Now().UTC()
	}
	return lastReport.UTC()
}

func (j *CacheJob) resolveAircraftNames(apiFlight liveapi.FlightEntry) (string, string) {
	if j.aircraftSvc == nil {
		return "", ""
	}
	return j.aircraftSvc.GetAircraftNameByID(apiFlight.AircraftID), j.aircraftSvc.GetLiveryNameByID(apiFlight.LiveryID)
}

func preserveFlightState(flight *CompleteFlight, existing *CompleteFlight, now time.Time) {
	if existing == nil {
		flight.Waypoints = make([]WaypointSnapshot, 0)
		flight.Phase = PhaseUnknown
		flight.DetectedAt = now
		return
	}

	flight.Waypoints = existing.Waypoints
	for i := range flight.Waypoints {
		flight.Waypoints[i].Timestamp = flight.Waypoints[i].Timestamp.UTC()
	}
	if !existing.LastUpdatedWaypoint.IsZero() {
		flight.LastUpdatedWaypoint = existing.LastUpdatedWaypoint.UTC()
	}
	flight.Phase = existing.Phase
	if existing.TakeoffTime != nil {
		takeoffTime := existing.TakeoffTime.UTC()
		flight.TakeoffTime = &takeoffTime
	}
	if existing.LandingTime != nil {
		landingTime := existing.LandingTime.UTC()
		flight.LandingTime = &landingTime
	}
	flight.Origin = existing.Origin
	flight.Destination = existing.Destination
	if !existing.LastFlightPlanFetch.IsZero() {
		flight.LastFlightPlanFetch = existing.LastFlightPlanFetch.UTC()
	}
	flight.TrendQueue = existing.TrendQueue
	if !existing.DetectedAt.IsZero() {
		flight.DetectedAt = existing.DetectedAt.UTC()
	}
	if flight.AircraftName == "" {
		flight.AircraftName = existing.AircraftName
	}
	if flight.LiveryName == "" {
		flight.LiveryName = existing.LiveryName
	}
}

func (j *CacheJob) cacheFlightRoute(flightID string, sessionID string) {
	routeResp, _, err := j.liveAPI.GetFlightRoute(flightID, sessionID)
	if err != nil {
		logging.Debug("Failed to cache flight route", "sessionID", sessionID, "flightID", flightID, "error", err)
		return
	}
	j.redisCache.Set(cache.FlightRouteKey(flightID), routeResp, cache.FlightPlanTTL)
}
