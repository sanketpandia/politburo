package cache

import "time"

// CacheInterface defines the contract for cache implementations
type                  CacheInterface interface 
{
	// Set stores a value in cache with the given key and duration
	Set(key string, value interface{}, duration time.Duration)

	// Get retrieves a value from cache by key
	// Returns the value and true if found, nil and false otherwise
	Get(key string) (interface{}, bool)

	// Delete removes a value from cache by key
	Delete(key string)

	// GetOrSet retrieves a value from cache, or loads it using the loader function if not found
	GetOrSet(key string, duration time.Duration, loader func() (any, error)) (interface{}, error)

	// Close closes any underlying connections (for Redis, etc.)
	Close() error
}
