package flights

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
	gameflights "infinite-experiment/politburo/internal/game/flights"
	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	gamesessions "infinite-experiment/politburo/internal/game/sessions"
	"infinite-experiment/politburo/internal/infiniteflight"
)

type flightsClientStub struct {
	bySession map[string][]infiniteflight.Flight
	errByID   map[string]error
}

func (s flightsClientStub) GetSessionFlights(_ context.Context, sessionID string) ([]infiniteflight.Flight, error) {
	if err := s.errByID[sessionID]; err != nil {
		return nil, err
	}
	return s.bySession[sessionID], nil
}

type cacheWrite struct {
	key   string
	value any
	ttl   time.Duration
}

type cacheStub struct {
	data     map[string]any
	writes   []cacheWrite
	setError error
}

func (s *cacheStub) GetJSON(_ context.Context, key string, destination any) error {
	value, ok := s.data[key]
	if !ok {
		return cache.ErrMiss
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, destination)
}

func (s *cacheStub) SetJSON(_ context.Context, key string, value any, ttl time.Duration) error {
	if s.setError != nil {
		return s.setError
	}
	s.writes = append(s.writes, cacheWrite{key: key, value: value, ttl: ttl})
	if s.data == nil {
		s.data = map[string]any{}
	}
	s.data[key] = value
	return nil
}

func (s *cacheStub) write(key string) (cacheWrite, bool) {
	for i := len(s.writes) - 1; i >= 0; i-- {
		if s.writes[i].key == key {
			return s.writes[i], true
		}
	}
	return cacheWrite{}, false
}

func withSessions(sessions []infiniteflight.Session) map[string]any {
	return map[string]any{
		cache.KeyActiveSessions: gamesessions.Snapshot{Result: sessions, LastCached: time.Now().UTC()},
	}
}

func TestJobRunSkipsWhenSessionsMissing(t *testing.T) {
	store := &cacheStub{data: map[string]any{}}
	job := New(flightsClientStub{}, store)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(store.writes) != 0 {
		t.Fatalf("writes = %#v", store.writes)
	}
}

func TestJobRunCachesFullSnapshot(t *testing.T) {
	liveryID := "df597aaf-456c-4878-9d84-45201f2aae74"
	store := &cacheStub{data: withSessions([]infiniteflight.Session{{ID: "session-1", Name: "Casual", NormalizedName: "casual"}})}
	store.data[cache.KeyLivery(liveryID)] = gameliveries.Livery{ID: liveryID, AircraftName: "A350", LiveryName: "Swiss"}
	job := New(flightsClientStub{bySession: map[string][]infiniteflight.Flight{
		"session-1": {{FlightID: "f1", Callsign: "Swiss 39 Heavy", Speed: 525.6, LiveryID: liveryID, LastReport: "2026-08-15 05:09:53Z", PilotState: 3}},
	}}, store)
	now := time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC)
	job.now = func() time.Time { return now }

	if job.Name() != "infinite-flight-flights" {
		t.Fatalf("Name() = %q", job.Name())
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	write, ok := store.write(cache.KeyActiveFlights("casual"))
	if !ok {
		t.Fatalf("missing write; writes = %#v", store.writes)
	}
	if write.ttl != gameflights.GameActiveFlightTTL {
		t.Fatalf("ttl = %s", write.ttl)
	}
	snapshot := write.value.(gameflights.Snapshot)
	if !snapshot.LastCached.Equal(now) || len(snapshot.Result) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	flight := snapshot.Result[0]
	if flight.AircraftName != "A350" || flight.LiveryName != "Swiss" || flight.Normalized.Speed != "526 kts" {
		t.Fatalf("flight = %#v", flight)
	}
	if flight.PathSync == nil || flight.PathSync.FPLSyncRequired || flight.History != nil {
		t.Fatalf("pathSync/history = %#v", flight)
	}
}

func TestJobRunAppendsHistoryUnderFlightKey(t *testing.T) {
	existing := gameflights.Snapshot{
		LastCached: time.Date(2026, time.August, 15, 5, 59, 0, 0, time.UTC),
		Result: []gameflights.Flight{{
			FlightID: "f1", Callsign: "prior", Speed: 400,
			PathSync: &gameflights.PathSync{FPLSyncRequired: false},
			History:  []gameflights.Flight{{FlightID: "f1", Callsign: "older"}},
		}},
	}
	store := &cacheStub{data: withSessions([]infiniteflight.Session{{ID: "session-1", NormalizedName: "casual"}})}
	store.data[cache.KeyActiveFlights("casual")] = existing
	job := New(flightsClientStub{bySession: map[string][]infiniteflight.Flight{
		"session-1": {{FlightID: "f1", Callsign: "current", Speed: 410}},
	}}, store)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	snapshotWrite, ok := store.write(cache.KeyActiveFlights("casual"))
	if !ok {
		t.Fatalf("missing snapshot write")
	}
	snapshot := snapshotWrite.value.(gameflights.Snapshot)
	if snapshot.Result[0].History != nil {
		t.Fatalf("snapshot still embeds history = %#v", snapshot.Result[0].History)
	}
	historyWrite, ok := store.write(cache.KeyFlightHistory("f1"))
	if !ok {
		t.Fatalf("missing history write; writes = %#v", store.writes)
	}
	if historyWrite.ttl != gameflights.GameActiveFlightTTL {
		t.Fatalf("history ttl = %s", historyWrite.ttl)
	}
	history := historyWrite.value.(gameflights.HistorySnapshot)
	if len(history.Result) != 2 {
		t.Fatalf("history = %#v", history.Result)
	}
	newest := history.Result[1]
	if newest.Callsign != "prior" || newest.History != nil || newest.PathSync != nil {
		t.Fatalf("historical copy = %#v", newest)
	}
}

func TestJobRunAppendsToExistingHistoryKey(t *testing.T) {
	existing := gameflights.Snapshot{
		LastCached: time.Date(2026, time.August, 15, 5, 59, 0, 0, time.UTC),
		Result:     []gameflights.Flight{{FlightID: "f1", Callsign: "prior", Speed: 400}},
	}
	store := &cacheStub{data: withSessions([]infiniteflight.Session{{ID: "session-1", NormalizedName: "casual"}})}
	store.data[cache.KeyActiveFlights("casual")] = existing
	store.data[cache.KeyFlightHistory("f1")] = gameflights.HistorySnapshot{
		Result: []gameflights.Flight{{FlightID: "f1", Callsign: "older"}},
	}
	job := New(flightsClientStub{bySession: map[string][]infiniteflight.Flight{
		"session-1": {{FlightID: "f1", Callsign: "current", Speed: 410}},
	}}, store)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	historyWrite, ok := store.write(cache.KeyFlightHistory("f1"))
	if !ok {
		t.Fatalf("missing history write")
	}
	history := historyWrite.value.(gameflights.HistorySnapshot)
	if len(history.Result) != 2 || history.Result[0].Callsign != "older" || history.Result[1].Callsign != "prior" {
		t.Fatalf("history = %#v", history.Result)
	}
}

func TestJobRunContinuesAfterSessionError(t *testing.T) {
	store := &cacheStub{data: withSessions([]infiniteflight.Session{
		{ID: "bad", NormalizedName: "expert"},
		{ID: "good", NormalizedName: "casual"},
	})}
	job := New(flightsClientStub{
		bySession: map[string][]infiniteflight.Flight{"good": {{FlightID: "f1", Callsign: "ok"}}},
		errByID:   map[string]error{"bad": errors.New("upstream")},
	}, store)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, ok := store.write(cache.KeyActiveFlights("expert")); ok {
		t.Fatalf("expert snapshot should be left in place")
	}
	if _, ok := store.write(cache.KeyActiveFlights("casual")); !ok {
		t.Fatalf("missing casual snapshot; writes = %#v", store.writes)
	}
}

func TestJobRunWarnsAtCap(t *testing.T) {
	upstream := make([]infiniteflight.Flight, gameflights.MaxFlightsPerRequest)
	for i := range upstream {
		upstream[i].FlightID = "flight-" + strconv.Itoa(i)
	}
	store := &cacheStub{data: withSessions([]infiniteflight.Session{{ID: "session-1", NormalizedName: "casual"}})}
	job := New(flightsClientStub{bySession: map[string][]infiniteflight.Flight{"session-1": upstream}}, store)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	write, ok := store.write(cache.KeyActiveFlights("casual"))
	if !ok {
		t.Fatalf("missing write")
	}
	snapshot := write.value.(gameflights.Snapshot)
	if len(snapshot.Result) != gameflights.MaxFlightsPerRequest {
		t.Fatalf("result length = %d", len(snapshot.Result))
	}
}
