package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"infinite-experiment/politburo/internal/config"
	"infinite-experiment/politburo/internal/database"
	"infinite-experiment/politburo/internal/jobs"
	"infinite-experiment/politburo/internal/logging"
	"infinite-experiment/politburo/internal/metrics"
	"infinite-experiment/politburo/internal/scheduler"
	"infinite-experiment/politburo/internal/ui"
)

type App struct {
	Config    config.Config
	StartedAt time.Time
	DB        *sql.DB
	Metrics   *metrics.Registry
	Scheduler *scheduler.Scheduler
	UI        *ui.Renderer
	closeOnce sync.Once
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	logging.Init(cfg.Environment)
	metricsRegistry := metrics.NewRegistry()

	db, err := database.Open(ctx, cfg.Database.URL, cfg.Database.PingTimeout)
	if err != nil {
		return nil, err
	}

	renderer, err := ui.NewRenderer()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize UI assets: %w", err)
	}

	jobScheduler := scheduler.New()
	if err := jobs.Register(jobScheduler); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("register jobs: %w", err)
	}

	application := &App{
		Config: cfg, StartedAt: time.Now().UTC(), DB: db,
		Metrics: metricsRegistry, Scheduler: jobScheduler, UI: renderer,
	}
	slog.Info("application initialized", "environment", cfg.Environment, "jobs_enabled", cfg.Jobs.Enabled)
	return application, nil
}

func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.Scheduler.Stop()
		if err := a.DB.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	})
}
