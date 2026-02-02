package routes

import (
	"net/http"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates and configures the HTTP router with all routes
// This is a pure routing function - all dependencies are injected via the App struct
func NewRouter(application *app.App) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestIDMiddleware)

	logging.Info("Router initialized with request ID middleware")

	// Health check endpoint
	r.Get("/healthCheck", api.HealthCheckHandler(
		application.Infra.DB,
		application.Infra.RedisCache,
		application.UpSince,
	))

	// API v1 routes with authentication
	r.Route("/api/v1", func(v1 chi.Router) {
		// Apply auth middleware to populate claims for all routes
		v1.Use(middleware.AuthMiddleware(
			application.Platform.ClaimsRepo,
			application.Platform.KeysRepo,
			application.Infra.SessionSvc,
		))

		logging.Info("Auth middleware applied to /api/v1 routes")

		// User status endpoint
		v1.Get("/user/status", application.Features.MembershipsHandler.GetUserStatus())

		// Pilot registration endpoint
		v1.Post("/pilots/register", application.Features.PilotsHandler.RegisterPilot())

		// Server initialization endpoint
		v1.Post("/server/init", application.Features.ServersHandler.InitServer())

		// Membership join endpoint
		v1.Post("/memberships/join", application.Features.MembershipsHandler.JoinVA())

		logging.Info("Registered routes: GET /api/v1/user/status, POST /api/v1/pilots/register, POST /api/v1/server/init, POST /api/v1/memberships/join")
	})

	return r
}
