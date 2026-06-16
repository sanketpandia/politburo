package routes

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	pirepsgen "infinite-experiment/politburo/internal/api/generated/pireps"
	registrationgen "infinite-experiment/politburo/internal/api/generated/registration"
	pirepsapi "infinite-experiment/politburo/internal/api/pireps"
	registration "infinite-experiment/politburo/internal/api/registration"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/middleware"
	"infinite-experiment/politburo/internal/platform/health"
	"infinite-experiment/politburo/internal/platform/roles"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// TODO Route strings to use constants. Also do we need to keep paths if they are in swagger path?
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
	r.Use(middleware.MetricsMiddleware(application.Infra.MetricsReg))

	logging.Info("Router initialized with request ID and metrics middleware")

	// Static file serving with CDN support
	// Serve static files from static/ at /static/
	// Resolve path relative to project root (handles .air_tmp working directory)
	staticDir := resolveProjectPath("static")
	fileServer := http.FileServer(http.Dir(staticDir))
	r.Group(func(static chi.Router) {
		static.Use(middleware.CDNMiddleware)
		static.Use(middleware.MimeTypeMiddleware) // Set correct MIME types (especially for .mjs files)
		static.Handle("/static/*", http.StripPrefix("/static/", fileServer))
	})
	logging.Info("Static file server configured", "path", staticDir, "route", "/static/*")

	// Serve favicon.ico from project root
	faviconPath := resolveProjectPath("favicon.ico")
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, faviconPath)
	})
	// TODO this is a very hacky way and needs to be cleaned up
	// Serve logo assets referenced by the base layout.
	logoSVGPath := resolveProjectPath("logo.svg")
	r.Get("/logo.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, logoSVGPath)
	})
	logoPNGPath := resolveProjectPath("logo.png")
	r.Get("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, logoPNGPath)
	})

	// Health check endpoint
	r.Get("/healthCheck", health.HealthCheckHandler(
		application.Infra.DB,
		application.Infra.RedisCache,
		application.UpSince,
	))

	// Initialize auth service (used by flights and dashboard link handlers).
	// The auth handler is wired via application.Features.AuthHandler (constructed in app.go).
	vaAdapter := &vaServiceAdapter{svc: application.Platform.VASvc}
	authSvc := auth.NewService(
		application.Infra.SessionSvc,
		application.Infra.URLSigner,
		application.Platform.ClaimsRepo,
		application.Platform.UsersSvc,
		vaAdapter,
	)

	// Auth routes (public) - token login handler
	r.Get("/auth/login", application.Features.AuthHandler.TokenLogin())

	// API v1 routes with authentication
	r.Route("/api/v1", func(v1 chi.Router) {
		// Apply auth middleware to populate claims for all routes
		v1.Use(middleware.AuthMiddleware(
			application.Platform.ClaimsRepo,
			application.Platform.KeysRepo,
			application.Infra.SessionSvc,
		))

		logging.Info("Auth middleware applied to /api/v1 routes")

		v1.Group(func(bot chi.Router) {
			bot.Use(middleware.RequireDiscordBotContextMiddleware())
			registerRegistrationRoutes(bot, application, authSvc)
		})

		// Pilot stats endpoint
		v1.Get("/pilot/stats", application.Features.PilotsHandler.GetPilotStats())

		// Get single flight by ID - returns CompleteFlight from cache
		// TODO Remove direct dependency
		v1.Get("/flights/{flight_id}", flights.GetFlightByID(application.Infra.RedisCache))

		// Logbook endpoint (staff/admin only)
		// TODO Review needs to be done.
		v1.Route("/pilots/{ifc_id}/logbook", func(logbook chi.Router) {
			logbook.Use(middleware.IsStaffMiddleware())
			logbook.Get("/", application.Features.PilotsHandler.GetUserLogbook())
		})

		// Self-service logbook: own IFC ID or staff/admin may view any
		// TODO Can be eliminated. Above flights and logbook can be merged.
		v1.Route("/user/{ifc_id}/flights", func(logbookSelf chi.Router) {
			logbookSelf.Use(middleware.IsRegisteredMiddleware())
			logbookSelf.Get("/", application.Features.PilotsHandler.GetUserLogbookSelf())
		})

		registerPirepRoutes(v1, application)

		// Events endpoints (tours and tour legs) - require registration and membership
		v1.Route("/events", func(events chi.Router) {
			events.Use(middleware.IsRegisteredMiddleware())
			events.Use(middleware.IsMemberMiddleware())
			events.Get("/", application.Features.EventsHandler.ListEvents())
			events.Post("/", application.Features.EventsHandler.CreateEvent())
			// pirep-config must come before dynamic {id} routes
			events.Get("/pirep-config", application.Features.EventsHandler.GetEventPirepConfig())
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

		// God-mode verification — authenticated scope only (bot checks is_god field, not status code)
		v1.Get("/admin/verify-god", application.Features.AuthHandler.VerifyGodMode())

		// Admin-only API routes
		v1.Route("/admin", func(adminAPI chi.Router) {
			adminAPI.Use(middleware.IsAdminMiddleware())

			// Airtable Data Provider API
			adminAPI.Route("/airtable", func(airtable chi.Router) {
				airtable.Post("/credentials", application.Platform.VAHandler.SaveAirtableCredentialsHandler())

			})

			// Session management API
			adminAPI.Route("/sessions", func(sessions chi.Router) {
				// Destroy sessions endpoint requires god-mode with key header
				sessions.With(middleware.IsGodMiddlewareWithKey()).Post("/destroy/{ifc_id}", application.Features.AuthHandler.DestroySessionsByIFCId())
			})

			// Livery mappings API
			adminAPI.Route("/livery-mappings", func(liveryMappings chi.Router) {
				liveryMappings.Get("/", application.Features.LiveryMappingsHandler.ListMappingsHandler())
				liveryMappings.Post("/", application.Features.LiveryMappingsHandler.CreateMappingHandler())
				liveryMappings.Delete("/{id}", application.Features.LiveryMappingsHandler.DeleteMappingHandler())
				liveryMappings.Get("/liveries", application.Features.LiveryMappingsHandler.GetAvailableLiveriesHandler())
				liveryMappings.Get("/unique-aircraft", application.Features.LiveryMappingsHandler.GetUniqueAircraftHandler())
				liveryMappings.Get("/unique-liveries", application.Features.LiveryMappingsHandler.GetUniqueLiveriesHandler())
				liveryMappings.Get("/defaults", application.Features.LiveryMappingsHandler.GetDefaultsHandler())
				liveryMappings.Post("/defaults", application.Features.LiveryMappingsHandler.SetDefaultsHandler())
			})
		})

		logging.Info("Registered routes: GET /api/v1/user/status, POST /api/v1/user/register, POST /api/v1/server/init, POST /api/v1/memberships/join, GET /api/v1/flights/va, GET /api/v1/flights/{flight_id}, POST /api/v1/signed-link, GET /api/v1/pilots/{ifc_id}/logbook, GET /api/v1/user/{ifc_id}/flights, GET /api/v1/pireps/config, POST /api/v1/pireps/submit, GET /api/v1/admin/verify-god, GET /api/v1/events, POST /api/v1/events, GET /api/v1/events/{id}, PUT /api/v1/events/{id}, DELETE /api/v1/events/{id}, PATCH /api/v1/events/{id}/status, GET /api/v1/events/{id}/summary, GET /api/v1/events/{id}/legs, POST /api/v1/events/{id}/legs, GET /api/v1/events/{id}/legs/{leg_id}, PUT /api/v1/events/{id}/legs/{leg_id}, DELETE /api/v1/events/{id}/legs/{leg_id}, POST /api/v1/admin/airtable/credentials, POST /api/v1/admin/flight-modes/config, POST /api/v1/admin/airtable/schema/{schemaType}, GET /api/v1/admin/airtable/schema/{schemaType}, GET /api/v1/admin/airtable/schemas")
	})

	// Dashboard routes (require authentication)
	r.Route("/dashboard", func(dashboard chi.Router) {
		// Apply UI authentication middleware to all dashboard routes
		// Renders 401 error page instead of plain text for browser requests
		dashboard.Use(uiAuthMiddleware(
			application.Infra.SessionSvc,
			application.Infra.TemplateRenderer,
		))

		// Active VA switching is available to any authenticated dashboard session.
		dashboard.Post("/switch-va", application.Features.DashboardHandler.SwitchActiveVAHandler())

		// member-only routes (member + staff + admin)
		dashboard.Group(func(member chi.Router) {
			member.Use(middleware.IsMemberMiddleware())

			// Dashboard page (all members)
			member.Get("/", application.Features.DashboardHandler.DashboardPageHandler())
			member.Get("/pilot-stats", application.Features.DashboardHandler.GetPilotStatsPartialHandler())

			// Leaderboard pilot logs endpoint
			member.Get("/leaderboard/pilot/logs", application.Features.DashboardHandler.GetPilotPirepLogsHandler())

			// Test click handler endpoint
			member.Get("/test-click", application.Features.DashboardHandler.TestClickHandler())

			// Live Flights page (all members)
			member.Get("/live", flights.NewLivePageHandler(application.Infra.RedisCache, application.Infra.TemplateRenderer).LiveFlightsPageHandler())

			// Get flight waypoints for route mapping (all members)
			// TODO This is a JSON endpoint and should have OAS
			member.Get("/flights/{flight_id}/waypoints", flights.GetFlightWaypoints(application.Infra.RedisCache))
			member.Get("/flights/{flight_id}/paths", flights.GetCachedFlightPaths(application.Infra.RedisCache))

			// Dashboard signed link endpoint (all members)
			member.Get("/link", application.Features.DashboardHandler.GetDashboardLinkHandler(authSvc))
		})

		// Staff-only routes (staff + admin)
		dashboard.Group(func(staff chi.Router) {
			staff.Use(middleware.IsStaffMiddleware())

			// Logbook page and endpoints (staff/admin only)
			staff.Get("/logbook", application.Features.PilotsHandler.LogbookPageHandler())
			staff.Get("/logbook/flights", application.Features.PilotsHandler.LogbookFlightsHandler())
		})

		// Admin-only routes
		dashboard.Group(func(admin chi.Router) {
			admin.Use(middleware.IsAdminMiddleware())

			// VA Admin features
			admin.Route("/vaadmin", func(vaadmin chi.Router) {
				vaadmin.Get("/", application.Features.VAAdminHandler.IndexPageHandler())
				vaadmin.Get("/datasource/status", application.Features.VAAdminHandler.DatasourceStatusCardHandler())
				vaadmin.Get("/setup", application.Features.VAAdminHandler.SetupPageHandler())
				vaadmin.Get("/setup/basic", application.Features.VAAdminHandler.BasicSetupFormHandler())
				vaadmin.Post("/setup/basic", application.Features.VAAdminHandler.SaveBasicSetupHandler())
				vaadmin.Get("/setup/checklist", application.Features.VAAdminHandler.SetupChecklistHandler())
				vaadmin.Post("/setup/callsign-test", application.Features.VAAdminHandler.CallsignTestHandler())
				vaadmin.Get("/flight-modes", application.Features.VAAdminHandler.FlightModesPageHandler())
				// Pilots Management
				vaadmin.Get("/pilots", application.Features.VAAdminHandler.PilotsPageHandler())
				vaadmin.Get("/pilots/list", application.Features.VAAdminHandler.PilotsListHandler())
				vaadmin.Post("/pilots/{pilot_id}/callsign", application.Features.VAAdminHandler.UpdatePilotCallsignHandler())
				vaadmin.Post("/pilots/{pilot_id}/role", application.Features.VAAdminHandler.UpdatePilotRoleHandler())
				vaadmin.Delete("/pilots/{pilot_id}", application.Features.VAAdminHandler.RemovePilotHandler())

				// Flight Modes Configuration
				vaadmin.Get("/flight-modes/list", application.Features.VAAdminHandler.FlightModesListHandler())
				vaadmin.Post("/flight-modes/create", application.Features.VAAdminHandler.CreateFlightModeHandler())
				vaadmin.Get("/flight-modes/{mode_id}/edit", application.Features.VAAdminHandler.GetFlightModeEditHandler())
				vaadmin.Post("/flight-modes/{mode_id}/toggle", application.Features.VAAdminHandler.ToggleFlightModeHandler())
				vaadmin.Post("/flight-modes/{mode_id}/update", application.Features.VAAdminHandler.UpdateFlightModeHandler())
				// Setup webhooks (list + add Live Flights webhook)
				vaadmin.Get("/webhooks", application.Features.VAAdminHandler.WebhooksPageHandler())
				vaadmin.Get("/webhooks/list", application.Features.VAAdminHandler.WebhooksListHandler())
				vaadmin.Post("/webhooks", application.Features.VAAdminHandler.CreateWebhookFormHandler())
				vaadmin.Post("/webhooks/run", application.Features.VAAdminHandler.WebhooksRunNowHandler())
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
				datasource.Get("/schema/{schemaType}/mapping-chooser", application.Features.DatasourceHandler.GetFieldMappingChooserHandler())
				datasource.Post("/schema/{schemaType}/mapping-apply", application.Features.DatasourceHandler.ApplyFieldMappingHandler())
			})

			// Livery Mappings
			admin.Get("/settings/livery-mappings", application.Features.LiveryMappingsHandler.ListMappingsPageHandler())
		})
	})

	// Error handlers
	r.NotFound(handleNotFound(application.Infra.TemplateRenderer))
	r.MethodNotAllowed(handleMethodNotAllowed(application.Infra.TemplateRenderer))

	return r
}

// handleNotFound handles 404 Not Found errors
func handleNotFound(templateRenderer *templates.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		data := map[string]interface{}{
			"PageTitle": "Not Found",
		}
		if err := templateRenderer.RenderStandalone(w, "pages/404.html", data); err != nil {
			logging.Error("Failed to render 404 page", "error", err)
		}
	}
}

// handleMethodNotAllowed handles 405 Method Not Allowed errors
func handleMethodNotAllowed(templateRenderer *templates.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		data := map[string]interface{}{
			"PageTitle": "Method Not Allowed",
		}
		if err := templateRenderer.RenderStandalone(w, "pages/405.html", data); err != nil {
			logging.Error("Failed to render 405 page", "error", err)
		}
	}
}

// TODO Cleanup required. All routes should be registered like this in separate files
func registerRegistrationRoutes(bot chi.Router, application *app.App, authSvc *auth.Service) {
	// TODO Live flights is separate from register and should be dealt with separately
	liveFlightsHandler := flights.NewVALiveFlightsContractHandler(application.Infra.RedisCache, authSvc).GetVALiveFlightsFromCache()
	registrationServer := registration.NewServer(
		application.Features.PilotsHandler,
		application.Features.MembershipsHandler,
		application.Features.ServersHandler,
		application.Features.AuthHandler,
		liveFlightsHandler,
	)
	strictServer := registrationgen.NewStrictHandler(registrationServer, nil)
	registrationgen.HandlerFromMux(strictServer, bot)
}

// TODO Cleanup required. All routes should be registered like this in separate files
func registerPirepRoutes(v1 chi.Router, application *app.App) {
	pirepsServer := pirepsapi.NewServer(
		application.Features.PirepHandler,
		application.Platform.VAHandler,
	)
	strictServer := pirepsgen.NewStrictHandler(pirepsServer, nil)

	v1.Group(func(memberPireps chi.Router) {
		memberPireps.Use(middleware.IsRegisteredMiddleware())
		memberPireps.Use(middleware.IsMemberMiddleware())
		memberPireps.Get("/pireps/config", strictServer.GetPirepConfig)
		memberPireps.Post("/pireps/submit", strictServer.SubmitPirep)
	})

	v1.Group(func(adminPireps chi.Router) {
		adminPireps.Use(middleware.IsAdminMiddleware())
		adminPireps.Post("/admin/flight-modes/config", strictServer.SaveFlightModesConfig)
	})
}

// uiAuthMiddleware checks for a valid session directly and renders the 401 error page
// if no session is found. Unlike the API AuthMiddleware, this never calls http.Error —
// it renders a proper HTML page so the browser displays it correctly.
func uiAuthMiddleware(
	sessionSvc *session.SessionService,
	templateRenderer *templates.Renderer,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check session cookie
			cookie, err := r.Cookie("session_id")
			if err != nil {
				render401(w, templateRenderer)
				return
			}

			sess, err := sessionSvc.GetSession(r.Context(), cookie.Value)
			if err != nil || sess == nil {
				render401(w, templateRenderer)
				return
			}

			if time.Now().After(sess.ExpiresAt) {
				render401(w, templateRenderer)
				return
			}

			activeVA := sess.GetActiveVA()
			if activeVA == nil {
				render401(w, templateRenderer)
				return
			}

			// Valid session — build claims and set context, same as AuthMiddleware
			userClaims := &auth.APIKeyClaims{
				UserUUID:           sess.UserID,
				VaUUID:             sess.ActiveVAID,
				RoleValue:          roles.VARole(activeVA.Role),
				DiscordUIDVal:      sess.DiscordID,
				DiscordServerIDVal: activeVA.DiscordServerID,
			}

			ctx := auth.SetUserClaims(r.Context(), userClaims)
			ctx = auth.SetSessionData(ctx, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// render401 renders the 401 Unauthorized error page
func render401(w http.ResponseWriter, templateRenderer *templates.Renderer) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	data := map[string]interface{}{
		"PageTitle": "Unauthorized",
	}
	if err := templateRenderer.RenderStandalone(w, "pages/401.html", data); err != nil {
		logging.Error("Failed to render 401 page", "error", err)
	}
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
