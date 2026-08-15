package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"infinite-experiment/politburo/internal/config"
	"infinite-experiment/politburo/internal/metrics"

	"github.com/redis/go-redis/v9"
)

var ErrMiss = errors.New("cache miss")

// Store is the JSON cache boundary used by jobs and cache-backed endpoints.
type Store interface {
	GetJSON(context.Context, string, any) error
	SetJSON(context.Context, string, any, time.Duration) error
}

type RedisStore struct {
	client  redisClient
	metrics *metrics.Registry
}

type redisClient interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Ping(context.Context) *redis.StatusCmd
	Close() error
}

func OpenRedis(ctx context.Context, cfg config.Redis, metricsRegistry *metrics.Registry) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	store := &RedisStore{client: client, metrics: metricsRegistry}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()
	if err := store.Ping(pingCtx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to Redis at %s: %w", cfg.Address(), err)
	}
	return store, nil
}

func (s *RedisStore) GetJSON(ctx context.Context, key string, destination any) error {
	startedAt := time.Now()
	outcome := "error"
	defer func() { s.observe("get", outcome, startedAt) }()

	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		outcome = "miss"
		return ErrMiss
	}
	if err != nil {
		return fmt.Errorf("get cache key %q: %w", key, err)
	}
	s.metrics.CachePayloadBytes.WithLabelValues("get").Observe(float64(len(data)))
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode cache key %q: %w", key, err)
	}
	outcome = "hit"
	return nil
}

func (s *RedisStore) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	startedAt := time.Now()
	outcome := "error"
	defer func() { s.observe("set", outcome, startedAt) }()

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache key %q: %w", key, err)
	}
	s.metrics.CachePayloadBytes.WithLabelValues("set").Observe(float64(len(data)))
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("set cache key %q: %w", key, err)
	}
	outcome = "success"
	s.metrics.CacheInserts.Inc()
	return nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	startedAt := time.Now()
	outcome := "success"
	err := s.client.Ping(ctx).Err()
	if err != nil {
		outcome = "error"
	}
	s.observe("ping", outcome, startedAt)
	return err
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) observe(operation, outcome string, startedAt time.Time) {
	s.metrics.CacheOperations.WithLabelValues(operation, outcome).Inc()
	s.metrics.CacheOperationDuration.WithLabelValues(operation).Observe(time.Since(startedAt).Seconds())
}
