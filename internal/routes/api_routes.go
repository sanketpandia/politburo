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

	// Public routes with metrics
	r.Group(func(public chi.Router) {
		public.Use(middleware.MetricsMiddleware(metricsReg))
		public.Get("/public/flight", api.UserFlightMapHandler(legacyCacheSvc))
		public.Get("/public/flight/user", api.UserFlightsCacheHandler(legacyCacheSvc))
	})

	// API v1 routes
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(middleware.MetricsMiddleware(metricsReg))
		v1.Use(middleware.AuthMiddleware(userRepoGorm, keyRepo, sessionSvc)) // global: all routes must be authenticated (using GORM or session cookie)
		v1.Get("/user/details", handlers.GetUserDetails())
		v1.Get("/admin/verify-god", handlers.VerifyGodMode())

		// Registered users group
		v1.Group(func(registered chi.Router) {
			// God-only group (admin + staff + member + registered)
			registered.Group(func(god chi.Router) {
				god.Use(middleware.IsGodMiddleware())
				god.Delete("/users/delete", handlers.DeleteAllUsers())
			})
			registered.Use(middleware.IsRegisteredMiddleware())

			registered.Post("/server/init", handlers.InitServerRegistrationV2())

			// Dashboard link generation for UI access
			registered.Post("/auth/generate-dashboard-link", handlers.GenerateDashboardLinkHandler())

			// Member-only group (requires registered first)
			registered.Group(func(member chi.Router) {
				member.Use(middleware.IsMemberMiddleware())

				// Pilot stats endpoint - comprehensive stats including game stats (future) and provider data
				member.Get("/pilot/stats", handlers.GetPilotStats())

				// PIREP filing endpoints
				member.Get("/pireps/config", handlers.GetPirepConfig())
				member.Post("/pireps/submit", handlers.SubmitPirep())

				member.Get("/va/live", api.VaFlightsHandler(flightSvc))
				member.Get("/live/sessions", api.LiveServers(flightSvc))

				// Staff-only group (requires member + registered)
				member.Group(func(staff chi.Router) {
					staff.Use(middleware.IsStaffMiddleware())
					staff.Get("/user/{user_id}/flights", api.UserFlightsHandler(flightSvc, cfgSvc))
					staff.Post("/va/userSync", api.SyncUser(vaMgmtSvc))

					// Admin-only group (staff + member + registered)
					staff.Group(func(admin chi.Router) {
						admin.Use(middleware.IsAdminMiddleware())

						admin.Post("/va/setRole", api.SetRole(vaMgmtSvc))
						admin.Post("/va/configs", api.SetConfigKeys(cfgSvc))
						admin.Get("/va/configs", api.GetVAConfigs(cfgSvc))
						admin.Get("/va/configs/keys", api.ListConfigKeys(cfgSvc))
						admin.Get("/debug", api.DebugHandler(*atApiSvc, *syncSvc))

						// Data provider configuration management
						admin.Post("/admin/data-provider/config", api.SaveDataProviderConfigHandler(deps))

						// Flight mode configuration management
						admin.Post("/va/flight-modes/config", handlers.SetFlightModesConfig())

						// Background jobs management
						admin.Post("/admin/jobs/sync-pilots", jobsHandler.TriggerPilotSync())
						admin.Get("/admin/jobs/status", jobsHandler.GetJobStatus())

						// Airport data management
						admin.Post("/admin/data/sync-airports", api.SyncAirportsHandler(airportLoader))

					})
				})
			})
		})

		// Public
		v1.Post("/user/register/init", handlers.InitUserRegistrationV2())
		v1.Post("/user/register/link", handlers.LinkUserToVA())
	})
}
