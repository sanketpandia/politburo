package sessions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"infinite-experiment/politburo/internal/cache"
	gamesessions "infinite-experiment/politburo/internal/game/sessions"
	"infinite-experiment/politburo/internal/infiniteflight"
)

const jobName = "infinite-flight-sessions"

type Job struct {
	client infiniteflight.SessionsClient
	cache  cache.Store
	now    func() time.Time
}

func New(client infiniteflight.SessionsClient, cacheStore cache.Store) *Job {
	return &Job{client: client, cache: cacheStore, now: time.Now}
}

func (j *Job) Name() string {
	return jobName
}

func (j *Job) Run(ctx context.Context) error {
	sessions, err := j.client.GetSessions(ctx)
	if err != nil {
		return fmt.Errorf("refresh sessions: %w", err)
	}

	var existingSnapshot gamesessions.Snapshot
	hasExistingSnapshot := true
	if err := j.cache.GetJSON(ctx, cache.KeyActiveSessions, &existingSnapshot); err != nil {
		// A failed read must not prevent fresh upstream data from replacing a
		// missing, expired, corrupt, or temporarily unavailable cache entry.
		hasExistingSnapshot = false
		slog.Warn("failed to read existing sessions from cache; resetting history", "error", err)
	}

	// time.Time marshals as an ISO 8601/RFC 3339 timestamp. Capture it once so
	// every session and its enclosing snapshot have the same refresh time.
	refreshedAt := j.now().UTC()
	sessions = prepareCurrentSessions(sessions, refreshedAt)

	snapshot := gamesessions.Snapshot{
		Result:     sessions,
		LastCached: refreshedAt,
	}
	if hasExistingSnapshot && !existingSnapshot.LastCached.IsZero() {
		snapshot.History = make([]gamesessions.Snapshot, 0, len(existingSnapshot.History)+1)
		for _, historicalSnapshot := range existingSnapshot.History {
			// History entries must remain flat instead of recursively embedding
			// all of their predecessors.
			historicalSnapshot.History = nil
			snapshot.History = append(snapshot.History, historicalSnapshot)
		}
		existingSnapshot.History = nil
		snapshot.History = append(snapshot.History, existingSnapshot)
		if len(snapshot.History) > 50 {
			snapshot.History = snapshot.History[len(snapshot.History)-50:]
		}
	}

	if err := j.cache.SetJSON(ctx, cache.KeyActiveSessions, snapshot, gamesessions.CacheTTL); err != nil {
		return fmt.Errorf("cache sessions: %w", err)
	}

	sessionNames := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionNames = append(sessionNames, session.NormalizedName)
	}
	if err := j.cache.SetJSON(ctx, cache.KeySessionNames, sessionNames, 0); err != nil {
		return fmt.Errorf("cache session names: %w", err)
	}

	userCount := 0
	for _, session := range sessions {
		userCount += session.UserCount
	}
	slog.Info("Infinite Flight sessions refreshed", "sessions", len(sessions), "users", userCount)
	return nil
}

// prepareCurrentSessions creates a new result slice containing only the latest
// upstream value for each session. Historical data is managed solely through
// Snapshot.History and must never leak into Snapshot.Result.
func prepareCurrentSessions(upstream []infiniteflight.Session, refreshedAt time.Time) []infiniteflight.Session {
	current := make([]infiniteflight.Session, 0, len(upstream))
	indexes := make(map[string]int, len(upstream))
	for _, session := range upstream {
		session.Timestamp = refreshedAt
		session.NormalizedName = infiniteflight.NormalizeSessionName(session.Name)
		identity := session.ID
		if identity == "" {
			identity = "name:" + session.NormalizedName
		}
		if index, exists := indexes[identity]; exists {
			current[index] = session
			continue
		}
		indexes[identity] = len(current)
		current = append(current, session)
	}
	return current
}
