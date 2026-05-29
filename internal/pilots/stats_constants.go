package pilots

import "time"

const (
	statsProfileCachePrefix = "pilot:stats:profile:"
	statsRefreshCachePrefix = "pilot:stats:refresh:"
)

const (
	statsProfileTTL      = 20 * time.Minute
	statsRefreshCooldown = 1 * time.Minute
)
