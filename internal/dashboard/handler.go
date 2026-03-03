package dashboard

import (
	"net/http"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
)

// Handler handles dashboard UI endpoints
type Handler struct {
	templateRenderer *templates.Renderer
	dashboardSvc     *Service
}

// NewHandler creates a new dashboard handler instance
func NewHandler(templateRenderer *templates.Renderer, dashboardSvc *Service) *Handler {
	return &Handler{
		templateRenderer: templateRenderer,
		dashboardSvc:     dashboardSvc,
	}
}

// DashboardPageHandler handles GET /dashboard
// Serves the main dashboard page with role-based cards
func (h *Handler) DashboardPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user claims from context (injected by auth middleware)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Get session data from context
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Cast to SessionData
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data, err := templates.PrepareTemplateData(sessionData, "Dashboard")
		if err != nil {
			logging.Error("Error preparing template data", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add current page identifier for menu highlighting
		data["CurrentPage"] = "dashboard"

		// Get active VA for leaderboard query
		activeVA := sessionData.GetActiveVA()
		if activeVA != nil {
			// Fetch leaderboard data
			leaderboard, activeEvent, err := h.dashboardSvc.GetEventLeaderboard(r.Context(), activeVA.VAID)
			if err != nil {
				logging.Warn("Failed to fetch leaderboard", "error", err, "va_id", activeVA.VAID)
				// Continue without leaderboard - not critical
				leaderboard = []LeaderboardEntry{}
			}
			data["Leaderboard"] = leaderboard
			data["ActiveEvent"] = activeEvent
		} else {
			data["Leaderboard"] = []LeaderboardEntry{}
			data["ActiveEvent"] = nil
		}

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/dashboard.html", data); err != nil {
			logging.Error("Error rendering dashboard page", "error", err)
			http.Error(w, "Error rendering dashboard page", http.StatusInternalServerError)
			return
		}
	}
}

// GetPilotPirepLogsHandler handles GET /dashboard/leaderboard/pilot/{pilot_at_id}/logs
// Returns JSON with pilot's PIREP logs for the active event
func (h *Handler) GetPilotPirepLogsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get session data from context
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Cast to SessionData
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		// Get active VA
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		// Get pilot_at_id from URL path
		pilotATID := r.URL.Query().Get("pilot_at_id")
		if pilotATID == "" {
			http.Error(w, "pilot_at_id parameter required", http.StatusBadRequest)
			return
		}

		// Fetch pilot logs
		logs, activeEvent, err := h.dashboardSvc.GetPilotPirepLogs(r.Context(), activeVA.VAID, pilotATID)
		if err != nil {
			logging.Error("Failed to fetch pilot logs", "error", err, "pilot_at_id", pilotATID)
			http.Error(w, "Failed to fetch pilot logs", http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data := map[string]interface{}{
			"logs":        logs,
			"activeEvent": activeEvent,
		}

		// Render HTML partial for HTMX
		if err := h.templateRenderer.RenderPartial(w, "partials/pilot-logs.html", data); err != nil {
			logging.Error("Failed to render pilot logs partial", "error", err)
			http.Error(w, "Failed to render pilot logs", http.StatusInternalServerError)
			return
		}
	}
}

// TestClickHandler handles GET /dashboard/test-click
// Test endpoint for HTMX click handler
func (h *Handler) TestClickHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Return empty HTML fragment since hx-swap="none" doesn't use the response
		w.Write([]byte("<!-- Click handler triggered -->"))
	}
}
