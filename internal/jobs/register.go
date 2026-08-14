package jobs

import "infinite-experiment/politburo/internal/scheduler"

// Register is the single composition point for scheduled work. The clean-slate
// application intentionally starts with no game jobs.
func Register(_ *scheduler.Scheduler) error {
	return nil
}
