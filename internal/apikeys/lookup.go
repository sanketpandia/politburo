package apikeys

import (
	"context"
	"errors"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/cache"
)

const cacheTTL = time.Minute

type statusReader interface {
	Active(ctx context.Context, key string) (active bool, found bool, err error)
}

type statusCache struct {
	Active bool `json:"active"`
	Found  bool `json:"found"`
}

// Lookup validates X-API-Key values against api_keys, caching results for one minute.
type Lookup struct {
	repo  statusReader
	cache cache.Store
}

func NewLookup(repo statusReader, cacheStore cache.Store) *Lookup {
	return &Lookup{repo: repo, cache: cacheStore}
}

func (l *Lookup) Lookup(r *http.Request, apiKey string) (auth.Claims, bool, error) {
	active, err := l.active(r.Context(), apiKey)
	if err != nil {
		return auth.Claims{}, false, err
	}
	if !active {
		return auth.Claims{}, false, nil
	}
	return auth.Claims{APIKeyPresent: true}, true, nil
}

func (l *Lookup) active(ctx context.Context, apiKey string) (bool, error) {
	cacheKey := cache.KeyAPIKeyStatus(apiKey)
	var cached statusCache
	if err := l.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
		return cached.Found && cached.Active, nil
	} else if err != nil && !errors.Is(err, cache.ErrMiss) {
		// Fall through to the database on cache read failures.
	}

	active, found, err := l.repo.Active(ctx, apiKey)
	if err != nil {
		return false, err
	}
	_ = l.cache.SetJSON(ctx, cacheKey, statusCache{Active: active, Found: found}, cacheTTL)
	return found && active, nil
}
