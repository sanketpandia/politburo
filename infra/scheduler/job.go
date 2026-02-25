package scheduler

import "context"

// Job defines the interface all scheduled jobs must implement
type Job interface {
	// Run executes the job once with the given context
	Run(ctx context.Context) error

	// Name returns a unique identifier for this job
	Name() string
}

// JobWithSchedule wraps a Job with its cron schedule (for future use)
type JobWithSchedule struct {
	Job      Job
	Schedule string // Cron expression (e.g., "*/5 * * * *" or "0 */5 * * * *" with seconds)
}
