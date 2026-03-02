package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/events"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/middleware"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/sync"

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

// getAdditionalDataKeys extracts keys from AdditionalData map for logging
func getAdditionalDataKeys(additionalData map[string]interface{}) []string {
	if additionalData == nil {
		return []string{}
	}
	keys := make([]string, 0, len(additionalData))
	for k := range additionalData {
		keys = append(keys, k)
	}
	return keys
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

		// PIREP endpoints - require registration
		v1.Route("/pireps", func(pireps chi.Router) {
			pireps.Use(middleware.IsRegisteredMiddleware())
			pireps.Get("/config", func(w http.ResponseWriter, r *http.Request) {
				logging.Info("PIREP config endpoint called (not yet implemented)")
				w.WriteHeader(http.StatusNotImplemented)
				w.Write([]byte(`{"status":"not_implemented","message":"PIREP config endpoint not yet implemented"}`))
			})
			pireps.Post("/submit", handleTourPirepSubmit(application))
		})

		// Events endpoints - require registration
		v1.Route("/events", func(events chi.Router) {
			events.Use(middleware.IsRegisteredMiddleware())
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

			// Session management API
			adminAPI.Route("/sessions", func(sessions chi.Router) {
				sessions.Post("/destroy/{ifc_id}", authHandler.DestroySessionsByIFCId())
			})

			// Livery mappings API
			adminAPI.Route("/livery-mappings", func(liveryMappings chi.Router) {
				liveryMappings.Get("/", application.Features.LiveryMappingsHandler.ListMappingsHandler())
				liveryMappings.Post("/", application.Features.LiveryMappingsHandler.CreateMappingHandler())
				liveryMappings.Delete("/{id}", application.Features.LiveryMappingsHandler.DeleteMappingHandler())
				liveryMappings.Get("/liveries", application.Features.LiveryMappingsHandler.GetAvailableLiveriesHandler())
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

			// Livery Mappings
			admin.Get("/settings/livery-mappings", application.Features.LiveryMappingsHandler.ListMappingsPageHandler())
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

// handleTourPirepSubmit handles POST /api/v1/pireps/submit for tour PIREPs
// Fetches user's community ID, matches flight, validates against tour legs, and prepares Airtable payload
func handleTourPirepSubmit(application *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Tour PIREP submit: missing claims")
			http.Error(w, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()
		discordUserID := claims.DiscordUserID()

		if vaDiscordServerID == "" {
			logging.Warn("Tour PIREP submit: VA not found in claims")
			http.Error(w, "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA
		va, err := application.Platform.VASvc.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil || va == nil {
			logging.Warn("Tour PIREP submit: failed to get VA", "error", err)
			http.Error(w, "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Parse request body
		var submitRequest dtos.PirepSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&submitRequest); err != nil {
			logging.Warn("Tour PIREP submit: invalid request body", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get user with VA affiliations
		userRepo := users.NewRepository(application.Infra.DB)
		user, err := userRepo.GetUserWithVAAffiliations(r.Context(), discordUserID)
		if err != nil || user == nil {
			logging.Warn("Tour PIREP submit: user not found", "discord_id", discordUserID, "error", err)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Get user's IFCommunityID
		if user.IFCommunityID == "" {
			logging.Warn("Tour PIREP submit: user has no IFCommunityID", "user_id", user.ID)
			http.Error(w, "User has no Infinite Flight community ID", http.StatusBadRequest)
			return
		}

		// Get tour flight mode from config (defaults to "tour" if not configured)
		tourFlightMode, _ := application.Platform.VAConfigSvc.GetConfigVal(r.Context(), va.ID, platformVA.ConfigKeyTourFlightMode)
		if tourFlightMode == "" {
			tourFlightMode = "tour" // Default fallback
		}

		logging.Info("Tour PIREP submit: processing", "user_id", user.ID, "if_community_id", user.IFCommunityID, "va_id", va.ID, "mode", tourFlightMode)

		// Get VA flights from cache
		flightDTOs, err := flights.GetVALiveFlightsDTOs(application.Infra.RedisCache, va.ID)
		if err != nil {
			logging.Warn("Tour PIREP submit: failed to get VA flights", "error", err)
			http.Error(w, "Failed to fetch live flights", http.StatusInternalServerError)
			return
		}

		if len(flightDTOs) == 0 {
			logging.Warn("Tour PIREP submit: no flights found for VA", "va_id", va.ID)
			http.Error(w, "No live flights found", http.StatusNotFound)
			return
		}

		// Match user's flight by IFCommunityID
		// We need to get the user's IF API ID from their community ID
		// For now, we'll try to match by UserID if available, or we need to look it up
		var matchedFlight *flights.CompleteFlight
		var userIFApiID string

		// If user has IFApiID, use it directly
		if user.IFApiID != nil && *user.IFApiID != "" {
			userIFApiID = *user.IFApiID
		} else {
			// TODO: Look up IF API ID from community ID using Live API
			// For now, we'll search flights by matching other criteria
			logging.Info("Tour PIREP submit: user has no IFApiID, will match by other criteria")
		}

		// Get CompleteFlight objects from cache to match by UserID
		for _, flightDTO := range flightDTOs {
			flightKey := cache.LiveFlightKey(flightDTO.FlightID)
			flightVal, found := application.Infra.RedisCache.Get(flightKey)
			if !found {
				continue
			}

			// Convert to CompleteFlight
			jsonBytes, err := json.Marshal(flightVal)
			if err != nil {
				continue
			}

			var completeFlight flights.CompleteFlight
			if err := json.Unmarshal(jsonBytes, &completeFlight); err != nil {
				continue
			}

			// Match by UserID if we have it
			if userIFApiID != "" && completeFlight.UserID == userIFApiID {
				matchedFlight = &completeFlight
				logging.Info("Tour PIREP submit: matched flight by UserID", "flight_id", completeFlight.FlightID, "user_id", completeFlight.UserID)
				break
			}
		}

		if matchedFlight == nil {
			logging.Warn("Tour PIREP submit: no matching flight found", "if_community_id", user.IFCommunityID, "if_api_id", userIFApiID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Could not identify your flight. Please ensure you are currently in the game's server with your VA callsign.",
				"result": map[string]interface{}{
					"success":       false,
					"error_message": "Could not identify your flight. Please ensure you are currently in the game's server with your VA callsign.",
					"error":         "FLIGHT_NOT_FOUND",
				},
			})
			return
		}

		// Get active tour (multi-leg event) for VA
		eventRepo := events.NewRepository(application.Infra.DB)
		eventSvc := events.NewService(eventRepo)
		activeTour, err := eventSvc.GetActiveMultiLegEvent(r.Context(), va.ID)
		if err != nil {
			logging.Warn("Tour PIREP submit: failed to get active tour", "error", err)
			http.Error(w, "Failed to get active tour", http.StatusInternalServerError)
			return
		}

		if activeTour == nil {
			logging.Warn("Tour PIREP submit: no active tour found", "va_id", va.ID)
			http.Error(w, "No active tour found", http.StatusNotFound)
			return
		}

		// Build route string from flight (Origin-Destination)
		flightRoute := ""
		if matchedFlight.Origin != "" && matchedFlight.Destination != "" {
			flightRoute = strings.ToUpper(matchedFlight.Origin + "-" + matchedFlight.Destination)
		}

		if flightRoute == "" {
			logging.Warn("Tour PIREP submit: flight has no route", "flight_id", matchedFlight.FlightID)
			http.Error(w, "Flight has no route information", http.StatusBadRequest)
			return
		}

		// Check route against tour legs (event legs have Origin-Destination, not RouteName)
		var matchedLeg *events.EventLeg
		for i := range activeTour.Legs {
			// Event legs use Origin-Destination format
			legRoute := strings.ToUpper(activeTour.Legs[i].Origin + "-" + activeTour.Legs[i].Destination)
			if legRoute == flightRoute {
				matchedLeg = &activeTour.Legs[i]
				logging.Info("Tour PIREP submit: matched leg",
					"leg_id", matchedLeg.ID,
					"leg_number", matchedLeg.LegNumber,
					"route", legRoute,
					"route_at_id", matchedLeg.RouteATID)
				break
			}
		}

		if matchedLeg == nil {
			logging.Warn("Tour PIREP submit: route does not match any tour leg", "flight_route", flightRoute, "tour_id", activeTour.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Your flight plan start and end do not denote a tour route.",
				"result": map[string]interface{}{
					"success":       false,
					"error_message": "Your flight plan start and end do not denote a tour route.",
					"error":         "ROUTE_NOT_MATCHED",
					"route":         flightRoute,
				},
			})
			return
		}

		// Get route Airtable ID from the matched leg's RouteATID field
		// The event leg has a route_at_id field that directly maps to the Airtable route record ID
		var routeATID string
		if matchedLeg.RouteATID != nil && *matchedLeg.RouteATID != "" {
			// Use the RouteATID directly from the event leg - this is the Airtable ID we need
			routeATID = *matchedLeg.RouteATID
			logging.Info("Tour PIREP submit: using route Airtable ID from event leg", "route_at_id", routeATID, "leg_id", matchedLeg.ID, "leg_number", matchedLeg.LegNumber)
		} else {
			// Fallback: try to find route by origin-destination in sync repository
			// This happens if the event leg doesn't have a route_at_id set
			syncRepo := sync.NewRepository(application.Infra.DB)
			route, err := syncRepo.FindRouteByName(r.Context(), va.ID, flightRoute)
			if err == nil && route != nil {
				routeATID = route.ATID
				logging.Info("Tour PIREP submit: found route Airtable ID from sync repository", "route_at_id", routeATID, "route", flightRoute)
			} else {
				logging.Warn("Tour PIREP submit: no route Airtable ID found - event leg has no route_at_id and route not found in sync", "route", flightRoute, "leg_id", matchedLeg.ID)
			}
		}

		// Convert flight time from hh:mm to seconds
		flightTimeSeconds := 0
		flightTimeParts := strings.Split(submitRequest.FlightTime, ":")
		if len(flightTimeParts) == 2 {
			hours, _ := strconv.Atoi(flightTimeParts[0])
			minutes, _ := strconv.Atoi(flightTimeParts[1])
			flightTimeSeconds = (hours * 3600) + (minutes * 60)
		}

		// Extract multiplier from event leg additional data
		multiplier := 1 // Default multiplier (use 1 unless specified otherwise)
		if matchedLeg.AdditionalData != nil {
			logging.Debug("Tour PIREP submit: checking additional_data for multiplier",
				"leg_id", matchedLeg.ID,
				"leg_number", matchedLeg.LegNumber,
				"additional_data_keys", getAdditionalDataKeys(matchedLeg.AdditionalData))
			if multValue, exists := matchedLeg.AdditionalData["multiplier"]; exists {
				logging.Debug("Tour PIREP submit: found multiplier value",
					"value", multValue,
					"type", fmt.Sprintf("%T", multValue))
				// Handle different numeric types that might come from JSON
				switch v := multValue.(type) {
				case int:
					multiplier = v
				case int64:
					multiplier = int(v)
				case float64:
					multiplier = int(v)
				case string:
					// Try to parse as integer
					if parsed, err := strconv.Atoi(v); err == nil {
						multiplier = parsed
					} else {
						logging.Warn("Tour PIREP submit: failed to parse multiplier string",
							"value", v,
							"error", err)
					}
				default:
					logging.Warn("Tour PIREP submit: multiplier has unexpected type",
						"value", multValue,
						"type", fmt.Sprintf("%T", multValue))
				}
				logging.Info("Tour PIREP submit: extracted multiplier from event leg",
					"multiplier", multiplier,
					"leg_id", matchedLeg.ID,
					"leg_number", matchedLeg.LegNumber)
			} else {
				logging.Info("Tour PIREP submit: multiplier key not found in additional_data, using default 1",
					"leg_id", matchedLeg.ID,
					"leg_number", matchedLeg.LegNumber,
					"available_keys", getAdditionalDataKeys(matchedLeg.AdditionalData))
			}
		} else {
			logging.Info("Tour PIREP submit: additional_data is nil, using default multiplier 1",
				"leg_id", matchedLeg.ID,
				"leg_number", matchedLeg.LegNumber)
		}

		// Apply multiplier to flight duration
		originalFlightTimeSeconds := flightTimeSeconds
		if multiplier > 0 {
			flightTimeSeconds = flightTimeSeconds * multiplier
		}
		logging.Info("Tour PIREP submit: flight duration calculation",
			"multiplier", multiplier,
			"original_seconds", originalFlightTimeSeconds,
			"adjusted_seconds", flightTimeSeconds,
			"leg_id", matchedLeg.ID,
			"leg_number", matchedLeg.LegNumber)

		// Enrich the request with default mandatory values
		// Create a copy to avoid modifying the original
		enrichedRequest := submitRequest

		// Calculate max speed and max altitude from waypoints
		var maxSpeed, maxAltitude *int
		if len(matchedFlight.Waypoints) > 0 {
			maxSpeedVal := matchedFlight.Waypoints[0].Speed
			maxAltitudeVal := matchedFlight.Waypoints[0].Altitude

			for _, wp := range matchedFlight.Waypoints {
				if wp.Speed > maxSpeedVal {
					maxSpeedVal = wp.Speed
				}
				if wp.Altitude > maxAltitudeVal {
					maxAltitudeVal = wp.Altitude
				}
			}

			maxSpeed = &maxSpeedVal
			maxAltitude = &maxAltitudeVal
		}

		// Generate flight comments with required format
		commentsParts := []string{}
		if submitRequest.FlightTime != "" {
			commentsParts = append(commentsParts, fmt.Sprintf("Actual FT: %s", submitRequest.FlightTime))
		}
		commentsParts = append(commentsParts, fmt.Sprintf("Multiplier: %d", multiplier))
		if flightRoute != "" {
			commentsParts = append(commentsParts, fmt.Sprintf("Actual Route from FPL: %s", flightRoute))
		}
		if maxSpeed != nil {
			commentsParts = append(commentsParts, fmt.Sprintf("Max Speed: %d knots", *maxSpeed))
		}
		if maxAltitude != nil {
			commentsParts = append(commentsParts, fmt.Sprintf("Max Altitude: %d ft", *maxAltitude))
		}

		// Append to existing remarks if present, otherwise set as new
		if len(commentsParts) > 0 {
			commentsText := strings.Join(commentsParts, "\n")
			if enrichedRequest.PilotRemarks != "" {
				enrichedRequest.PilotRemarks = enrichedRequest.PilotRemarks + "\n\n" + commentsText
			} else {
				enrichedRequest.PilotRemarks = commentsText
			}
		}

		// Resolve aircraft and livery names using mappings
		aircraftName := application.Platform.VAConfigSvc.ResolveAircraftName(r.Context(), va.ID, matchedFlight.LiveryID)
		if aircraftName == "" {
			// Fallback to original if mapping fails
			aircraftName = matchedFlight.AircraftName
		}

		liveryName := application.Platform.VAConfigSvc.ResolveLiveryName(r.Context(), va.ID, matchedFlight.LiveryID)
		if liveryName == "" {
			// Fallback to original if mapping fails
			liveryName = matchedFlight.LiveryName
		}

		// Get provider configs (following the pattern from pilot sync job)
		dataProviderConfigRepo := repositories.NewDataProviderConfigRepo(application.Infra.DB)

		// Get credentials config separately
		credentialsConfig, err := dataProviderConfigRepo.GetActiveConfigByType(r.Context(), va.ID, "airtable", "credentials")
		if err != nil {
			logging.Error("Tour PIREP submit: failed to get credentials config", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to get Airtable credentials configuration",
			})
			return
		}

		if credentialsConfig == nil {
			logging.Error("Tour PIREP submit: no active credentials config found", "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Airtable credentials are not configured. Please configure Airtable API key and Base ID in the datasource settings.",
			})
			return
		}

		// Parse credentials config data directly (credentials config is stored as flat structure)
		var credsData struct {
			APIKey       string            `json:"api_key"`
			BaseID       string            `json:"base_id"`
			SyncSettings dtos.SyncSettings `json:"sync_settings"`
		}
		credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
		if err != nil {
			logging.Error("Tour PIREP submit: failed to marshal credentials config", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse credentials configuration",
			})
			return
		}
		if err := json.Unmarshal(credsBytes, &credsData); err != nil {
			logging.Error("Tour PIREP submit: failed to parse credentials config", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse credentials configuration",
			})
			return
		}

		// Validate credentials
		if credsData.APIKey == "" || credsData.BaseID == "" {
			logging.Error("Tour PIREP submit: Airtable credentials are empty", "va_id", va.ID, "has_api_key", credsData.APIKey != "", "has_base_id", credsData.BaseID != "")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Airtable credentials are not configured. Please configure Airtable API key and Base ID in the datasource settings.",
			})
			return
		}

		// Get pirep schema config separately
		pirepConfig, err := dataProviderConfigRepo.GetActiveConfigByType(r.Context(), va.ID, "airtable", "pirep")
		if err != nil {
			logging.Error("Tour PIREP submit: failed to get pirep schema config", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to get PIREP schema configuration",
			})
			return
		}

		if pirepConfig == nil {
			logging.Error("Tour PIREP submit: no pirep schema configured", "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "PIREP schema is not configured. Please configure the PIREP schema in the datasource settings.",
			})
			return
		}

		// Parse pirep schema config data (this is just the EntitySchema, not full ProviderConfigData)
		var pirepSchema dtos.EntitySchema
		schemaBytes, err := json.Marshal(pirepConfig.ConfigData)
		if err != nil {
			logging.Error("Tour PIREP submit: failed to marshal pirep config data", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse PIREP schema configuration",
			})
			return
		}

		if err := json.Unmarshal(schemaBytes, &pirepSchema); err != nil {
			logging.Error("Tour PIREP submit: failed to parse pirep schema", "error", err, "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to parse PIREP schema configuration",
			})
			return
		}

		// Set entity_type if not set (for backward compatibility)
		if pirepSchema.EntityType == "" {
			pirepSchema.EntityType = "pirep"
		}

		// Validate that schema has fields configured
		if len(pirepSchema.Fields) == 0 {
			logging.Error("Tour PIREP submit: pirep schema has no fields configured", "va_id", va.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "PIREP schema has no fields configured. Please configure field mappings in the datasource settings.",
			})
			return
		}

		// Helper function to get Airtable field name from schema
		getFieldName := func(internalName string) string {
			fieldMapping := pirepSchema.GetFieldMapping(internalName)
			if fieldMapping != nil {
				logging.Debug("Tour PIREP submit: found field mapping",
					"internal_name", internalName,
					"airtable_name", fieldMapping.AirtableName)
				return fieldMapping.AirtableName
			}
			logging.Debug("Tour PIREP submit: field mapping not found", "internal_name", internalName)
			return "" // Return empty if field not found in schema
		}

		// Log all available field mappings for debugging
		fieldMappings := make([]map[string]string, len(pirepSchema.Fields))
		for i, f := range pirepSchema.Fields {
			fieldMappings[i] = map[string]string{
				"internal_name": f.InternalName,
				"airtable_name": f.AirtableName,
			}
		}
		logging.Info("Tour PIREP submit: available field mappings",
			"total_fields", len(pirepSchema.Fields),
			"fields", fieldMappings)

		// Prepare Airtable payload using schema field mappings
		mappedFields := make(map[string]interface{})

		// Flight Time (required field)
		if flightTimeField := getFieldName("flight_time"); flightTimeField != "" {
			mappedFields[flightTimeField] = flightTimeSeconds // Converted to seconds
		}

		// Get user's VARole for this VA to get Airtable Pilot ID (user already has UserVARoles preloaded)
		var userVARole *users.UserVARole
		for i := range user.UserVARoles {
			if user.UserVARoles[i].VAID == va.ID {
				userVARole = &user.UserVARoles[i]
				break
			}
		}

		// Pilot Callsign (required field) - must be an array of Airtable record IDs for linked records
		if callsignField := getFieldName("callsign"); callsignField != "" {
			if userVARole != nil && userVARole.AirtablePilotID != nil && *userVARole.AirtablePilotID != "" {
				// Send as array of Airtable record IDs (linked record field)
				mappedFields[callsignField] = []string{*userVARole.AirtablePilotID}
				logging.Info("Tour PIREP submit: set callsign field", "callsign_field", callsignField, "pilot_at_id", *userVARole.AirtablePilotID)
			} else {
				logging.Warn("Tour PIREP submit: user has no Airtable Pilot ID - callsign field will be empty",
					"va_id", va.ID,
					"user_id", user.ID,
					"callsign_field", callsignField,
					"has_varole", userVARole != nil,
					"has_pilot_id", userVARole != nil && userVARole.AirtablePilotID != nil)
				// Don't set callsign if AirtablePilotID is missing - this will cause validation error
				// but that's better than sending invalid data
			}
		} else {
			logging.Warn("Tour PIREP submit: callsign field mapping not found in schema", "va_id", va.ID)
		}

		// Aircraft
		if aircraftField := getFieldName("aircraft"); aircraftField != "" && aircraftName != "" {
			mappedFields[aircraftField] = aircraftName
		}

		// Airline (mapped from livery)
		if airlineField := getFieldName("airline"); airlineField != "" && liveryName != "" {
			mappedFields[airlineField] = liveryName
		}

		// Flight Mode
		if flightModeField := getFieldName("flight_mode"); flightModeField != "" {
			mappedFields[flightModeField] = "World Tour 10"
		}

		// Route - use Airtable ID if available
		if routeField := getFieldName("route_at_id"); routeField != "" {
			if routeATID != "" {
				// Route field should be an array of Airtable IDs for linked records
				mappedFields[routeField] = []string{routeATID}
				logging.Info("Tour PIREP submit: set route field", "route_field", routeField, "route_at_id", routeATID)
			} else {
				logging.Warn("Tour PIREP submit: route Airtable ID is empty - route field will be empty",
					"route_field", routeField,
					"flight_route", flightRoute,
					"leg_id", matchedLeg.ID)
				// Note: If routeATID is empty, we don't set the route field
				// as Airtable linked record fields require valid IDs
			}
		} else {
			logging.Warn("Tour PIREP submit: route_at_id field mapping not found in schema", "va_id", va.ID)
		}

		// IFC Username (optional)
		if ifcUsernameField := getFieldName("ifc_username"); ifcUsernameField != "" && user.IFCommunityID != "" {
			mappedFields[ifcUsernameField] = user.IFCommunityID
		}

		// Pilot Remarks - use enriched remarks that were built earlier
		if remarksField := getFieldName("pilot_remarks"); remarksField != "" {
			if enrichedRequest.PilotRemarks != "" {
				mappedFields[remarksField] = enrichedRequest.PilotRemarks
			}
		}

		// Mode-specific fields (Fuel, Cargo, Passengers)
		if submitRequest.FuelKg != nil {
			if fuelField := getFieldName("fuel_kg"); fuelField != "" {
				mappedFields[fuelField] = *submitRequest.FuelKg
			}
		}
		if submitRequest.CargoKg != nil {
			if cargoField := getFieldName("cargo_kg"); cargoField != "" {
				mappedFields[cargoField] = *submitRequest.CargoKg
			}
		}
		if submitRequest.Passengers != nil {
			if paxField := getFieldName("passengers"); paxField != "" {
				mappedFields[paxField] = *submitRequest.Passengers
			}
		}

		// Log the final Airtable payload before submission
		payloadJSON, _ := json.MarshalIndent(map[string]interface{}{"fields": mappedFields}, "", "  ")
		logging.Info("Tour PIREP submit: Airtable payload prepared",
			"tour_id", activeTour.ID,
			"leg_id", matchedLeg.ID,
			"leg_number", matchedLeg.LegNumber,
			"flight_id", matchedFlight.FlightID,
			"route", flightRoute,
			"payload", string(payloadJSON))

		// Convert dtos.EntitySchema to platformVA.EntitySchema
		vaSchema := convertDTOsEntitySchema(&pirepSchema)
		if vaSchema == nil {
			logging.Error("Tour PIREP submit: failed to convert schema")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to process schema",
			})
			return
		}

		// Extract credentials and set in context
		creds := &platformVA.ProviderCredentials{
			APIKey: credsData.APIKey,
			BaseID: credsData.BaseID,
			SyncSettings: platformVA.SyncSettings{
				BatchSize:          credsData.SyncSettings.BatchSize,
				RateLimitPerSecond: credsData.SyncSettings.RateLimitPerSecond,
				RetryAttempts:      credsData.SyncSettings.RetryAttempts,
				TimeoutSeconds:     credsData.SyncSettings.TimeoutSeconds,
			},
		}

		// Set credentials in context
		ctx := context.WithValue(r.Context(), "provider_credentials", creds)

		// Initialize Airtable provider
		airtableProvider := providers.NewAirtableProvider(application.Infra.RedisCache)

		// Submit to Airtable
		logging.Info("Tour PIREP submit: submitting to Airtable", "table", vaSchema.TableName)
		pirepID, err := airtableProvider.SubmitRecord(ctx, vaSchema, mappedFields)
		if err != nil {
			logging.Error("Tour PIREP submit: Airtable submission failed", "error", err, "table", vaSchema.TableName)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to submit PIREP to Airtable",
				"error":   err.Error(),
			})
			return
		}

		logging.Info("Tour PIREP submit: successfully submitted to Airtable", "pirep_id", pirepID)

		// Return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "PIREP submitted successfully",
			"data": map[string]interface{}{
				"pirep_id":   pirepID,
				"tour_id":    activeTour.ID,
				"leg_id":     matchedLeg.ID,
				"leg_number": matchedLeg.LegNumber,
				"route":      flightRoute,
			},
		})
	}
}
