package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MetricsMiddleware records HTTP metrics for each request
func MetricsMiddleware(metricsReg *metrics.MetricsRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the route pattern from chi context
			routePattern := "unknown"
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					routePattern = pattern
				}
			}

			// Record request in flight. Chi route patterns are only final after the
			// handler runs, so in-flight may use "unknown" for unmatched/early requests.
			inFlightRoutePattern := routePattern
			metricsReg.HTTPRequestsInFlight.WithLabelValues(inFlightRoutePattern).Inc()
			defer metricsReg.HTTPRequestsInFlight.WithLabelValues(inFlightRoutePattern).Dec()

			// Measure request duration
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &statusRecorder{ResponseWriter: w, statusCode: 200}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			observedStatusCode := wrapped.statusCode
			if wrapped.writeErr != nil && observedStatusCode == http.StatusOK {
				// The response had started, then the client disconnected/timed out.
				// Record this as client-closed instead of a misleading successful 200.
				observedStatusCode = 499
			}
			statusCode := strconv.Itoa(observedStatusCode)
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					routePattern = pattern
				}
			}

			metricsReg.HTTPRequestsTotal.WithLabelValues(
				routePattern,
				r.Method,
				statusCode,
			).Inc()

			metricsReg.HTTPRequestDuration.WithLabelValues(
				routePattern,
				r.Method,
			).Observe(duration)

			// Extract request ID from context or generate one
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = "req-" + time.Now().Format("20060102150405")
			}

			// Record only low-cardinality context presence flags. Do not log raw Discord IDs.
			discordUserContextPresent := strings.TrimSpace(r.Header.Get("X-Discord-User-Id")) != "" || strings.TrimSpace(r.Header.Get("X-Discord-Id")) != ""
			discordServerContextPresent := strings.TrimSpace(r.Header.Get("X-Discord-Server-Id")) != "" || strings.TrimSpace(r.Header.Get("X-Server-Id")) != ""

			// Log request
			logging.Info("HTTP request completed",
				"request_id", requestID,
				"method", r.Method,
				"endpoint", routePattern,
				"status_code", observedStatusCode,
				"duration_ms", int(duration*1000),
				"write_error", wrapped.writeErr != nil,
				"discord_user_context_present", discordUserContextPresent,
				"discord_server_context_present", discordServerContextPresent,
			)
		})
	}
}

// RequestIDMiddleware adds a request ID to the context if not present
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Generate a request ID if not provided
			requestID = "req-" + uuid.New().String()
		}
		// Store request ID in context
		ctx := context.WithValue(r.Context(), "request_id", requestID)

		// Add to response header for tracing
		w.Header().Add("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
	writeErr   error
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.statusCode = 200
		r.written = true
	}
	n, err := r.ResponseWriter.Write(b)
	if err != nil {
		r.writeErr = err
	}
	return n, err
}

// isIDLike checks if a string looks like an ID (numeric or UUID)
func isIDLike(s string) bool {
	if s == "" {
		return false
	}

	// Check if all numeric
	for _, c := range s {
		if c < '0' || c > '9' {
			// Check if it's UUID-like (contains hyphens)
			if strings.Contains(s, "-") && len(s) == 36 {
				return true
			}
			return false
		}
	}
	return true
}
