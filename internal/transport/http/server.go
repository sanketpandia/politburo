package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"

	healthapi "infinite-experiment/politburo/internal/api/generated/health"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/transport/http/health"
	appmiddleware "infinite-experiment/politburo/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	app *app.App
}

func NewServer(application *app.App) *Server {
	return &Server{app: application}
}

func (s *Server) Run(parent context.Context) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	router := s.router()
	server := &stdhttp.Server{
		Addr:         ":" + s.app.Config.HTTP.Port,
		Handler:      router,
		ReadTimeout:  s.app.Config.HTTP.ReadTimeout,
		WriteTimeout: s.app.Config.HTTP.WriteTimeout,
		IdleTimeout:  s.app.Config.HTTP.IdleTimeout,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	if s.app.Config.Jobs.Enabled {
		s.app.Scheduler.Start()
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server started", "address", server.Addr)
		serverErr <- server.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, stdhttp.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.app.Config.HTTP.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func (s *Server) router() stdhttp.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(appmiddleware.AccessLog(s.app.Metrics))

	healthHandler := health.NewHandler(s.app.DB, s.app.StartedAt)
	healthapi.HandlerFromMux(healthHandler, router)
	router.Handle("/metrics", promhttp.HandlerFor(s.app.Metrics.Prometheus, promhttp.HandlerOpts{}))
	return router
}
