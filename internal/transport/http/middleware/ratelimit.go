package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/transport/http/response"
)

// Rate limit groups and default per-minute limits (ported from legacy; unwired).
const (
	RateLimitGroupRegistration = "registration"
	RateLimitGroupSubmit       = "submit"
	RateLimitGroupRead         = "read"

	RateLimitRegistration = 5
	RateLimitSubmit       = 10
	RateLimitRead         = 60
)

// RateLimiter increments a per-minute bucket and returns the new count.
type RateLimiter interface {
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RateLimit enforces per-caller, per-minute limits. Not mounted by default;
// apply to write-heavy route groups when needed.
func RateLimit(limiter RateLimiter, group string, limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			now := time.Now()
			minuteBucket := now.Unix() / 60
			key := fmt.Sprintf("ratelimit:%s:%s:%d", group, callerID(r), minuteBucket)
			count, err := limiter.Incr(r.Context(), key, 2*time.Minute)
			if err != nil {
				// Fail open when the limiter backend is unavailable.
				next.ServeHTTP(w, r)
				return
			}
			if int(count) > limit {
				nextBucket := (minuteBucket + 1) * 60
				retryAfter := nextBucket - now.Unix()
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				response.WriteError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					fmt.Sprintf("Too many requests. Retry after %d seconds.", retryAfter))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func callerID(r *http.Request) string {
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok && claims.DsUserID != "" {
		return claims.DsUserID
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok && claims.PbUserID != "" {
		return claims.PbUserID
	}
	host := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	return host
}
