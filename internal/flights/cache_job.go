package flights

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/platform/va"
	"strings"
	"time"
)

// CacheJob syncs live flight data with intelligent caching and waypoint tracking
// Implements the simplified 3-key cache architecture with CompleteFlight objects
type CacheJob struct {
	liveAPI           *liveapi.Client
	redisCache        *cache.RedisCacheService
	redisQueue        *queue.RedisQueueService
	vaRepo            *va.Repository
	aircraftSvc       *aircraft.Service
	metrics           *metrics.MetricsRegistry
	vaPatterns        []VAPattern
	lastPatternUpdate time.Time
}

// NewCacheJob creates a new flights cache job
func NewCacheJob(
	liveAPI *liveapi.Client,
	redisCache *cache.RedisCacheService,
	redisQueue *queue.RedisQueueService,
	vaRepo *va.Repository,
	aircraftSvc *aircraft.Service,
	metricsReg *metrics.MetricsRegistry,
) *CacheJob {
	return &CacheJob{
		liveAPI:           liveAPI,
		redisCache:        redisCache,
		redisQueue:        redisQueue,
		vaRepo:            vaRepo,
		aircraftSvc:       aircraftSvc,
		metrics:           metricsReg,
		lastPatternUpdate: time.Now().Add(-10 * time.Minute), // Force initial refresh
	}
}

// Name returns the job name for the scheduler
func (j *CacheJob) Name() string {
	return "FlightsCacheJob"
}

// Run executes the flights cache job
// This job tracks live flights for all active VAs that have callsign prefix/suffix configured.
// It does NOT require Airtable to be enabled - only callsign configuration is needed.
func (j *CacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	logging.Info("Starting flights cache job")

	// 1. Refresh VA patterns every 5 minutes (only depends on callsign config, not Airtable)
	if err := j.refreshVAPatterns(ctx); err != nil {
		logging.Warn("Failed to refresh VA patterns, using cached patterns", "error", err)
	}

	// 2. Get session list from cache
	sessionListVal, found := j.redisCache.Get(cache.KeySessionList)
	if !found {
		logging.Warn("No sessions found in cache, skipping flights cache")
		return nil
	}

	sessionListStr, ok := sessionListVal.(string)
	if !ok {
		logging.Error("Session list is not a string")
		return fmt.Errorf("invalid session list type")
	}

	if sessionListStr == "" {
		logging.Warn("No sessions found in cache, skipping flights cache")
		return nil
	}

	sessionIDs := strings.Split(sessionListStr, "|")

	// 3. Initialize in-memory maps for aggregation
	sessionFlights := make(map[string][]string) // sessionID -> flightIDs
	vaFlights := make(map[string][]string)      // vaID -> flightIDs

	totalFlights := 0
	processedSessions := 0

	// 4. For each session
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}

		// Get session name from cache
		sessionNameVal, found := j.redisCache.Get(cache.SessionNameKey(sessionID))
		sessionName := sessionID // Default to ID
		if found {
			if name, ok := sessionNameVal.(string); ok {
				sessionName = name
			}
		}

		// Fetch flights for this session from Live API
		flightsResp, statusCode, err := j.liveAPI.GetFlights(sessionID)
		if err != nil {
			logging.Error("Failed to get flights from Infinite Live API",
				"sessionID", sessionID,
				"sessionName", sessionName,
				"statusCode", statusCode,
				"error", err,
			)
			continue
		}

		filteredFlights := 0

		// Process each flight
		for _, apiFlight := range flightsResp.Flights {
			// Skip empty callsigns
			if apiFlight.Callsign == "" {
				continue
			}

			// Match flight to VAs - get all matching VA IDs
			matchedVAs := j.matchFlightToVAs(apiFlight.Callsign)
			if len(matchedVAs) == 0 {
				continue // Skip non-VA flights
			}

			// Load or create CompleteFlight
			completeFlight, err := j.buildCompleteFlight(apiFlight, sessionID, sessionName, matchedVAs)
			if err != nil {
				logging.Warn("Failed to build complete flight",
					"flightID", apiFlight.FlightID,
					"error", err,
				)
				continue
			}

			// Append waypoint if needed (checks 100-second interval internally)
			j.appendWaypoint(completeFlight)

			// Cache complete flight with 7-day TTL
			flightKey := cache.LiveFlightKey(apiFlight.FlightID)
			j.redisCache.Set(flightKey, completeFlight, 7*24*time.Hour)

			// Check if we should fetch flight plan based on phase and timing
			// Only enqueue if enough time has passed since last fetch
			shouldFetch, _ := ShouldFetchFlightPlan(completeFlight)
			if shouldFetch {
				// Enqueue flight plan request (non-blocking, errors are logged but don't stop processing)
				if j.redisQueue != nil {
					queueItem := &queue.FlightPlanQueueItem{
						SessionID: sessionID,
						FlightID:  apiFlight.FlightID,
					}
					if err := j.redisQueue.EnqueueFlightPlan(context.Background(), "flight_plan_queue", queueItem); err != nil {
						logging.Warn("Failed to enqueue flight plan request",
							"sessionID", sessionID,
							"flightID", apiFlight.FlightID,
							"error", err,
						)
					} else {
						// Track successful enqueue
						if j.metrics != nil {
							j.metrics.QueueEnqueuedTotal.WithLabelValues("flight_plan_queue", "flight_plan").Inc()
						}
					}
				}
			} else {
				logging.Debug("Skipping flight plan enqueue - too soon",
					"flightID", apiFlight.FlightID,
					"phase", completeFlight.Phase,
					"lastFlightPlanFetch", completeFlight.LastFlightPlanFetch,
				)
			}

			// Add to in-memory aggregation maps
			sessionFlights[sessionID] = append(sessionFlights[sessionID], apiFlight.FlightID)
			for _, vaID := range matchedVAs {
				vaFlights[vaID] = append(vaFlights[vaID], apiFlight.FlightID)
			}

			filteredFlights++
			totalFlights++
		}

		processedSessions++
		logging.Debug("Processed flights for session",
			"sessionID", sessionID,
			"sessionName", sessionName,
			"totalFlights", len(flightsResp.Flights),
			"filteredFlights", filteredFlights,
		)
	}

	// 5. Write session flight lists (atomic, end of run)
	for sessionID, flightIDs := range sessionFlights {
		flightIDsStr := strings.Join(flightIDs, "|")
		sessionKey := cache.LiveFlightsKey(sessionID)
		j.redisCache.Set(sessionKey, flightIDsStr, 5*time.Minute)
	}

	// 6. Write VA flight lists (atomic, end of run)
	for vaID, flightIDs := range vaFlights {
		flightIDsStr := strings.Join(flightIDs, "|")
		vaKey := cache.LiveVAFlightsKey(vaID)
		j.redisCache.Set(vaKey, flightIDsStr, 5*time.Minute)
	}

	duration := time.Since(startTime)
	logging.Info("Flights cache job completed",
		"totalSessions", processedSessions,
		"totalFlights", totalFlights,
		"duration", duration,
	)

	return nil
}

// refreshVAPatterns loads VA callsign patterns from repository and caches in memory
// NOTE: This job ONLY depends on callsign prefix/suffix configuration, NOT on Airtable being enabled.
// Any active VA with callsign_prefix OR callsign_suffix configured will have its flights tracked.
func (j *CacheJob) refreshVAPatterns(ctx context.Context) error {
	// Refresh every 5 minutes
	if time.Since(j.lastPatternUpdate) < 5*time.Minute {
		return nil // Too soon, skip refresh
	}

	// Fetch from repository - only requires active VA with callsign prefix/suffix config
	// Does NOT check is_airtable_enabled or any Airtable configuration
	configs, err := j.vaRepo.GetAllActiveVACallsignConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch VA callsign configs: %w", err)
	}

	// Convert []map[string]string to []VAPattern
	patterns := make([]VAPattern, 0, len(configs))
	for _, config := range configs {
		vaID, ok := config["va_id"]
		if !ok {
			continue // Skip invalid configs
		}

		pattern := VAPattern{
			VAID:   vaID,
			Prefix: config["callsign_prefix"],
			Suffix: config["callsign_suffix"],
		}
		patterns = append(patterns, pattern)
	}

	j.vaPatterns = patterns
	j.lastPatternUpdate = time.Now()

	logging.Info("Refreshed VA callsign patterns", "count", len(patterns))
	return nil
}

// matchFlightToVAs returns all VA IDs that match this callsign
// Matches based on callsign prefix/suffix only - no Airtable dependency
func (j *CacheJob) matchFlightToVAs(callsign string) []string {
	if len(j.vaPatterns) == 0 {
		return nil // Fail closed if no patterns
	}

	matchedVAs := make([]string, 0, 1) // Most flights match 0-1 VAs

	// Check against all VA patterns (single pass)
	// Works with VAs that have prefix only, suffix only, or both
	for _, pattern := range j.vaPatterns {
		// Skip VAs with no pattern requirements (both prefix and suffix empty)
		if pattern.Prefix == "" && pattern.Suffix == "" {
			continue
		}

		// Use platform VA function (handles game suffix stripping)
		// Matches if callsign satisfies the configured prefix/suffix pattern
		if va.MatchesVAPattern(callsign, pattern.Prefix, pattern.Suffix) {
			matchedVAs = append(matchedVAs, pattern.VAID)
		}
	}

	return matchedVAs
}

// buildCompleteFlight creates or updates a CompleteFlight from API data
func (j *CacheJob) buildCompleteFlight(
	apiFlight liveapi.FlightEntry,
	sessionID string,
	sessionName string,
	matchedVAs []string,
) (*CompleteFlight, error) {
	// Try to load existing flight from cache
	flightKey := cache.LiveFlightKey(apiFlight.FlightID)
	var existing *CompleteFlight

	cachedVal, found := j.redisCache.Get(flightKey)
	if found {
		// Convert cached value to CompleteFlight
		jsonBytes, err := json.Marshal(cachedVal)
		if err == nil {
			var cf CompleteFlight
			if err := json.Unmarshal(jsonBytes, &cf); err == nil {
				existing = &cf
			}
		}
	}

	// Parse LastReport time
	lastReport, err := parseLiveAPITime(apiFlight.LastReport)
	if err != nil {
		lastReport = time.Now().UTC() // Fallback to now if parsing fails
	} else {
		lastReport = lastReport.UTC() // Ensure UTC
	}

	// Look up aircraft and livery names from cache using aircraft service
	var aircraftName, liveryName string
	if j.aircraftSvc != nil {
		aircraftName = j.aircraftSvc.GetAircraftNameByID(apiFlight.AircraftID)
		liveryName = j.aircraftSvc.GetLiveryNameByID(apiFlight.LiveryID)
	}

	// Create or update CompleteFlight
	now := time.Now().UTC()
	flight := &CompleteFlight{
		// Core flight data
		FlightID: apiFlight.FlightID,
		Callsign: apiFlight.Callsign,
		UserID:   apiFlight.UserID,
		Username: apiFlight.Username,

		// Session context
		SessionID:   sessionID,
		SessionName: sessionName,

		// Current position data (normalized)
		Latitude:      normalizeCoordinate(apiFlight.Latitude),
		Longitude:     normalizeCoordinate(apiFlight.Longitude),
		Altitude:      normalizeAltitude(apiFlight.Altitude),
		Speed:         normalizeSpeed(apiFlight.Speed),
		Track:         normalizeTrack(apiFlight.Track),
		VerticalSpeed: normalizeVerticalSpeed(apiFlight.VerticalSpeed),

		// Aircraft identifiers
		AircraftID:   apiFlight.AircraftID,
		LiveryID:     apiFlight.LiveryID,
		AircraftName: aircraftName,
		LiveryName:   liveryName,

		// VA associations
		VAIDs: matchedVAs,

		// Metadata
		LastUpdated: now,
		LastReport:  lastReport,
	}

	// Preserve waypoints and flight plan data from existing flight if present
	if existing != nil {
		flight.Waypoints = existing.Waypoints
		// Ensure waypoint timestamps are UTC
		for i := range flight.Waypoints {
			flight.Waypoints[i].Timestamp = flight.Waypoints[i].Timestamp.UTC()
		}
		if !existing.LastUpdatedWaypoint.IsZero() {
			flight.LastUpdatedWaypoint = existing.LastUpdatedWaypoint.UTC()
		}
		flight.Phase = existing.Phase
		if existing.TakeoffTime != nil {
			utcTakeoff := existing.TakeoffTime.UTC()
			flight.TakeoffTime = &utcTakeoff
		}
		if existing.LandingTime != nil {
			utcLanding := existing.LandingTime.UTC()
			flight.LandingTime = &utcLanding
		}
		// Preserve route information and flight plan fetch time (managed by flight plan worker)
		flight.Origin = existing.Origin
		flight.Destination = existing.Destination
		if !existing.LastFlightPlanFetch.IsZero() {
			flight.LastFlightPlanFetch = existing.LastFlightPlanFetch.UTC()
		}
		// Preserve detected_at timestamp (when we first detected this flight)
		if !existing.DetectedAt.IsZero() {
			flight.DetectedAt = existing.DetectedAt.UTC()
		}
		// Preserve aircraft/livery names if cache lookup failed (fallback to existing)
		if flight.AircraftName == "" && existing.AircraftName != "" {
			flight.AircraftName = existing.AircraftName
		}
		if flight.LiveryName == "" && existing.LiveryName != "" {
			flight.LiveryName = existing.LiveryName
		}
	} else {
		// Initialize empty waypoints for new flight
		flight.Waypoints = make([]WaypointSnapshot, 0)
		flight.LastUpdatedWaypoint = time.Time{} // Zero time means no waypoints yet
		flight.Phase = PhaseUnknown
		// Set detected_at timestamp when first creating this flight record
		flight.DetectedAt = now
	}

	// Update flight phase based on current state (use normalized values)
	j.updateFlightPhase(flight, flight.Speed, flight.Altitude)

	return flight, nil
}

// updateFlightPhase determines and updates the flight phase based on speed and altitude
// This is a simplified version that works with CompleteFlight directly
// speed and altitude are already normalized (int types)
func (j *CacheJob) updateFlightPhase(flight *CompleteFlight, speed int, altitude int) {
	now := time.Now().UTC()
	prevPhase := flight.Phase

	var newPhase FlightPhase

	// Handle transition from PhaseUnknown to initial state
	if prevPhase == PhaseUnknown || prevPhase == "" {
		newPhase = j.determineInitialPhase(speed, altitude)
		// Set takeoff time if transitioning to takeoff/climb/cruise/descent
		if newPhase != PhaseOnGround && newPhase != PhaseUnknown && flight.TakeoffTime == nil {
			takeoffTime := now.UTC()
			flight.TakeoffTime = &takeoffTime
		}
		flight.Phase = newPhase
		return
	}

	// Apply phase transition logic for known phases
	// Note: speed is in m/s (int), altitude is in feet (int)
	switch {
	case speed < 50:
		// Aircraft is stationary or taxiing (< 50 m/s = ~97 knots)
		if prevPhase == PhaseDescent || prevPhase == PhaseCruise || prevPhase == PhaseClimb {
			newPhase = PhaseLanded
			// Only set landing time if not already set
			if flight.LandingTime == nil {
				landingTime := now.UTC()
				flight.LandingTime = &landingTime
			}
		} else {
			newPhase = PhaseOnGround
		}

	case prevPhase == PhaseOnGround && speed > 80:
		// Aircraft is accelerating for takeoff (> 80 m/s = ~155 knots)
		newPhase = PhaseTakeoff
		// Only set takeoff time if not already set
		if flight.TakeoffTime == nil {
			takeoffTime := now.UTC()
			flight.TakeoffTime = &takeoffTime
		}

	case prevPhase == PhaseTakeoff && altitude > 8000:
		// Aircraft has climbed above 8000 feet
		newPhase = PhaseClimb

	case prevPhase == PhaseClimb && (altitude > 30000 || speed > 300):
		// Aircraft reached cruise altitude (>30000 ft) or speed (>300 m/s = ~583 knots)
		newPhase = PhaseCruise

	case prevPhase == PhaseCruise && altitude < 15000:
		// Aircraft is descending below 15000 feet
		newPhase = PhaseDescent

	default:
		// Maintain previous phase if no transition condition met
		newPhase = prevPhase
		if newPhase == "" {
			newPhase = PhaseUnknown
		}
	}

	flight.Phase = newPhase
}

// determineInitialPhase infers the initial flight phase from speed and altitude
// when transitioning from PhaseUnknown
// speed is in m/s (int), altitude is in feet (int)
func (j *CacheJob) determineInitialPhase(speed int, altitude int) FlightPhase {
	switch {
	case speed < 50:
		// Low speed indicates on ground (< 50 m/s = ~97 knots)
		return PhaseOnGround

	case altitude > 30000 || speed > 300:
		// High altitude (>30000 ft) or speed (>300 m/s = ~583 knots) indicates cruise
		return PhaseCruise

	case altitude > 8000 && speed > 80:
		// High altitude (>8000 ft) with good speed (>80 m/s = ~155 knots) indicates climb
		return PhaseClimb

	case altitude < 15000 && speed > 50 && altitude > 1000:
		// Medium altitude (1000-15000 ft) with speed (>50 m/s) could be descent
		return PhaseDescent

	case speed > 80 && altitude < 1000:
		// Low altitude (<1000 ft) with speed (>80 m/s) indicates takeoff
		return PhaseTakeoff

	case speed > 80:
		// Good speed (>80 m/s) but unclear altitude - default to takeoff
		return PhaseTakeoff

	default:
		// Cannot determine - remain unknown
		return PhaseUnknown
	}
}

// appendWaypoint appends a waypoint snapshot if >100 seconds elapsed since last waypoint
func (j *CacheJob) appendWaypoint(flight *CompleteFlight) {
	now := time.Now().UTC()

	// Check if 100+ seconds since last waypoint
	// If LastUpdatedWaypoint is zero time, this is the first waypoint
	if !flight.LastUpdatedWaypoint.IsZero() {
		if time.Since(flight.LastUpdatedWaypoint) < 100*time.Second {
			return // Too soon, skip
		}
	}

	// Create new waypoint snapshot (values are already normalized in CompleteFlight)
	waypoint := WaypointSnapshot{
		Timestamp: now.UTC(),
		Latitude:  flight.Latitude,  // Already normalized to 4 decimals
		Longitude: flight.Longitude, // Already normalized to 4 decimals
		Altitude:  flight.Altitude,  // Already normalized to feet (int)
		Speed:     flight.Speed,     // Already normalized to m/s (int)
		Track:     flight.Track,     // Already normalized to 1 decimal
	}

	// Append to flight's waypoints array
	flight.Waypoints = append(flight.Waypoints, waypoint)
	flight.LastUpdatedWaypoint = now.UTC()

	// Prune to max 600 waypoints (~20 hours worth)
	// Keep most recent 600
	if len(flight.Waypoints) > 600 {
		flight.Waypoints = flight.Waypoints[len(flight.Waypoints)-600:]
	}
}
