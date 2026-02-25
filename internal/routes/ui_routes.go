package routes

// import (
// 	"infinite-experiment/politburo/infra/cache"
// 	"infinite-experiment/politburo/infra/metrics"
// 	"infinite-experiment/politburo/infra/providers"
// 	"infinite-experiment/politburo/infra/security"
// 	"infinite-experiment/politburo/infra/session"
// 	"infinite-experiment/politburo/internal/common"
// 	"infinite-experiment/politburo/internal/db/repositories"
// 	"infinite-experiment/politburo/internal/flights"
// 	"infinite-experiment/politburo/internal/middleware"
// 	"infinite-experiment/politburo/internal/pilots"
// 	"infinite-experiment/politburo/internal/services"
// 	vizbuUI "infinite-experiment/politburo/vizburo/ui"
// 	"net/http"
// 	"path/filepath"
// 	"strings"

// 	"github.com/go-chi/chi/v5"
// )

// // RegisterUIRoutes registers all UI-related routes
// func RegisterUIRoutes(
// 	r chi.Router,
// 	metricsReg *metrics.MetricsRegistry,
// 	sessionSvc *session.SessionService,
// 	urlSigner *security.URLSignerService,
// 	userRepo *repositories.UserRepositoryGORM,
// 	vaRoleRepo *repositories.VAUserRoleRepository,
// 	vaRepo *repositories.VAGORMRepository,
// 	flightSvc *flights.Service,
// 	cache cache.CacheInterface,
// 	liveAPI *common.LiveAPIService,
// 	configSvc *services.DataProviderConfigService,
// 	airtableProvider *providers.AirtableProvider,
// 	vaGormRepo *repositories.VAGormRepository,
// 	eventRepo *repositories.VAEventRepository,
// 	routeRepo *repositories.RouteATSyncedRepo,
// ) {
// 	authHandler := vizbuUI.NewAuthHandler(sessionSvc, urlSigner, userRepo, vaRoleRepo, vaRepo)

// 	// Initialize pilot management service (migrated to pilots package)
// 	pilotMgmtSvc := pilots.NewManagementService(vaRoleRepo)

// 	// Import middleware
// 	authMiddleware := middleware.AuthMiddleware(userRepo, nil, sessionSvc) // keysRepo is nil for UI routes

// 	// Static file serving (CSS, JS, images) with correct MIME types
// 	fileServer := http.FileServer(http.Dir("vizburo/ui/static"))
// 	r.Group(func(staticRoutes chi.Router) {
// 		staticRoutes.Use(middleware.MetricsMiddleware(metricsReg))
// 		staticRoutes.Handle("/static/*", http.StripPrefix("/static/", mimeTypeMiddleware(fileServer)))
// 	})

// 	// Default route - redirect to login
// 	r.Group(func(rootRoutes chi.Router) {
// 		rootRoutes.Use(middleware.MetricsMiddleware(metricsReg))
// 		rootRoutes.Get("/", func(w http.ResponseWriter, r *http.Request) {
// 			http.Redirect(w, r, "/auth/login", http.StatusMovedPermanently)
// 		})
// 	})

// 	// Auth routes (public) with metrics
// 	r.Group(func(auth chi.Router) {
// 		auth.Use(middleware.MetricsMiddleware(metricsReg))
// 		auth.Get("/auth/login", authHandler.TokenLoginHandler)
// 		auth.Post("/auth/logout", authHandler.LogoutHandler)
// 	})

// 	// Dashboard routes (require authentication)
// 	r.Route("/dashboard", func(dashboard chi.Router) {
// 		// Apply metrics and authentication middleware to all dashboard routes
// 		dashboard.Use(middleware.MetricsMiddleware(metricsReg))
// 		dashboard.Use(authMiddleware)

// 		// Main dashboard page (all authenticated users)
// 		dashboard.Get("/", vizbuUI.DashboardHandler)

// 		// HTMX VA switch endpoint (all authenticated users)
// 		dashboard.Post("/switch-va", authHandler.SwitchVAHandler)

// 		// Staff-only routes (staff + admin)
// 		dashboard.Group(func(staff chi.Router) {
// 			staff.Use(middleware.IsStaffMiddleware())

// 			// Live Flights page (staff + admin)
// 			// NOTE: When uncommenting, import flights package and add:
// 			// staff.Get("/live", flights.LiveFlightsPageHandler())
// 			//
// 			// This route displays live flights on a map using Gleo.
// 			// Accessible via signed links from /api/v1/flights/va endpoint.

// 			// Logbook page and endpoints (staff + admin)
// 			staff.Get("/logbook", vizbuUI.LogbookHandler)
// 			staff.Get("/logbook/flights", func(w http.ResponseWriter, r *http.Request) {
// 				vizbuUI.LogbookFlightsHandler(w, r, flightSvc)
// 			})
// 			staff.Get("/logbook/flight/{session_id}/{flight_id}/map", func(w http.ResponseWriter, r *http.Request) {
// 				vizbuUI.FlightMapHandler(w, r, cache, liveAPI, flightSvc)
// 			})
// 			staff.Get("/logbook/pilots/search", func(w http.ResponseWriter, r *http.Request) {
// 				vizbuUI.PilotSearchHandler(w, r, vaRoleRepo)
// 			})
// 			staff.Get("/logbook/map/reset", vizbuUI.MapResetHandler)

// 			// Pilots page and list endpoint (staff + admin can view)
// 			staff.Get("/pilots", vizbuUI.PilotsHandler)
// 			staff.Get("/pilots/list", func(w http.ResponseWriter, r *http.Request) {
// 				vizbuUI.PilotsListHandler(w, r, pilotMgmtSvc)
// 			})

// 			// Callsign update (staff + admin can update)
// 			staff.Post("/pilots/{pilot_id}/callsign", func(w http.ResponseWriter, r *http.Request) {
// 				vizbuUI.UpdatePilotCallsignHandler(w, r, pilotMgmtSvc)
// 			})

// 			// Admin-only routes (admin only)
// 			staff.Group(func(admin chi.Router) {
// 				admin.Use(middleware.IsAdminMiddleware())

// 				// Pilots management (admin only)
// 				admin.Post("/pilots/{pilot_id}/role", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.UpdatePilotRoleHandler(w, r, pilotMgmtSvc)
// 				})
// 				admin.Delete("/pilots/{pilot_id}", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.RemovePilotHandler(w, r, pilotMgmtSvc)
// 				})

// 				// Datasource configuration (admin only)
// 				admin.Get("/settings/datasource", vizbuUI.DatasourceSettingsHandler)
// 				admin.Get("/settings/datasource/config", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetDatasourceConfigHandler(w, r, configSvc)
// 				})
// 				admin.Post("/settings/datasource/config", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.SaveDatasourceConfigHandler(w, r, configSvc)
// 				})
// 				admin.Post("/settings/datasource/test", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.TestConnectionHandler(w, r, airtableProvider)
// 				})
// 				admin.Post("/settings/datasource/table-fields", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.FetchTableFieldsHandler(w, r, airtableProvider, configSvc)
// 				})

// 				// PIREP configuration (admin only)
// 				admin.Get("/settings/pirep", vizbuUI.PirepConfigHandler)
// 				admin.Get("/settings/pirep/config", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetPirepConfigHandler(w, r, vaGormRepo)
// 				})
// 				admin.Get("/settings/pirep/mode/{mode_id}/edit", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetPirepModeEditHandler(w, r, vaGormRepo)
// 				})
// 				admin.Post("/settings/pirep/mode/{mode_id}/toggle", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.TogglePirepModeHandler(w, r, vaGormRepo)
// 				})
// 				admin.Post("/settings/pirep/mode/{mode_id}/update", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.UpdatePirepModeHandler(w, r, vaGormRepo)
// 				})

// 				// Events management (admin only)
// 				admin.Get("/events", vizbuUI.EventsHandler)
// 				admin.Get("/events/list", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetEventsListHandler(w, r, eventRepo, routeRepo)
// 				})
// 				admin.Get("/events/form", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetEventFormHandler(w, r, eventRepo, routeRepo)
// 				})
// 				admin.Get("/events/form/{event_id}", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.GetEventFormHandler(w, r, eventRepo, routeRepo)
// 				})
// 				admin.Get("/events/routes/search", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.RouteSearchHandler(w, r, routeRepo)
// 				})
// 				admin.Post("/events/create", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.CreateEventHandler(w, r, eventRepo)
// 				})
// 				admin.Post("/events/{event_id}/update", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.UpdateEventHandler(w, r, eventRepo)
// 				})
// 				admin.Delete("/events/{event_id}", func(w http.ResponseWriter, r *http.Request) {
// 					vizbuUI.DeleteEventHandler(w, r, eventRepo)
// 				})
// 			})
// 		})
// 	})

// 	// UI API routes
// 	r.Route("/ui/api", func(uiApi chi.Router) {
// 		uiApi.Use(middleware.MetricsMiddleware(metricsReg))
// 		uiApi.Get("/health", func(w http.ResponseWriter, r *http.Request) {
// 			w.Header().Set("Content-Type", "application/json")
// 			w.WriteHeader(http.StatusOK)
// 			w.Write([]byte(`{"status": "ok"}`))
// 		})
// 	})
// }

// // mimeTypeMiddleware wraps a file server and sets correct MIME types for various file types
// func mimeTypeMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Get the file extension
// 		ext := filepath.Ext(r.URL.Path)

// 		// Set correct MIME type for .mjs files (ES modules)
// 		if strings.EqualFold(ext, ".mjs") {
// 			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
// 		}

// 		// Call the wrapped handler
// 		next.ServeHTTP(w, r)
// 	})
// }
