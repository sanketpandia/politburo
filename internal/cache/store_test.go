package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

type redisClientStub struct {
	getValue  string
	getError  error
	setError  error
	pingError error
}

func (s *redisClientStub) Get(context.Context, string) *redis.StringCmd {
	return redis.NewStringResult(s.getValue, s.getError)
}

func (s *redisClientStub) Set(context.Context, string, any, time.Duration) *redis.StatusCmd {
	return redis.NewStatusResult("OK", s.setError)
}

func (s *redisClientStub) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", s.pingError)
}

func (*redisClientStub) Close() error { return nil }

func TestRedisStoreRecordsHitMissAndFailure(t *testing.T) {
	tests := []struct {
		name    string
		client  *redisClientStub
		outcome string
		wantErr bool
	}{
		{name: "hit", client: &redisClientStub{getValue: `{"value":1}`}, outcome: "hit"},
		{name: "miss", client: &redisClientStub{getError: redis.Nil}, outcome: "miss", wantErr: true},
		{name: "redis failure", client: &redisClientStub{getError: errors.New("unavailable")}, outcome: "error", wantErr: true},
		{name: "decode failure", client: &redisClientStub{getValue: `{`}, outcome: "error", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := metrics.NewRegistry()
			store := &RedisStore{client: tt.client, metrics: registry}
			var destination map[string]int
			err := store.GetJSON(context.Background(), "sessions", &destination)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := testutil.ToFloat64(registry.CacheOperations.WithLabelValues("get", tt.outcome)); got != 1 {
				t.Fatalf("get %s operations = %v, want 1", tt.outcome, got)
			}
		})
	}
}

func TestRedisStoreRecordsSuccessfulInsert(t *testing.T) {
	registry := metrics.NewRegistry()
	store := &RedisStore{client: &redisClientStub{}, metrics: registry}

	if err := store.SetJSON(context.Background(), "sessions", map[string]int{"value": 1}, time.Minute); err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}
	if got := testutil.ToFloat64(registry.CacheOperations.WithLabelValues("set", "success")); got != 1 {
		t.Fatalf("successful sets = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registry.CacheInserts); got != 1 {
		t.Fatalf("cache inserts = %v, want 1", got)
	}
}

func TestRedisStoreRecordsPingFailure(t *testing.T) {
	registry := metrics.NewRegistry()
	store := &RedisStore{client: &redisClientStub{pingError: errors.New("unavailable")}, metrics: registry}

	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("Ping() error = nil, want failure")
	}
	if got := testutil.ToFloat64(registry.CacheOperations.WithLabelValues("ping", "error")); got != 1 {
		t.Fatalf("failed pings = %v, want 1", got)
	}
}
