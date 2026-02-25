package scheduler

import (
	"context"
	"fmt"
	"sync"

	"infinite-experiment/politburo/infra/logging"

	"github.com/robfig/cron/v3"
)

// Scheduler manages all cron jobs
type Scheduler struct {
	cron     *cron.Cron
	jobs     map[string]Job          // job name -> Job
	entryIDs map[string]cron.EntryID // job name -> cron entry ID
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewScheduler creates a new job scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// Create cron with second-precision and logger
	cronLogger := &cronLogger{}
	c := cron.New(
		cron.WithSeconds(),
		cron.WithLogger(cronLogger),
	)

	return &Scheduler{
		cron:     c,
		jobs:     make(map[string]Job),
		entryIDs: make(map[string]cron.EntryID),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Register adds a job with its cron schedule
func (s *Scheduler) Register(job Job, schedule string) error {
	name := job.Name()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("job %s already registered", name)
	}

	// Wrap job execution with context and error handling
	wrappedFunc := func() {
		s.wg.Add(1)
		defer s.wg.Done()

		logging.Debug("Starting scheduled job", "job", name)

		if err := job.Run(s.ctx); err != nil {
			logging.Error("Job execution failed", "job", name, "error", err)
		} else {
			logging.Debug("Job completed successfully", "job", name)
		}
	}

	// Add to cron
	entryID, err := s.cron.AddFunc(schedule, wrappedFunc)
	if err != nil {
		return fmt.Errorf("failed to schedule job %s: %w", name, err)
	}

	s.jobs[name] = job
	s.entryIDs[name] = entryID

	logging.Info("Registered scheduled job", "job", name, "schedule", schedule)

	return nil
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	logging.Info("Starting scheduler", "job_count", len(s.jobs))
	s.cron.Start()
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop() {
	logging.Info("Stopping scheduler")

	// Cancel context to signal jobs to stop
	s.cancel()

	// Stop accepting new job runs
	cronCtx := s.cron.Stop()

	// Wait for cron's internal shutdown
	<-cronCtx.Done()

	// Wait for all running jobs to complete
	s.wg.Wait()

	logging.Info("Scheduler stopped")
}

// RunNow executes a job immediately (useful for testing/manual triggers)
func (s *Scheduler) RunNow(jobName string) error {
	job, exists := s.jobs[jobName]
	if !exists {
		return fmt.Errorf("job %s not found", jobName)
	}

	logging.Info("Running job immediately", "job", jobName)
	return job.Run(s.ctx)
}

// GetJobs returns a list of all registered job names
func (s *Scheduler) GetJobs() []string {
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	return names
}

// cronLogger adapter for robfig/cron to use logging package
type cronLogger struct{}

func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	logging.Info(msg, keysAndValues...)
}

func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	logging.Error(msg, append(keysAndValues, "error", err)...)
}
