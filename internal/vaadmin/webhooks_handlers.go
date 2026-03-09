package vaadmin

import (
	"fmt"
	"net/http"
	"strings"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

const webhookTypeLiveFlights = "live_flights"

// WebhooksPageHandler handles GET /dashboard/vaadmin/webhooks
func (h *Handler) WebhooksPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}
		data, err := templates.PrepareTemplateData(sessionData, "VA Admin - Setup webhooks")
		if err != nil {
			logging.Error("Failed to prepare template data", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data["CurrentPage"] = "vaadmin-pilots" // webhooks is a sub-page of VA Admin; keep that nav item active
		if err := h.templateRenderer.RenderTemplate(w, "pages/vaadmin-webhooks.html", data); err != nil {
			logging.Error("Error rendering webhooks page", "error", err)
			http.Error(w, "Error rendering webhooks page", http.StatusInternalServerError)
			return
		}
	}
}

// WebhooksListHandler handles GET /dashboard/vaadmin/webhooks/list (HTMX partial)
func (h *Handler) WebhooksListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}
		list, err := h.webhookRepo.ListByVA(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to list webhooks", "error", err)
			http.Error(w, "Failed to list webhooks", http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{
			"Webhooks":         list,
			"ActiveVA":         activeVA,
			"RunNowAvailable":  h.liveFlightsRunner != nil,
		}
		if err := h.templateRenderer.RenderPartial(w, "partials/webhooks-list.html", data); err != nil {
			logging.Error("Error rendering webhooks list", "error", err)
			http.Error(w, "Error rendering webhooks list", http.StatusInternalServerError)
			return
		}
	}
}

// CreateWebhookFormHandler handles POST /dashboard/vaadmin/webhooks (HTMX form submit)
func (h *Handler) CreateWebhookFormHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		webhookURL := strings.TrimSpace(r.FormValue("webhook_url"))
		if webhookURL == "" {
			http.Error(w, "webhook_url is required", http.StatusBadRequest)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		wb := &platformVA.VAWebhook{
			VAID:             activeVA.VAID,
			WebhookType:      webhookTypeLiveFlights,
			WebhookURL:       webhookURL,
			Label:            label,
			FrequencyMinutes: 30,
			IsActive:         true,
		}
		if err := h.webhookRepo.CreateWebhook(r.Context(), wb); err != nil {
			logging.Error("Failed to create webhook", "error", err)
			http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
			return
		}
		list, _ := h.webhookRepo.ListByVA(r.Context(), activeVA.VAID)
		data := map[string]interface{}{
			"Webhooks":         list,
			"ActiveVA":         activeVA,
			"RunNowAvailable":  h.liveFlightsRunner != nil,
		}
		w.Header().Set("HX-Trigger", "webhook-created")
		if err := h.templateRenderer.RenderPartial(w, "partials/webhooks-list.html", data); err != nil {
			logging.Error("Error rendering webhooks list after create", "error", err)
			http.Error(w, "Error rendering webhooks list", http.StatusInternalServerError)
			return
		}
	}
}

// WebhooksRunNowHandler handles POST /dashboard/vaadmin/webhooks/run — runs live flights webhook for current VA (test).
func (h *Handler) WebhooksRunNowHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.liveFlightsRunner == nil {
			http.Error(w, "Run now not configured", http.StatusServiceUnavailable)
			return
		}
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}
		if err := h.liveFlightsRunner.RunForVA(r.Context(), activeVA.VAID); err != nil {
			logging.Error("Run now failed", "error", err, "va_id", activeVA.VAID)
			w.Header().Set("HX-Trigger", fmt.Sprintf(`{"webhookRunResult":{"ok":false,"message":%q}}`, err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("HX-Trigger", `{"webhookRunResult":{"ok":true,"message":"Live flights sent to Discord."}}`)
		w.WriteHeader(http.StatusOK)
	}
}
