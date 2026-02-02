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

			// Record request in flight
			metricsReg.HTTPRequestsInFlight.WithLabelValues(routePattern).Inc()
			defer metricsReg.HTTPRequestsInFlight.WithLabelValues(routePattern).Dec()

			// Measure request duration
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &statusRecorder{ResponseWriter: w, statusCode: 200}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(wrapped.statusCode)

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

			// Extract user context if available
			userID := r.Header.Get("X-Discord-Id")
			serverID := r.Header.Get("X-Server-Id")

			// Log request
			logging.Info("HTTP request completed",
				"request_id", requestID,
				"method", r.Method,
				"endpoint", routePattern,
				"status_code", wrapped.statusCode,
				"duration_ms", int(duration*1000),
				"server_id", serverID,
				"user_id", userID,
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
	return r.ResponseWriter.Write(b)
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
