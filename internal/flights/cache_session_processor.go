package flights

import (
	"context"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/sessions"
)

const flightPlanQueueName = "flight_plan_queue"

func (j *CacheJob) processSession(ctx context.Context, sessionID string) FlightAggregation {
	result := NewFlightAggregation()
	if sessionID == "" {
		return result
	}

	sessionName := sessions.GetSessionName(j.redisCache, sessionID)
	flightsResp, statusCode, err := j.liveAPI.GetFlights(sessionID)
	if err != nil {
		j.recordCacheJobFailure("liveapi_flights")
		logging.Error("Failed to get flights from Infinite Live API",
			"sessionName", sessionName,
			"statusCode", statusCode,
			"error", err,
		)
		return result
	}

	filteredFlights := 0
	for _, apiFlight := range flightsResp.Flights {
		if apiFlight.Callsign == "" {
			continue
		}

		matchedVAs := j.matchFlightToVAs(apiFlight.Callsign)
		matchedVAs = j.filterVAsForSession(ctx, matchedVAs, sessionID)
		if len(matchedVAs) == 0 {
			continue
		}

		completeFlight, err := j.buildCompleteFlight(apiFlight, sessionID, sessionName, matchedVAs)
		if err != nil {
			j.recordCacheJobFailure("build_flight")
			logging.Warn("Failed to build complete flight", "error", err)
			continue
		}

		appendWaypoint(completeFlight)
		j.redisCache.Set(cache.LiveFlightKey(apiFlight.FlightID), completeFlight, cache.LiveFlightTTL)
		j.enqueueFlightPlanIfDue(ctx, sessionID, apiFlight.FlightID, completeFlight)

		result.AddFlight(sessionID, apiFlight.FlightID, matchedVAs)
		filteredFlights++
	}

	result.ProcessedSessions = 1
	logging.Debug("Processed flights for session",
		"sessionName", sessionName,
		"totalFlights", len(flightsResp.Flights),
		"filteredFlights", filteredFlights,
	)
	return result
}

func (j *CacheJob) enqueueFlightPlanIfDue(ctx context.Context, sessionID string, flightID string, flight *CompleteFlight) {
	shouldFetch, _ := ShouldFetchFlightPlan(flight)
	if !shouldFetch {
		logging.Debug("Skipping flight plan enqueue",
			"phase", flight.Phase,
			"lastFlightPlanFetch", flight.LastFlightPlanFetch,
		)
		return
	}
	if j.redisQueue == nil {
		return
	}

	queueItem := &queue.FlightPlanQueueItem{SessionID: sessionID, FlightID: flightID}
	if err := j.redisQueue.EnqueueFlightPlan(ctx, flightPlanQueueName, queueItem); err != nil {
		j.recordCacheJobFailure("enqueue_flight_plan")
		logging.Warn("Failed to enqueue flight plan request", "error", err)
		return
	}
	if j.metrics != nil {
		j.metrics.QueueEnqueuedTotal.WithLabelValues(flightPlanQueueName, "flight_plan").Inc()
	}
}
