package liveries

import (
	"context"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	"infinite-experiment/politburo/internal/infiniteflight"
)

type liveriesClientStub struct {
	liveries []infiniteflight.Livery
	err      error
}

func (s liveriesClientStub) GetAircraftLiveries(context.Context) ([]infiniteflight.Livery, error) {
	return s.liveries, s.err
}

type cacheWrite struct {
	key   string
	value any
	ttl   time.Duration
}

type cacheStub struct {
	writes   []cacheWrite
	setError error
}

func (s *cacheStub) GetJSON(context.Context, string, any) error { return cache.ErrMiss }

func (s *cacheStub) SetJSON(_ context.Context, key string, value any, ttl time.Duration) error {
	if s.setError != nil {
		return s.setError
	}
	s.writes = append(s.writes, cacheWrite{key: key, value: value, ttl: ttl})
	return nil
}

func TestJobRunCachesLiveriesByID(t *testing.T) {
	store := &cacheStub{}
	job := New(liveriesClientStub{liveries: []infiniteflight.Livery{{
		ID: "df597aaf-456c-4878-9d84-45201f2aae74", AircraftID: "aircraft-1", AircraftName: "A350", LiveryName: "Swiss",
	}}}, store)
	if job.Name() != "infinite-flight-liveries" {
		t.Fatalf("Name() = %q", job.Name())
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(store.writes) != 1 {
		t.Fatalf("writes = %#v", store.writes)
	}
	write := store.writes[0]
	if write.key != cache.KeyLivery("df597aaf-456c-4878-9d84-45201f2aae74") || write.ttl != gameliveries.CacheTTL {
		t.Fatalf("write = %#v", write)
	}
	livery, ok := write.value.(gameliveries.Livery)
	if !ok || livery.AircraftName != "A350" || livery.LiveryName != "Swiss" {
		t.Fatalf("cached livery = %#v", write.value)
	}
}

func TestJobRunWrapsClientError(t *testing.T) {
	job := New(liveriesClientStub{err: context.DeadlineExceeded}, &cacheStub{})
	err := job.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refresh liveries") {
		t.Fatalf("Run() error = %v", err)
	}
}
