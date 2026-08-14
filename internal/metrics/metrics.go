package metrics

import "github.com/prometheus/client_golang/prometheus"

type Registry struct {
	Prometheus      *prometheus.Registry
	Requests        *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
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
	registry.MustRegister(requests, duration)
	return &Registry{Prometheus: registry, Requests: requests, RequestDuration: duration}
}
