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

	politburoapi "infinite-experiment/politburo/internal/api/generated/politburo"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/transport/http/api/gamesessions"
	"infinite-experiment/politburo/internal/transport/http/api/health"
	appmiddleware "infinite-experiment/politburo/internal/transport/http/middleware"
	"infinite-experiment/politburo/internal/transport/http/response"
	uihttp "infinite-experiment/politburo/internal/transport/http/ui"

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
	router.Use(appmiddleware.CORS(s.app.Config.HTTP.AllowedOrigins))
	router.Use(middleware.Recoverer)
	router.Use(appmiddleware.AccessLog(s.app.Metrics))
	router.Use(appmiddleware.APIKeyAuth(s.app.APIKeys))

	healthHandler := health.NewHandler(s.app.DB, s.app.Cache, s.app.StartedAt)
	sessionsHandler := gamesessions.NewHandler(s.app.Cache)
	uiHandler := uihttp.NewHandler(s.app.UI)
	handler := apiHandler{health: healthHandler, sessions: sessionsHandler}

	// Public ops + OpenAPI machine API (paths include /health/* and /api/v1/*).
	// APIKeyAuth requires a valid api_keys row for /api/v1 only.
	politburoapi.HandlerWithOptions(handler, politburoapi.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: writeParameterError,
	})
	router.Handle("/metrics", promhttp.HandlerFor(s.app.Metrics.Prometheus, promhttp.HandlerOpts{}))

	// Browser UI surface (session auth scaffold; lookup nil until login lands).
	router.Group(func(dashboard chi.Router) {
		dashboard.Use(appmiddleware.UISessionAuth(nil))
		dashboard.Get("/dashboard", uiHandler.Dashboard)
		dashboard.Get("/dashboard/", uiHandler.Dashboard)
	})
	router.Handle("/static/*", uihttp.Static())

	return router
}

type apiHandler struct {
	health   *health.Handler
	sessions *gamesessions.Handler
}

func (h apiHandler) GetLiveness(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.health.GetLiveness(w, r)
}

func (h apiHandler) GetReadiness(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.health.GetReadiness(w, r)
}

func (h apiHandler) GetActiveSessions(w stdhttp.ResponseWriter, r *stdhttp.Request, params politburoapi.GetActiveSessionsParams) {
	h.sessions.GetActiveSessions(w, r, params.History)
}

func writeParameterError(w stdhttp.ResponseWriter, _ *stdhttp.Request, err error) {
	response.WriteError(w, stdhttp.StatusBadRequest, "INVALID_QUERY_FILTER", err.Error())
}
