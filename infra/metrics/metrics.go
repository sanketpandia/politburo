package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsRegistry holds all Prometheus metrics for Politburo
type MetricsRegistry struct {
	// HTTP Metrics
	HTTPRequestsTotal     prometheus.CounterVec
	HTTPRequestDuration   prometheus.HistogramVec
	HTTPRequestsInFlight  prometheus.GaugeVec

	// Database Metrics
	DBQueriesTotal   prometheus.CounterVec
	DBQueryDuration  prometheus.HistogramVec
	DBConnections    prometheus.GaugeVec

	// Cache Metrics
	CacheHitsTotal   prometheus.CounterVec
	CacheMissesTotal prometheus.CounterVec
	CacheSize        prometheus.GaugeVec

	// Business Metrics
	FlightsProcessedTotal prometheus.Counter
	UsersActive           prometheus.Gauge
	SyncJobDuration       prometheus.HistogramVec
}

// NewMetricsRegistry initializes and returns a new MetricsRegistry with all metrics
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		// HTTP Metrics
		HTTPRequestsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_http_requests_total",
				Help: "Total HTTP requests processed by endpoint, method, and status code",
			},
			[]string{"endpoint", "method", "status_code"},
		),
		HTTPRequestDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "politburo_http_request_duration_seconds",
				Help:    "HTTP request latency distribution in seconds",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"endpoint", "method"},
		),
		HTTPRequestsInFlight: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
			[]string{"endpoint"},
		),

		// Database Metrics
		DBQueriesTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_db_queries_total",
				Help: "Total database queries by operation type",
			},
			[]string{"query_type"},
		),
		DBQueryDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "politburo_db_query_duration_seconds",
				Help:    "Database query execution time in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"query_type"},
		),
		DBConnections: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_db_connections",
				Help: "Current number of database connections",
			},
			[]string{"state"},
		),

		// Cache Metrics
		CacheHitsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_cache_hits_total",
				Help: "Total cache hits by cache key pattern",
			},
			[]string{"cache_key_pattern"},
		),
		CacheMissesTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_cache_misses_total",
				Help: "Total cache misses by cache key pattern",
			},
			[]string{"cache_key_pattern"},
		),
		CacheSize: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_cache_size_bytes",
				Help: "Current cache size in bytes",
			},
			[]string{"cache_name"},
		),

		// Business Metrics
		FlightsProcessedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "politburo_flights_processed_total",
				Help: "Total flight records processed",
			},
		),
		UsersActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "politburo_users_active",
				Help: "Current number of active users",
			},
		),
		SyncJobDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "politburo_sync_job_duration_seconds",
				Help:    "Sync job execution time in seconds",
				Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120, 300, 600},
			},
			[]string{"job_name"},
		),
	}
}
