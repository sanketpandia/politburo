package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"strings"
	"time"

	"go.uber.org/zap"
)

// FlightPhase represents the current phase of a flight
type FlightPhase string

const (
	PhaseOnGround FlightPhase = "on_ground"
	PhaseTakeoff  FlightPhase = "takeoff"
	PhaseClimb    FlightPhase = "climb"
	PhaseCruise   FlightPhase = "cruise"
	PhaseDescent  FlightPhase = "descent"
	PhaseLanded   FlightPhase = "landed"
	PhaseUnknown  FlightPhase = "unknown"
)

// FlightState tracks the state and metadata of a flight for intelligent caching
type FlightState struct {
	FlightID     string      `json:"flight_id"`
	Callsign     string      `json:"callsign"`
	Phase        FlightPhase `json:"phase"`
	LastSpeed    float64     `json:"last_speed"`
	LastAltitude float64     `json:"last_altitude"`
	TakeoffTime  *time.Time  `json:"takeoff_time,omitempty"`
	LandingTime  *time.Time  `json:"landing_time,omitempty"`
	LastUpdated  time.Time   `json:"last_updated"`
	NextPollTime time.Time   `json:"next_poll_time"`
	LastPhase    FlightPhase `json:"last_phase,omitempty"`
}

// FlightsCacheJob syncs live flight data with intelligent polling based on flight phase
// Implements the strategy described in docs/dev/live_flights.md
type FlightsCacheJob struct {
	liveAPISvc        *common.LiveAPIService
	redisSvc          *common.RedisCacheService
	vaConfigSvc       *common.VAConfigService
	logger            *zap.SugaredLogger
	lastRunTime       time.Time
	vaPatterns        []map[string]string // Cached VA callsign patterns
	lastPatternUpdate time.Time
}

// NewFlightsCacheJob creates a new flights cache job
func NewFlightsCacheJob(liveAPISvc *common.LiveAPIService, redisSvc *common.RedisCacheService, vaConfigSvc *common.VAConfigService, logger *zap.SugaredLogger) *FlightsCacheJob {
	return &FlightsCacheJob{
		liveAPISvc:        liveAPISvc,
		redisSvc:          redisSvc,
		vaConfigSvc:       vaConfigSvc,
		logger:            logger,
		lastPatternUpdate: time.Now().Add(-20 * time.Minute),
	}
}

// Run executes the flights cache job
func (j *FlightsCacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	j.logger.Info("Starting flights cache job")

	// Refresh VA patterns every 5 minutes
	if time.Since(j.lastPatternUpdate) > 5*time.Minute {
		patterns, err := j.vaConfigSvc.GetAllCallsigns(ctx)
		if err != nil {
			j.logger.Warnw("Failed to fetch VA callsign patterns, using cached patterns", "error", err)
		} else {
			j.vaPatterns = patterns
			j.lastPatternUpdate = time.Now()
			j.logger.Debugw("Refreshed VA callsign patterns", "count", len(patterns))
		}
	}

	// Get session list from cache
	sessionListVal, found := j.redisSvc.Get("if:sessions")
	if !found {
		j.logger.Warn("No sessions found in cache, skipping flights cache")
		return nil
	}

	sessionListStr, ok := sessionListVal.(string)
	if !ok {
		j.logger.Error("Session list is not a string")
		return fmt.Errorf("invalid session list type")
	}

	if sessionListStr == "" {
		j.logger.Warn("No sessions found in cache, skipping flights cache")
		return nil
	}

	sessionIDs := strings.Split(sessionListStr, "|")
	totalFlights := 0
	processedSessions := 0

	// Fetch flights for each session
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}

		// Get session name from cache
		sessionNameVal, found := j.redisSvc.Get(fmt.Sprintf("if:session:name:%s", sessionID))
		sessionName := sessionID // Default to ID
		if found {
			if name, ok := sessionNameVal.(string); ok {
				sessionName = name
			}
		}

		// Fetch flights for this session from Live API
		flightsResp, statusCode, err := j.liveAPISvc.GetFlights(sessionID)
		if err != nil {
			j.logger.Errorw("Failed to get flights from Infinite Live API",
				"sessionID", sessionID,
				"sessionName", sessionName,
				"statusCode", statusCode,
				"error", err,
			)
			continue
		}

		callsigns := make([]string, 0, len(flightsResp.Flights))
		filteredFlights := 0

		// Process and cache each flight
		for _, flight := range flightsResp.Flights {
			// Skip empty callsigns
			if flight.Callsign == "" {
				continue
			}

			// Filter by VA pattern - only cache flights matching known VA patterns
			if !j.matchesAnyVAPattern(flight.Callsign) {
				continue
			}

			callsigns = append(callsigns, flight.Callsign)
			filteredFlights++

			// Determine flight phase based on altitude and speed
			phase := j.determineFlightPhase(ctx, flight.FlightID, flight.Speed, flight.Altitude)

			// Create enhanced flight data with session context and phase
			enhancedFlight := map[string]interface{}{
				"flight_id":    flight.FlightID,
				"callsign":     flight.Callsign,
				"session_id":   sessionID,
				"session_name": sessionName,
				"speed":        flight.Speed,
				"altitude":     flight.Altitude,
				"latitude":     flight.Latitude,
				"longitude":    flight.Longitude,
				"track":        flight.Track,
				"phase":        phase,
				"last_updated": time.Now(),
			}

			// Cache flight with 5-minute TTL (will be refreshed based on phase)
			cacheKey := fmt.Sprintf("if:flight:%s", flight.FlightID)
			j.redisSvc.Set(cacheKey, enhancedFlight, 5*time.Minute)

			// Also cache by session+callsign for quick lookups
			sessionCallsignKey := fmt.Sprintf("if:session:%s:flight:%s", sessionID, flight.Callsign)
			j.redisSvc.Set(sessionCallsignKey, enhancedFlight, 5*time.Minute)

			totalFlights++
		}

		// Cache callsign list for this session (5-minute TTL for live data)
		callsignStr := strings.Join(callsigns, "|")
		callsignKey := fmt.Sprintf("if:session:callsigns:%s", sessionID)
		j.redisSvc.Set(callsignKey, callsignStr, 5*time.Minute)

		processedSessions++
		j.logger.Debugw("Cached flights for session",
			"sessionID", sessionID,
			"sessionName", sessionName,
			"totalFlights", len(flightsResp.Flights),
			"filteredFlights", filteredFlights,
		)
	}

	duration := time.Since(startTime)
	j.lastRunTime = time.Now()

	j.logger.Infow("Flights cache job completed",
		"totalSessions", processedSessions,
		"totalFlights", totalFlights,
		"duration", duration,
	)

	return nil
}

// determineFlightPhase calculates the current flight phase based on speed and altitude
// Implements the logic from docs/dev/live_flights.md
func (j *FlightsCacheJob) determineFlightPhase(ctx context.Context, flightID string, speed, altitude float64) FlightPhase {
	// Get previous flight state from cache
	stateKey := fmt.Sprintf("if:flight:state:%s", flightID)
	var prevState FlightState

	// Try to get cached state - Get returns interface{} which needs conversion
	cachedVal, found := j.redisSvc.Get(stateKey)
	if !found {
		// First time seeing this flight, determine initial phase
		if speed < 50 {
			return PhaseOnGround
		}
		return PhaseUnknown
	}

	// Convert cached value back to FlightState struct
	// The cache stores as JSON, so we need to re-marshal and unmarshal
	jsonBytes, err := json.Marshal(cachedVal)
	if err != nil {
		j.logger.Warnw("Failed to marshal cached flight state", "error", err)
		if speed < 50 {
			return PhaseOnGround
		}
		return PhaseUnknown
	}
	if err := json.Unmarshal(jsonBytes, &prevState); err != nil {
		j.logger.Warnw("Failed to unmarshal cached flight state", "error", err)
		if speed < 50 {
			return PhaseOnGround
		}
		return PhaseUnknown
	}

	var newPhase FlightPhase
	now := time.Now()

	// Apply phase transition logic from docs
	switch {
	case speed < 50:
		// Aircraft is stationary or taxiing
		if prevState.Phase == PhaseDescent || prevState.Phase == PhaseCruise || prevState.Phase == PhaseClimb {
			newPhase = PhaseLanded
			landingTime := now
			prevState.LandingTime = &landingTime
		} else {
			newPhase = PhaseOnGround
		}

	case prevState.Phase == PhaseOnGround && speed > 80:
		// Aircraft is accelerating for takeoff
		newPhase = PhaseTakeoff
		takeoffTime := now
		prevState.TakeoffTime = &takeoffTime

	case prevState.Phase == PhaseTakeoff && altitude > 8000:
		// Aircraft has climbed above 8000 feet
		newPhase = PhaseClimb

	case prevState.Phase == PhaseClimb && (altitude > 30000 || speed > 300):
		// Aircraft reached cruise altitude or speed
		newPhase = PhaseCruise

	case prevState.Phase == PhaseCruise && altitude < 15000:
		// Aircraft is descending
		newPhase = PhaseDescent

	default:
		// Maintain previous phase if no transition condition met
		newPhase = prevState.Phase
		if newPhase == "" {
			newPhase = PhaseUnknown
		}
	}

	// Update and save flight state
	prevState.FlightID = flightID
	prevState.Phase = newPhase
	prevState.LastSpeed = speed
	prevState.LastAltitude = altitude
	prevState.LastUpdated = now

	// Calculate next poll time based on phase
	prevState.NextPollTime = j.calculateNextPollTime(newPhase, now)

	// Save updated state to cache (1-hour TTL to handle flights that disappear)
	j.redisSvc.Set(stateKey, prevState, 1*time.Hour)

	return newPhase
}

// calculateNextPollTime determines when to next poll this flight based on its phase
// Implements intelligent polling intervals from docs/dev/live_flights.md
func (j *FlightsCacheJob) calculateNextPollTime(phase FlightPhase, now time.Time) time.Time {
	switch phase {
	case PhaseOnGround, PhaseLanded:
		// Poll every 2 minutes when on ground
		return now.Add(2 * time.Minute)
	case PhaseTakeoff, PhaseClimb, PhaseCruise, PhaseDescent:
		// Poll every 5 minutes during active flight phases
		return now.Add(5 * time.Minute)
	default:
		// Unknown phase, poll more frequently
		return now.Add(1 * time.Minute)
	}
}

// RunScheduled runs the flights cache job on a schedule
func (j *FlightsCacheJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				j.logger.Errorw("Flights cache job failed", "error", err)
			}
		case <-ctx.Done():
			j.logger.Info("Flights cache job stopped")
			return
		}
	}
}

// GetLastRunTime returns the last time this job ran successfully
func (j *FlightsCacheJob) GetLastRunTime() time.Time {
	return j.lastRunTime
}

// matchesAnyVAPattern checks if a callsign matches any VA's prefix/suffix pattern
func (j *FlightsCacheJob) matchesAnyVAPattern(callsign string) bool {
	// If no patterns loaded yet, allow all flights (fail open)
	if len(j.vaPatterns) == 0 {
		return true
	}

	// Check against each VA pattern
	for _, pattern := range j.vaPatterns {
		prefix := pattern["callsign_prefix"]
		suffix := pattern["callsign_suffix"]

		// If VA has no pattern requirements, skip it
		if prefix == "" && suffix == "" {
			continue
		}

		// Check if callsign matches this VA's pattern
		if common.MatchesVAPattern(callsign, prefix, suffix) {
			return true
		}
	}

	return false
}
