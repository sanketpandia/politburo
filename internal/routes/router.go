package routes

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/middleware"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// resolveProjectPath resolves a relative path to an absolute path relative to project root
// This is needed because the binary may run from .air_tmp, so relative paths don't work
func resolveProjectPath(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}

	// Find project root by looking for go.mod file
	wd, err := os.Getwd()
	if err != nil {
		// Fallback to relative path if we can't determine working directory
		return relPath
	}

	// Start from current directory and walk up to find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, relPath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	// Fallback: use current working directory
	return filepath.Join(wd, relPath)
}

// NewRouter creates and configures the HTTP router with all routes
// This is a pure routing function - all dependencies are injected via the App struct
func NewRouter(application *app.App) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestIDMiddleware)

	logging.Info("Router initialized with request ID middleware")

	// Static file serving with CDN support
	// Serve static files from static/ at /static/
	// Resolve path relative to project root (handles .air_tmp working directory)
	staticDir := resolveProjectPath("static")
	fileServer := http.FileServer(http.Dir(staticDir))
	r.Group(func(static chi.Router) {
		static.Use(middleware.CDNMiddleware)
		static.Handle("/static/*", http.StripPrefix("/static/", fileServer))
	})
	logging.Info("Static file server configured", "path", staticDir, "route", "/static/*")

	// Health check endpoint
	r.Get("/healthCheck", api.HealthCheckHandler(
		application.Infra.DB,
		application.Infra.RedisCache,
		application.UpSince,
	))

	// Initialize auth service and handler (used by both API and UI routes)
	// Use adapter to avoid circular dependency (va package imports auth)
	vaAdapter := &vaServiceAdapter{svc: application.Platform.VASvc}
	authSvc := auth.NewService(
		application.Infra.SessionSvc,
		application.Infra.URLSigner,
		application.Platform.ClaimsRepo,
		application.Platform.UsersSvc,
		vaAdapter,
	)
	authHandler := auth.NewHandler(authSvc)

	// Auth routes (public) - token login handler
	r.Get("/auth/login", authHandler.TokenLogin())

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

		// Live flights endpoint - returns cached live flights for the current VA
		// Reads from prepopulated cache (game:live:vaflights:<va_id> and game:live:flight:<flight_id>)
		// Includes signed link for browser access to /live page
		v1.Get("/flights/va", flights.GetVALiveFlightsFromCache(application.Infra.RedisCache, authSvc))

		// Get single flight by ID - returns CompleteFlight from cache
		v1.Get("/flights/{flight_id}", flights.GetFlightByID(application.Infra.RedisCache))

		// Signed link generation endpoint
		v1.Post("/signed-link", authHandler.GenerateSignedLink())

		// Logbook endpoint
		v1.Get("/pilots/{ifc_id}/logbook", application.Features.PilotsHandler.GetUserLogbook())

		// Events endpoints
		v1.Route("/events", func(events chi.Router) {
			events.Get("/", application.Features.EventsHandler.ListEvents())
			events.Post("/", application.Features.EventsHandler.CreateEvent())
			events.Get("/{id}", application.Features.EventsHandler.GetEvent())
			events.Put("/{id}", application.Features.EventsHandler.UpdateEvent())
			events.Delete("/{id}", application.Features.EventsHandler.DeleteEvent())
			events.Patch("/{id}/status", application.Features.EventsHandler.UpdateEventStatus())
			events.Get("/{id}/summary", application.Features.EventsHandler.GetEventSummary())
			events.Get("/{id}/legs", application.Features.EventsHandler.GetEventLegs())
			events.Post("/{id}/legs", application.Features.EventsHandler.CreateEventLeg())
			events.Get("/{id}/legs/{leg_id}", application.Features.EventsHandler.GetEventLeg())
			events.Put("/{id}/legs/{leg_id}", application.Features.EventsHandler.UpdateEventLeg())
			events.Patch("/{id}/legs/{leg_id}/additional-data", application.Features.EventsHandler.UpdateEventLegAdditionalData())
			events.Delete("/{id}/legs/{leg_id}", application.Features.EventsHandler.DeleteEventLeg())
		})

		// Admin-only API routes
		v1.Route("/admin", func(adminAPI chi.Router) {
			adminAPI.Use(middleware.IsAdminMiddleware())

			// Airtable Data Provider API
			adminAPI.Route("/airtable", func(airtable chi.Router) {
				airtable.Post("/credentials", application.Platform.VAHandler.SaveAirtableCredentialsHandler())
				airtable.Get("/credentials", application.Platform.VAHandler.GetAirtableCredentialsHandler())
				airtable.Post("/schema/{schemaType}", application.Platform.VAHandler.SaveAirtableSchemaHandler())
				airtable.Get("/schema/{schemaType}", application.Platform.VAHandler.GetAirtableSchemaHandler())
				airtable.Get("/schemas", application.Platform.VAHandler.GetAirtableSchemasHandler())
			})
		})

		logging.Info("Registered routes: GET /api/v1/user/status, POST /api/v1/pilots/register, POST /api/v1/server/init, POST /api/v1/memberships/join, GET /api/v1/flights/va, GET /api/v1/flights/{flight_id}, POST /api/v1/signed-link, GET /api/v1/pilots/{ifc_id}/logbook, GET /api/v1/events, POST /api/v1/events, GET /api/v1/events/{id}, PUT /api/v1/events/{id}, DELETE /api/v1/events/{id}, PATCH /api/v1/events/{id}/status, GET /api/v1/events/{id}/summary, GET /api/v1/events/{id}/legs, POST /api/v1/events/{id}/legs, GET /api/v1/events/{id}/legs/{leg_id}, PUT /api/v1/events/{id}/legs/{leg_id}, DELETE /api/v1/events/{id}/legs/{leg_id}, POST /api/v1/admin/airtable/credentials, GET /api/v1/admin/airtable/credentials, POST /api/v1/admin/airtable/schema/{schemaType}, GET /api/v1/admin/airtable/schema/{schemaType}, GET /api/v1/admin/airtable/schemas")
	})

	// Dashboard routes (require authentication)
	r.Route("/dashboard", func(dashboard chi.Router) {
		// Apply authentication middleware to all dashboard routes
		dashboard.Use(middleware.AuthMiddleware(
			application.Platform.ClaimsRepo,
			application.Platform.KeysRepo,
			application.Infra.SessionSvc,
		))

		// member-only routes (member + staff + admin)
		dashboard.Group(func(member chi.Router) {
			member.Use(middleware.IsMemberMiddleware())

			// Live Flights page (staff + admin)
			member.Get("/live", flights.LiveFlightsPageHandler(application.Infra.RedisCache))

			// Get flight waypoints for route mapping (staff + admin)
			member.Get("/flights/{flight_id}/waypoints", flights.GetFlightWaypoints(application.Infra.RedisCache))

			// Logbook page and endpoints (staff + admin)
			member.Get("/logbook", application.Features.PilotsHandler.LogbookPageHandler())
			member.Get("/logbook/flights", application.Features.PilotsHandler.LogbookFlightsHandler())
		})

		// Admin-only routes
		dashboard.Group(func(admin chi.Router) {
			admin.Use(middleware.IsAdminMiddleware())

			// VA Admin features
			admin.Route("/vaadmin", func(vaadmin chi.Router) {
				// Pilots Management
				vaadmin.Get("/pilots", application.Features.VAAdminHandler.PilotsPageHandler())
				vaadmin.Get("/pilots/list", application.Features.VAAdminHandler.PilotsListHandler())
				vaadmin.Post("/pilots/{pilot_id}/callsign", application.Features.VAAdminHandler.UpdatePilotCallsignHandler())
				vaadmin.Post("/pilots/{pilot_id}/role", application.Features.VAAdminHandler.UpdatePilotRoleHandler())
				vaadmin.Delete("/pilots/{pilot_id}", application.Features.VAAdminHandler.RemovePilotHandler())

				// Flight Modes Configuration
				vaadmin.Get("/flight-modes/list", application.Features.VAAdminHandler.FlightModesListHandler())
				vaadmin.Get("/flight-modes/{mode_id}/edit", application.Features.VAAdminHandler.GetFlightModeEditHandler())
				vaadmin.Post("/flight-modes/{mode_id}/toggle", application.Features.VAAdminHandler.ToggleFlightModeHandler())
				vaadmin.Post("/flight-modes/{mode_id}/update", application.Features.VAAdminHandler.UpdateFlightModeHandler())
			})

			// Events Management
			admin.Route("/events", func(events chi.Router) {
				events.Get("/", application.Features.EventsHandler.EventsPageHandler())
				events.Get("/list", application.Features.EventsHandler.EventsListHandler())
				events.Get("/form", application.Features.EventsHandler.EventFormHandler())
				events.Get("/form/{event_id}", application.Features.EventsHandler.EventFormHandler())
				events.Post("/create", application.Features.EventsHandler.CreateEventHandler())
				// Routes search must come before dynamic {event_id} routes
				events.Get("/routes/search", application.Features.EventsHandler.RouteSearchHandler())
				events.Post("/{event_id}/update", application.Features.EventsHandler.UpdateEventHandler())
				events.Delete("/{event_id}", application.Features.EventsHandler.DeleteEventHandler())
				events.Get("/{event_id}/legs/form", application.Features.EventsHandler.LegFormHandler())
				events.Get("/{event_id}/legs/form/{leg_id}", application.Features.EventsHandler.LegFormHandler())
				events.Post("/{event_id}/legs/create", application.Features.EventsHandler.CreateLegHandler())
				events.Post("/{event_id}/legs/{leg_id}/update", application.Features.EventsHandler.UpdateLegHandler())
				events.Delete("/{event_id}/legs/{leg_id}", application.Features.EventsHandler.DeleteLegHandler())
			})

			// Datasource Configuration
			admin.Route("/settings/datasource", func(datasource chi.Router) {
				datasource.Get("/", application.Features.DatasourceHandler.DatasourcePageHandler())
				datasource.Get("/status", application.Features.DatasourceHandler.GetDatasourceStatusHandler())
				datasource.Get("/type-selector", application.Features.DatasourceHandler.GetDatasourceTypeSelectorHandler())
				datasource.Get("/schema-selector", application.Features.DatasourceHandler.GetSchemaTypeSelectorHandler())
				datasource.Get("/credentials-form", application.Features.DatasourceHandler.GetCredentialsFormHandler())
				datasource.Post("/credentials", application.Features.DatasourceHandler.SaveCredentialsHandler())
				datasource.Post("/test-connection", application.Features.DatasourceHandler.TestConnectionHandler())
				datasource.Get("/schema/{schemaType}", application.Features.DatasourceHandler.GetSchemaConfigHandler())
				datasource.Post("/schema/{schemaType}", application.Features.DatasourceHandler.SaveSchemaHandler())
				datasource.Post("/schema/{schemaType}/sync", application.Features.DatasourceHandler.SyncTableSchemaHandler())
			})
		})
	})

	return r
}

// vaServiceAdapter adapts platform VA service to auth VAService interface
// This is in routes package to avoid circular dependency (va package imports auth)
type vaServiceAdapter struct {
	svc *platformVA.Service
}

// GetByDiscordServerID implements auth.VAService interface
func (a *vaServiceAdapter) GetByDiscordServerID(ctx context.Context, discordServerID string) (auth.VAInfo, error) {
	va, err := a.svc.GetByDiscordServerID(ctx, discordServerID)
	if err != nil {
		return auth.VAInfo{}, err
	}
	return auth.VAInfo{
		ID:   va.ID,
		Name: va.Name,
		Code: va.Code,
	}, nil
}
