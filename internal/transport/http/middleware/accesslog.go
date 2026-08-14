package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	appmetrics "infinite-experiment/politburo/internal/metrics"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func AccessLog(metrics *appmetrics.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(writer, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			status := writer.Status()
			metrics.Requests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
			metrics.RequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(started).Seconds())
			slog.Info("http request",
				"request_id", chimiddleware.GetReqID(r.Context()),
				"method", r.Method,
				"route", route,
				"status", status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}
