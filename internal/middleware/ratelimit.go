package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"

	goredis "github.com/redis/go-redis/v9"
)

// Rate limit groups and their per-minute request limits.
const (
	RateLimitGroupRegistration = "registration"
	RateLimitGroupSubmit       = "submit"
	RateLimitGroupRead         = "read"

	rateLimitRegistration = 5
	rateLimitSubmit       = 10
	rateLimitRead         = 60
)

// groupLimit maps group name to requests-per-minute.
var groupLimit = map[string]int{
	RateLimitGroupRegistration: rateLimitRegistration,
	RateLimitGroupSubmit:       rateLimitSubmit,
	RateLimitGroupRead:         rateLimitRead,
}

// RateLimitMiddleware returns a middleware that enforces per-user,
// per-minute request limits using Redis INCR + EXPIRE.
//
// Key format: ratelimit:{group}:{user_id}:{unix_minute}
//
// When the limit is exceeded the middleware writes 429 with a Retry-After
// header indicating seconds until the next minute bucket opens.
func RateLimitMiddleware(client *goredis.Client, group string, limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Identify the caller. Prefer Discord user ID from claims; fall back
			// to the IP address so unauthenticated requests are still throttled.
			userID := callerID(r)

			// Current minute bucket.
			minuteBucket := start.Unix() / 60
			key := fmt.Sprintf("ratelimit:%s:%s:%d", group, userID, minuteBucket)

			ctx := r.Context()
			count, err := client.Incr(ctx, key).Result()
			if err != nil {
				// Redis unavailable — allow the request to avoid blocking the service.
				logging.Warn("rate limit Redis error, allowing request",
					"group", group,
					"user_id", userID,
					"error", err,
				)
				next.ServeHTTP(w, r)
				return
			}

			// On first increment, set TTL to 2 minutes so the key is cleaned
			// up even if the bucket rolls over before expiry fires.
			if count == 1 {
				client.Expire(ctx, key, 2*time.Minute)
			}

			if int(count) > limit {
				// Seconds until the next minute bucket starts.
				nextBucket := (minuteBucket + 1) * 60
				retryAfter := nextBucket - start.Unix()

				logging.Warn("rate limit exceeded",
					"group", group,
					"user_id", userID,
					"count", count,
					"limit", limit,
				)

				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				httpdto.WriteError(w, start, "RATE_LIMIT_EXCEEDED",
					fmt.Sprintf("Too many requests. Retry after %d seconds.", retryAfter),
					http.StatusTooManyRequests,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// callerID extracts a stable identifier for the request caller. Uses the
// Discord user ID from claims when available, or falls back to remote IP.
func callerID(r *http.Request) string {
	if claims := auth.GetUserClaims(r.Context()); claims != nil {
		if uid := claims.DiscordUserID(); uid != "" {
			return uid
		}
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
