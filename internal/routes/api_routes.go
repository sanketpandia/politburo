package routes

import (
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/middleware"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/services"
	"infinite-experiment/politburo/internal/va"

	"github.com/go-chi/chi/v5"
)

// RegisterAPIRoutes registers all API v1 routes and handlers
// This keeps API route registration separate from the main router setup
func RegisterAPIRoutes(r chi.Router, metricsReg *metrics.MetricsRegistry, userRepoGorm *repositories.UserRepositoryGORM, keyRepo *repositories.KeysRepo,
	handlers *api.Handlers, legacyCacheSvc *cache.CacheService, cfgSvc *common.VAConfigService, vaMgmtSvc *services.VAManagementService,
	atApiSvc *common.AirtableApiService, syncSvc *services.AtSyncService, flightSvc *flights.Service, jobsHandler *api.JobsHandler, deps *api.Dependencies, airportLoader *common.AirportLoaderService, sessionSvc *session.SessionService) {

	// Initialize feature-specific handlers (NEW PATTERN - Phase 3)
	// TODO: Re-enable when pirep service is added to dependencies
	// pirepHandlers := pireps.NewHandler(...)
	pilotStatsHandler := pilots.NewHandler(deps.Services.PilotStats)
	worldTourHandlers := api.NewWorldTourHandlers(deps)
	flightHandlers := flights.NewHandler(deps.Services.Flights, cfgSvc, legacyCacheSvc)

	// VA handlers (consolidated in va package)
	vaHandler := va.NewHandler(
		deps.Services.VAService,
		deps.Services.VAConfig,
		deps.Services.VAEventService,
		userRepoGorm,
		deps.Repo.Va, // Legacy repo for FlightModesConfigService compatibility
	)

	// Public routes with metrics
	r.Group(func(public chi.Router) {
		public.Use(middleware.MetricsMiddleware(metricsReg))
		public.Get("/public/flight", flightHandlers.GetFlightFromCache())
		public.Get("/public/flight/user", flightHandlers.GetUserFlightsFromCache())
	})

	// API v1 routes
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(middleware.MetricsMiddleware(metricsReg))
		v1.Use(middleware.AuthMiddleware(userRepoGorm, keyRepo, sessionSvc)) // global: all routes must be authenticated (using GORM or session cookie)
		v1.Get("/user/details", handlers.User.GetDetails())
		v1.Get("/admin/verify-god", handlers.User.VerifyGodMode())

		// Registered users group
		v1.Group(func(registered chi.Router) {
			// God-only group (admin + staff + member + registered)
			registered.Group(func(god chi.Router) {
				god.Use(middleware.IsGodMiddleware())
				god.Delete("/users/delete", handlers.User.DeleteAllUsers())
			})
			registered.Use(middleware.IsRegisteredMiddleware())

			registered.Post("/server/init", handlers.User.InitServerRegistration())

			// Dashboard link generation for UI access
			registered.Post("/auth/generate-dashboard-link", handlers.GenerateDashboardLinkHandler())

			// Member-only group (requires registered first)
			registered.Group(func(member chi.Router) {
				member.Use(middleware.IsMemberMiddleware())

				// Pilot stats endpoint - comprehensive stats including game stats (future) and provider data
				member.Get("/pilot/stats", pilotStatsHandler.GetPilotStats())

				// PIREP filing endpoints
				// TODO: Re-enable when pirep handler is implemented
				// member.Get("/pireps/config", pirepHandlers.GetConfig())
				// member.Post("/pireps/submit", pirepHandlers.Submit())

				member.Get("/va/live", flightHandlers.GetVALiveFlights())
				member.Get("/live/sessions", flightHandlers.GetLiveSessions())

				// World Tour bot endpoints (member level)
				member.Get("/world-tour/active", worldTourHandlers.GetActiveTour())
				member.Get("/world-tour/{tour_id}/progress/{user_id}", worldTourHandlers.GetUserProgress())
				member.Get("/world-tour/{tour_id}/leg/{leg_number}", worldTourHandlers.GetTourLeg())
				member.Get("/world-tour/{tour_id}/leaderboard", worldTourHandlers.GetTourLeaderboard())
				member.Post("/world-tour/validate-route", worldTourHandlers.ValidateRoute())

				// Staff-only group (requires member + registered)
				member.Group(func(staff chi.Router) {
					staff.Use(middleware.IsStaffMiddleware())
					staff.Get("/user/{user_id}/flights", flightHandlers.GetUserFlights())
					staff.Post("/va/userSync", vaHandler.SyncUser())

					// Admin-only group (staff + member + registered)
					staff.Group(func(admin chi.Router) {
						admin.Use(middleware.IsAdminMiddleware())

						admin.Post("/va/setRole", vaHandler.SetRole())
						admin.Post("/va/configs", vaHandler.SetConfigs())
						admin.Get("/va/configs", vaHandler.GetConfigs())
						admin.Get("/va/configs/keys", vaHandler.ListConfigKeys())
						admin.Get("/debug", api.DebugHandler(*atApiSvc, *syncSvc))

						// Data provider configuration management
						admin.Post("/admin/data-provider/config", api.SaveDataProviderConfigHandler(deps))

						// Flight mode configuration management
						admin.Post("/va/flight-modes/config", vaHandler.SetFlightModesConfig())

						// Background jobs management
						admin.Post("/admin/jobs/sync-pilots", jobsHandler.TriggerPilotSync())
						admin.Get("/admin/jobs/status", jobsHandler.GetJobStatus())

						// Airport data management
						admin.Post("/admin/data/sync-airports", api.SyncAirportsHandler(airportLoader))

						// World Tour admin endpoints
						admin.Post("/world-tour", worldTourHandlers.CreateTour())
						admin.Get("/world-tour", worldTourHandlers.GetTours())
						admin.Put("/world-tour/{id}", worldTourHandlers.UpdateTour())
						admin.Delete("/world-tour/{id}", worldTourHandlers.DeleteTour())
						admin.Post("/world-tour/{tour_id}/legs", worldTourHandlers.AddLeg())
						admin.Put("/world-tour/legs/{leg_id}", worldTourHandlers.UpdateLeg())
						admin.Delete("/world-tour/legs/{leg_id}", worldTourHandlers.DeleteLeg())

					})
				})
			})
		})

		// Public
		v1.Post("/user/register/init", handlers.User.InitRegistrationV2())
		v1.Post("/user/register/link", handlers.User.LinkToVA())
	})
}
