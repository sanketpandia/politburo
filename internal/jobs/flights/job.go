package flights

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"infinite-experiment/politburo/internal/cache"
	gameflights "infinite-experiment/politburo/internal/game/flights"
	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	gamesessions "infinite-experiment/politburo/internal/game/sessions"
	"infinite-experiment/politburo/internal/infiniteflight"
)

const jobName = "infinite-flight-flights"

type Job struct {
	client infiniteflight.FlightsClient
	cache  cache.Store
	now    func() time.Time
}

func New(client infiniteflight.FlightsClient, cacheStore cache.Store) *Job {
	return &Job{client: client, cache: cacheStore, now: time.Now}
}

func (j *Job) Name() string {
	return jobName
}

func (j *Job) Run(ctx context.Context) error {
	var sessionsSnapshot gamesessions.Snapshot
	if err := j.cache.GetJSON(ctx, cache.KeyActiveSessions, &sessionsSnapshot); err != nil {
		slog.Warn("skipping flights refresh; active sessions cache unavailable", "error", err)
		return nil
	}
	if len(sessionsSnapshot.Result) == 0 {
		slog.Warn("skipping flights refresh; no active sessions cached")
		return nil
	}

	refreshedAt := j.now().UTC()
	liveries := make(map[string]gameliveries.Livery)
	totalFlights := 0
	for _, session := range sessionsSnapshot.Result {
		if session.ID == "" || session.NormalizedName == "" {
			continue
		}
		count, err := j.refreshSession(ctx, session, liveries, refreshedAt)
		if err != nil {
			slog.Error("failed to refresh session flights", "sessionId", session.ID, "server", session.NormalizedName, "error", err)
			continue
		}
		totalFlights += count
	}
	slog.Info("Infinite Flight flights refreshed", "sessions", len(sessionsSnapshot.Result), "flights", totalFlights)
	return nil
}

func (j *Job) refreshSession(ctx context.Context, session infiniteflight.Session, liveries map[string]gameliveries.Livery, refreshedAt time.Time) (int, error) {
	upstream, err := j.client.GetSessionFlights(ctx, session.ID)
	if err != nil {
		return 0, fmt.Errorf("fetch flights: %w", err)
	}
	if len(upstream) >= gameflights.MaxFlightsPerRequest {
		slog.Warn("Infinite Flight flights response hit the max record cap", "sessionId", session.ID, "server", session.NormalizedName, "flights", len(upstream))
	}

	var existing gameflights.Snapshot
	if err := j.cache.GetJSON(ctx, cache.KeyActiveFlights(session.NormalizedName), &existing); err != nil {
		if err != cache.ErrMiss {
			slog.Warn("failed to read existing flights snapshot; treating as empty", "server", session.NormalizedName, "error", err)
		}
		existing.Result = nil
	}

	existingByID := make(map[string]*gameflights.Flight, len(existing.Result))
	for i := range existing.Result {
		existingByID[existing.Result[i].FlightID] = &existing.Result[i]
	}

	mapped := make([]gameflights.Flight, 0, len(upstream))
	for _, item := range upstream {
		livery := j.lookupLivery(ctx, liveries, item.LiveryID)
		mapped = append(mapped, gameflights.MapFlight(item, session, livery, existingByID[item.FlightID], refreshedAt))
	}

	snapshot := gameflights.Snapshot{
		Result:     gameflights.UpsertFlights(existing.Result, mapped),
		LastCached: refreshedAt,
	}
	if snapshot.Result == nil {
		snapshot.Result = make([]gameflights.Flight, 0)
	}
	j.cacheHistories(ctx, existingByID, snapshot.Result)
	if err := j.cache.SetJSON(ctx, cache.KeyActiveFlights(session.NormalizedName), snapshot, gameflights.GameActiveFlightTTL); err != nil {
		return 0, fmt.Errorf("cache flights: %w", err)
	}
	return len(snapshot.Result), nil
}

func (j *Job) cacheHistories(ctx context.Context, existingByID map[string]*gameflights.Flight, current []gameflights.Flight) {
	for i := range current {
		flight := current[i]
		if flight.FlightID == "" {
			continue
		}
		prior, hadPrior := existingByID[flight.FlightID]
		if !hadPrior {
			continue
		}
		history := j.loadHistory(ctx, flight.FlightID, prior)
		next := gameflights.HistorySnapshot{Result: gameflights.NextHistory(history, *prior)}
		if err := j.cache.SetJSON(ctx, cache.KeyFlightHistory(flight.FlightID), next, gameflights.GameActiveFlightTTL); err != nil {
			slog.Error("failed to cache flight history", "flightId", flight.FlightID, "error", err)
		}
	}
}

func (j *Job) loadHistory(ctx context.Context, flightID string, prior *gameflights.Flight) []gameflights.Flight {
	var snapshot gameflights.HistorySnapshot
	if err := j.cache.GetJSON(ctx, cache.KeyFlightHistory(flightID), &snapshot); err != nil {
		if err != cache.ErrMiss {
			slog.Warn("failed to read existing flight history; treating as empty", "flightId", flightID, "error", err)
		}
		if prior != nil && len(prior.History) > 0 {
			return prior.History
		}
		return nil
	}
	return snapshot.Result
}

func (j *Job) lookupLivery(ctx context.Context, liveries map[string]gameliveries.Livery, liveryID string) *gameliveries.Livery {
	if liveryID == "" {
		return nil
	}
	if cached, ok := liveries[liveryID]; ok {
		if cached.ID == "" {
			return nil
		}
		return &cached
	}
	var livery gameliveries.Livery
	if err := j.cache.GetJSON(ctx, cache.KeyLivery(liveryID), &livery); err != nil {
		liveries[liveryID] = gameliveries.Livery{}
		return nil
	}
	liveries[liveryID] = livery
	return &livery
}
