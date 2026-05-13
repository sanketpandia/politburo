package runtime

import (
	"context"
	"fmt"
	"net/http"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/routes"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server composes the HTTP server, background jobs, and graceful shutdown for a binary.
type Server struct {
	application *app.App
	withMetrics bool
	withJobs    bool
	port        string
}

// NewAPIServer returns a Server configured for the main API binary (metrics + jobs enabled).
func NewAPIServer(a *app.App) *Server {
	return &Server{
		application: a,
		withMetrics: true,
		withJobs:    true,
		port:        a.Config.Port,
	}
}

// NewVizburoServer returns a Server configured for the Vizburo UI binary (metrics + jobs disabled).
func NewVizburoServer(a *app.App) *Server {
	return &Server{
		application: a,
		withMetrics: false,
		withJobs:    false,
		port:        a.Config.VizburoPort,
	}
}

// Run starts background jobs (if enabled), listens for HTTP traffic, and blocks until
// a shutdown signal is received or the server fails. It shuts down the HTTP server and
// application before returning.
func (s *Server) Run(ctx context.Context) error {
	if s.withJobs {
		if err := s.startBackgroundJobs(); err != nil {
			return err
		}
	}

	cfg := s.application.Config
	srv := &http.Server{
		Addr:         ":" + s.port,
		Handler:      s.buildHandler(),
		ReadTimeout:  cfg.HTTPServer.ReadTimeout,
		WriteTimeout: cfg.HTTPServer.WriteTimeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	shutdownCtx, stop := NotifyShutdown(ctx)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logging.Info("HTTP server starting", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logging.Error("Server error", "error", err)
		return err
	case <-shutdownCtx.Done():
		logging.Info("Shutdown signal received")
	}

	logging.Info("Initiating graceful shutdown...")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		logging.Error("HTTP server shutdown error", "error", err)
	} else {
		logging.Info("HTTP server stopped gracefully")
	}

	s.application.Shutdown(timeoutCtx)
	logging.Info("Shutdown complete")
	return nil
}

func (s *Server) buildHandler() http.Handler {
	router := routes.NewRouter(s.application)
	if !s.withMetrics {
		return router
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", router)
	logging.Info("Prometheus metrics endpoint registered at /metrics")
	return mux
}

func (s *Server) startBackgroundJobs() error {
	if err := routes.RegisterScheduledJobs(s.application); err != nil {
		return fmt.Errorf("failed to register scheduled jobs: %w", err)
	}
	s.application.Infra.Scheduler.Start()
	logging.Info("Scheduler started with registered jobs")

	if err := routes.RegisterWorkers(s.application); err != nil {
		return fmt.Errorf("failed to register workers: %w", err)
	}
	logging.Info("Background workers started")

	if s.application.Infra.WatermillRouter != nil {
		go func() {
			if err := s.application.Infra.WatermillRouter.Run(context.Background()); err != nil {
				logging.Error("Watermill router stopped with error", "error", err)
			}
		}()
		logging.Info("Watermill router started")
	}

	return nil
}
