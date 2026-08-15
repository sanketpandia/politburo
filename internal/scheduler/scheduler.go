package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"infinite-experiment/politburo/internal/metrics"

	"github.com/robfig/cron/v3"
)

type Job interface {
	Name() string
	Run(context.Context) error
}

type Scheduler struct {
	cron    *cron.Cron
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	jobs    map[string]Job
	metrics *metrics.Registry
	start   sync.Once
	stop    sync.Once
}

func New(metricsRegistry *metrics.Registry) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	logger := cronLogger{}
	return &Scheduler{
		cron: cron.New(
			cron.WithSeconds(),
			cron.WithChain(cron.Recover(logger), cron.SkipIfStillRunning(logger)),
		),
		ctx: ctx, cancel: cancel, jobs: make(map[string]Job), metrics: metricsRegistry,
	}
}

func (s *Scheduler) Register(job Job, schedule string) error {
	if job == nil {
		return fmt.Errorf("register job: nil job")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.Name()]; exists {
		return fmt.Errorf("register job %q: duplicate name", job.Name())
	}
	if _, err := s.cron.AddFunc(schedule, func() {
		s.run(job)
	}); err != nil {
		return fmt.Errorf("register job %q: %w", job.Name(), err)
	}
	s.jobs[job.Name()] = job
	return nil
}

func (s *Scheduler) run(job Job) {
	name := job.Name()
	startedAt := time.Now()
	outcome := "error"
	s.metrics.JobRunning.WithLabelValues(name).Set(1)
	defer func() {
		s.metrics.JobRunning.WithLabelValues(name).Set(0)
		s.metrics.JobRuns.WithLabelValues(name, outcome).Inc()
		s.metrics.JobDuration.WithLabelValues(name).Observe(time.Since(startedAt).Seconds())
	}()

	if err := job.Run(s.ctx); err != nil {
		slog.Error("scheduled job failed", "job", name, "error", err)
		return
	}
	outcome = "success"
	s.metrics.JobLastSuccess.WithLabelValues(name).SetToCurrentTime()
}

func (s *Scheduler) Start() {
	s.start.Do(func() {
		slog.Info("scheduler starting", "jobs", len(s.jobs))
		s.cron.Start()
	})
}

func (s *Scheduler) Stop() {
	s.stop.Do(func() {
		s.cancel()
		<-s.cron.Stop().Done()
		slog.Info("scheduler stopped")
	})
}

type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...interface{}) {
	slog.Info(msg, "cron", keysAndValues)
}

func (cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	slog.Error(msg, "error", err, "cron", keysAndValues)
}
