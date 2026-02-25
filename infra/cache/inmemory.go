package cache

import (
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	"infinite-experiment/politburo/infra/metrics"
)

// CacheService is the legacy in-memory cache implementation
// Deprecated: Use RedisCacheService for production
type CacheService struct {
	cache       *cache.Cache
	metricsReg  *metrics.MetricsRegistry
}

// Ensure CacheService implements CacheInterface
var _ CacheInterface = (*CacheService)(nil)

func NewCacheService(defaultExpirationSeconds, cleanUpIntervalSeconds int) *CacheService {

	defaultExpiration := time.Duration(defaultExpirationSeconds) * time.Second
	cleanUpInterval := time.Duration(cleanUpIntervalSeconds) * time.Second
	c := cache.New(defaultExpiration, cleanUpInterval)
	return &CacheService{cache: c, metricsReg: nil}
}

// NewCacheServiceWithMetrics creates a new CacheService with metrics collection
func NewCacheServiceWithMetrics(defaultExpirationSeconds, cleanUpIntervalSeconds int, metricsReg *metrics.MetricsRegistry) *CacheService {
	defaultExpiration := time.Duration(defaultExpirationSeconds) * time.Second
	cleanUpInterval := time.Duration(cleanUpIntervalSeconds) * time.Second
	c := cache.New(defaultExpiration, cleanUpInterval)
	return &CacheService{cache: c, metricsReg: metricsReg}
}

// extractCacheKeyPattern extracts the pattern from a cache key (e.g., "flight" from "flight:123:details")
func extractCacheKeyPattern(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func (cs *CacheService) Set(key string, value interface{}, duration time.Duration) {
	cs.cache.Set(key, value, duration)
}

func (cs *CacheService) Get(key string) (interface{}, bool) {
	val, found := cs.cache.Get(key)

	// Record cache metrics if metrics registry is available
	if cs.metricsReg != nil {
		pattern := extractCacheKeyPattern(key)
		if found {
			cs.metricsReg.CacheHitsTotal.WithLabelValues(pattern).Inc()
		} else {
			cs.metricsReg.CacheMissesTotal.WithLabelValues(pattern).Inc()
		}
	}

	return val, found
}

func (cs *CacheService) Delete(key string) {
	cs.cache.Delete(key)
}

func (cs *CacheService) GetOrSet(
	key string,
	duration time.Duration,
	loader func() (any, error)) (interface{}, error) {
	// Get() now records metrics internally
	if val, found := cs.Get(key); found {
		return val, nil
	}

	val, err := loader()
	if err != nil {
		return nil, err
	}

	cs.Set(key, val, duration)
	return val, nil
}

// Close closes the cache (no-op for in-memory cache)
func (cs *CacheService) Close() error {
	return nil
}
