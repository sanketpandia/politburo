package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsRegistry holds all Prometheus metrics for Politburo
type MetricsRegistry struct {
	// HTTP Metrics
	HTTPRequestsTotal    prometheus.CounterVec
	HTTPRequestDuration  prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.GaugeVec

	// Cache Metrics
	CacheHitsTotal   prometheus.CounterVec
	CacheMissesTotal prometheus.CounterVec
	CacheSize        prometheus.GaugeVec

	// Business Metrics
	SyncJobDuration prometheus.HistogramVec

	// Generic Queue Metrics (for all queue types)
	QueueDepth              prometheus.GaugeVec   // Labels: queue_name, queue_type
	QueuePending            prometheus.GaugeVec   // Labels: queue_name, queue_type
	QueueEnqueuedTotal      prometheus.CounterVec // Labels: queue_name, queue_type
	QueueDequeuedTotal      prometheus.CounterVec // Labels: queue_name, queue_type
	QueueProcessingDuration prometheus.HistogramVec // Labels: queue_name, queue_type
	QueueErrorsTotal        prometheus.CounterVec // Labels: queue_name, queue_type, error_type
	QueueRetriesTotal       prometheus.CounterVec // Labels: queue_name, queue_type
	QueueAcknowledgedTotal  prometheus.CounterVec // Labels: queue_name, queue_type

	// Dead Letter Queue Metrics
	DLQDepth                prometheus.GaugeVec   // Labels: queue_name, queue_type
	DLQItemsTotal           prometheus.CounterVec // Labels: queue_name, queue_type, error_type
	DLQRequeuedTotal        prometheus.CounterVec // Labels: queue_name, queue_type

	// Enhanced Sync Job Metrics
	SyncJobRecordsProcessed prometheus.CounterVec // Labels: job_name, provider, entity_type, va_id, status
	SyncJobRecordsFailed    prometheus.CounterVec // Labels: job_name, provider, entity_type, va_id, error_type
	SyncJobAutoLinked       prometheus.CounterVec // Labels: provider, entity_type, va_id
	SyncJobStatusUpdated    prometheus.CounterVec // Labels: provider, entity_type, va_id, status_value

	// Rate Limiting Metrics
	RateLimitThrottled      prometheus.CounterVec // Labels: provider, va_id
	RateLimitAllowed        prometheus.CounterVec // Labels: provider, va_id

	// Webhook Delivery Metrics
	WebhooksDeliveredTotal prometheus.CounterVec // Labels: webhook_target, status
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
		SyncJobDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "politburo_sync_job_duration_seconds",
				Help:    "Sync job execution time in seconds",
				Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120, 300, 600},
			},
			[]string{"job_name", "provider", "entity_type"},
		),

		// Generic Queue Metrics
		QueueDepth: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_queue_depth",
				Help: "Current number of items in queue",
			},
			[]string{"queue_name", "queue_type"},
		),
		QueuePending: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_queue_pending",
				Help: "Current number of pending items in queue",
			},
			[]string{"queue_name", "queue_type"},
		),
		QueueEnqueuedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_queue_enqueued_total",
				Help: "Total number of items enqueued",
			},
			[]string{"queue_name", "queue_type"},
		),
		QueueDequeuedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_queue_dequeued_total",
				Help: "Total number of items dequeued",
			},
			[]string{"queue_name", "queue_type"},
		),
		QueueProcessingDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "politburo_queue_processing_duration_seconds",
				Help:    "Time spent processing queue items in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"queue_name", "queue_type"},
		),
		QueueErrorsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_queue_errors_total",
				Help: "Total number of queue processing errors",
			},
			[]string{"queue_name", "queue_type", "error_type"},
		),
		QueueRetriesTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_queue_retries_total",
				Help: "Total number of queue item retries",
			},
			[]string{"queue_name", "queue_type"},
		),
		QueueAcknowledgedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_queue_acknowledged_total",
				Help: "Total number of queue items acknowledged",
			},
			[]string{"queue_name", "queue_type"},
		),

		// Dead Letter Queue Metrics
		DLQDepth: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "politburo_dlq_depth",
				Help: "Current number of items in dead letter queue",
			},
			[]string{"queue_name", "queue_type"},
		),
		DLQItemsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_dlq_items_total",
				Help: "Total number of items moved to dead letter queue",
			},
			[]string{"queue_name", "queue_type", "error_type"},
		),
		DLQRequeuedTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_dlq_requeued_total",
				Help: "Total number of items requeued from dead letter queue",
			},
			[]string{"queue_name", "queue_type"},
		),

		// Enhanced Sync Job Metrics
		SyncJobRecordsProcessed: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_sync_job_records_processed_total",
				Help: "Total number of records processed by sync job",
			},
			[]string{"job_name", "provider", "entity_type", "va_id", "status"},
		),
		SyncJobRecordsFailed: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_sync_job_records_failed_total",
				Help: "Total number of records failed in sync job",
			},
			[]string{"job_name", "provider", "entity_type", "va_id", "error_type"},
		),
		SyncJobAutoLinked: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_sync_job_auto_linked_total",
				Help: "Total number of records auto-linked in sync job",
			},
			[]string{"provider", "entity_type", "va_id"},
		),
		SyncJobStatusUpdated: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_sync_job_status_updated_total",
				Help: "Total number of status updates in sync job",
			},
			[]string{"provider", "entity_type", "va_id", "status_value"},
		),

		// Rate Limiting Metrics
		RateLimitThrottled: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_rate_limit_throttled_total",
				Help: "Total number of requests throttled by rate limiter",
			},
			[]string{"provider", "va_id"},
		),
		RateLimitAllowed: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_rate_limit_allowed_total",
				Help: "Total number of requests allowed by rate limiter",
			},
			[]string{"provider", "va_id"},
		),

		WebhooksDeliveredTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "politburo_webhooks_delivered_total",
				Help: "Total number of webhook delivery attempts",
			},
			[]string{"webhook_target", "status"},
		),
	}
}
