package scheduler

import "infinite-experiment/politburo/infra/logging"

// Registry provides a fluent API for building scheduler configuration
type Registry struct {
	scheduler *Scheduler
}

// NewRegistry creates a new job registry
func NewRegistry(scheduler *Scheduler) *Registry {
	return &Registry{scheduler: scheduler}
}

// Add registers a job with its schedule
// Returns itself for method chaining
func (r *Registry) Add(job Job, schedule string) *Registry {
	if err := r.scheduler.Register(job, schedule); err != nil {
		// Log error but continue (fail gracefully)
		logging.Error("Failed to register job", "job", job.Name(), "error", err)
	}
	return r
}

// Build returns the configured scheduler
func (r *Registry) Build() *Scheduler {
	return r.scheduler
}
