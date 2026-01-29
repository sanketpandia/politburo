package routes

import (
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/metrics"
	"infinite-experiment/politburo/internal/middleware"
	"infinite-experiment/politburo/internal/services"

	"github.com/go-chi/chi/v5"
)

// RegisterAPIRoutes registers all API v1 routes and handlers
// This keeps API route registration separate from the main router setup
func RegisterAPIRoutes(r chi.Router, metricsReg *metrics.MetricsRegistry, userRepoGorm *repositories.UserRepositoryGORM, keyRepo *repositories.KeysRepo,
	handlers *api.Handlers, legacyCacheSvc *common.CacheService, cfgSvc *common.VAConfigService, vaMgmtSvc *services.VAManagementService,
	atApiSvc *common.AirtableApiService, syncSvc *services.AtSyncService, flightSvc *services.FlightsService, jobsHandler *api.JobsHandler, deps *api.Dependencies, airportLoader *common.AirportLoaderService, sessionSvc *common.SessionService) {

	// Initialize feature-specific handlers (NEW PATTERN - Phase 2.1, 2.2)
	pirepHandlers := api.NewPirepHandlers(deps)
	vaConfigHandlers := api.NewVAConfigHandlers(deps)
	pilotStatsHandlers := api.NewPilotStatsHandlers(deps)
	worldTourHandlers := api.NewWorldTourHandlers(deps)
	flightHandlers := api.NewFlightHandlers(deps)
	vaHandlers := api.NewVAHandlers(deps)

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
				member.Get("/pilot/stats", pilotStatsHandlers.GetPilotStats())

				// PIREP filing endpoints
				member.Get("/pireps/config", pirepHandlers.GetConfig())
				member.Post("/pireps/submit", pirepHandlers.Submit())

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
					staff.Post("/va/userSync", vaHandlers.SyncUser())

					// Admin-only group (staff + member + registered)
					staff.Group(func(admin chi.Router) {
						admin.Use(middleware.IsAdminMiddleware())

						admin.Post("/va/setRole", vaHandlers.SetRole())
						admin.Post("/va/configs", vaHandlers.SetConfigs())
						admin.Get("/va/configs", vaHandlers.GetConfigs())
						admin.Get("/va/configs/keys", vaHandlers.ListConfigKeys())
						admin.Get("/debug", api.DebugHandler(*atApiSvc, *syncSvc))

						// Data provider configuration management
						admin.Post("/admin/data-provider/config", api.SaveDataProviderConfigHandler(deps))

						// Flight mode configuration management
						admin.Post("/va/flight-modes/config", vaConfigHandlers.SetFlightModesConfig())

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
