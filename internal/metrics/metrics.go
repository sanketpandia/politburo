package metrics

import "github.com/prometheus/client_golang/prometheus"

type Registry struct {
	Prometheus             *prometheus.Registry
	Requests               *prometheus.CounterVec
	RequestDuration        *prometheus.HistogramVec
	CacheOperations        *prometheus.CounterVec
	CacheOperationDuration *prometheus.HistogramVec
	CachePayloadBytes      *prometheus.HistogramVec
	CacheInserts           prometheus.Counter
	JobRuns                *prometheus.CounterVec
	JobDuration            *prometheus.HistogramVec
	JobRunning             *prometheus.GaugeVec
	JobLastSuccess         *prometheus.GaugeVec
}

func NewRegistry() *Registry {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "politburo",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests.",
	}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "politburo",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
	}, []string{"method", "route"})
	cacheOperations := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "politburo",
		Subsystem: "cache",
		Name:      "operations_total",
		Help:      "Total cache operations by operation and outcome.",
	}, []string{"operation", "outcome"})
	cacheDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "politburo",
		Subsystem: "cache",
		Name:      "operation_duration_seconds",
		Help:      "Cache operation duration in seconds.",
	}, []string{"operation"})
	cachePayloadBytes := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "politburo",
		Subsystem: "cache",
		Name:      "payload_bytes",
		Help:      "Encoded cache payload size in bytes by operation.",
		Buckets:   prometheus.ExponentialBuckets(128, 4, 9),
	}, []string{"operation"})
	cacheInserts := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "politburo",
		Subsystem: "cache",
		Name:      "inserts_total",
		Help:      "Total successful cache inserts.",
	})
	jobRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "politburo",
		Subsystem: "jobs",
		Name:      "runs_total",
		Help:      "Total scheduled job runs by job and outcome.",
	}, []string{"job", "outcome"})
	jobDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "politburo",
		Subsystem: "jobs",
		Name:      "run_duration_seconds",
		Help:      "Scheduled job run duration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12),
	}, []string{"job"})
	jobRunning := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "politburo",
		Subsystem: "jobs",
		Name:      "running",
		Help:      "Whether a scheduled job is currently running.",
	}, []string{"job"})
	jobLastSuccess := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "politburo",
		Subsystem: "jobs",
		Name:      "last_success_timestamp_seconds",
		Help:      "Unix timestamp of the last successful scheduled job run.",
	}, []string{"job"})
	registry.MustRegister(
		requests, duration,
		cacheOperations, cacheDuration, cachePayloadBytes, cacheInserts,
		jobRuns, jobDuration, jobRunning, jobLastSuccess,
	)
	return &Registry{
		Prometheus: registry, Requests: requests, RequestDuration: duration,
		CacheOperations: cacheOperations, CacheOperationDuration: cacheDuration,
		CachePayloadBytes: cachePayloadBytes, CacheInserts: cacheInserts,
		JobRuns: jobRuns, JobDuration: jobDuration, JobRunning: jobRunning,
		JobLastSuccess: jobLastSuccess,
	}
}
