package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// Handler handles VA webhook CRUD (admin-only, scoped to current VA)
type Handler struct {
	webhookRepo *platformVA.WebhookRepo
	vaSvc       *platformVA.Service
}

// NewHandler creates a new webhooks handler
func NewHandler(webhookRepo *platformVA.WebhookRepo, vaSvc *platformVA.Service) *Handler {
	return &Handler{
		webhookRepo: webhookRepo,
		vaSvc:       vaSvc,
	}
}

// CreateWebhookRequest is the body for POST /webhooks
type CreateWebhookRequest struct {
	WebhookType string `json:"webhook_type"`
	WebhookURL  string `json:"webhook_url"`
	Label       string `json:"label"`
}

// List returns all webhooks for the current VA
func (h *Handler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()
		vaID, err := h.resolveVAID(r)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusUnauthorized)
			return
		}
		list, err := h.webhookRepo.ListByVA(r.Context(), vaID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to list webhooks", http.StatusInternalServerError)
			return
		}
		common.RespondSuccess(w, initTime, "Webhooks listed", list)
	}
}

// Create creates a webhook for the current VA
func (h *Handler) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()
		vaID, err := h.resolveVAID(r)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusUnauthorized)
			return
		}
		var body CreateWebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.WebhookType == "" {
			common.RespondError(w, initTime, nil, "webhook_type is required", http.StatusBadRequest)
			return
		}
		if body.WebhookURL == "" {
			common.RespondError(w, initTime, nil, "webhook_url is required", http.StatusBadRequest)
			return
		}
		// Only live_flights is supported for now
		if body.WebhookType != WebhookTypeLiveFlights {
			common.RespondError(w, initTime, nil, "webhook_type must be live_flights", http.StatusBadRequest)
			return
		}
		wb := &platformVA.VAWebhook{
			VAID:              vaID,
			WebhookType:       body.WebhookType,
			WebhookURL:        body.WebhookURL,
			Label:             body.Label,
			FrequencyMinutes: 30,
			IsActive:          true,
		}
		if err := h.webhookRepo.CreateWebhook(r.Context(), wb); err != nil {
			common.RespondError(w, initTime, err, "Failed to create webhook", http.StatusInternalServerError)
			return
		}
		common.RespondSuccess(w, initTime, "Webhook created", wb, http.StatusCreated)
	}
}

// Update updates a webhook (toggle is_active or update URL/label). Webhook must belong to current VA.
func (h *Handler) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()
		vaID, err := h.resolveVAID(r)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusUnauthorized)
			return
		}
		id := chi.URLParam(r, "id")
		if id == "" {
			common.RespondError(w, initTime, nil, "id is required", http.StatusBadRequest)
			return
		}
		wb, err := h.webhookRepo.GetWebhookByID(r.Context(), id)
		if err != nil || wb == nil {
			common.RespondError(w, initTime, err, "Webhook not found", http.StatusNotFound)
			return
		}
		if wb.VAID != vaID {
			common.RespondError(w, initTime, nil, "Webhook not found", http.StatusNotFound)
			return
		}
		var body struct {
			WebhookURL *string `json:"webhook_url"`
			Label      *string `json:"label"`
			IsActive   *bool   `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.WebhookURL != nil {
			wb.WebhookURL = *body.WebhookURL
		}
		if body.Label != nil {
			wb.Label = *body.Label
		}
		if body.IsActive != nil {
			wb.IsActive = *body.IsActive
		}
		if err := h.webhookRepo.UpdateWebhook(r.Context(), wb); err != nil {
			common.RespondError(w, initTime, err, "Failed to update webhook", http.StatusInternalServerError)
			return
		}
		common.RespondSuccess(w, initTime, "Webhook updated", wb)
	}
}

// Delete deletes a webhook. Webhook must belong to current VA.
func (h *Handler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()
		vaID, err := h.resolveVAID(r)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusUnauthorized)
			return
		}
		id := chi.URLParam(r, "id")
		if id == "" {
			common.RespondError(w, initTime, nil, "id is required", http.StatusBadRequest)
			return
		}
		wb, err := h.webhookRepo.GetWebhookByID(r.Context(), id)
		if err != nil || wb == nil {
			common.RespondError(w, initTime, err, "Webhook not found", http.StatusNotFound)
			return
		}
		if wb.VAID != vaID {
			common.RespondError(w, initTime, nil, "Webhook not found", http.StatusNotFound)
			return
		}
		if err := h.webhookRepo.DeleteWebhook(r.Context(), id); err != nil {
			common.RespondError(w, initTime, err, "Failed to delete webhook", http.StatusInternalServerError)
			return
		}
		common.RespondSuccess(w, initTime, "Webhook deleted", nil)
	}
}

func (h *Handler) resolveVAID(r *http.Request) (string, error) {
	claims := auth.GetUserClaims(r.Context())
	if claims == nil {
		return "", fmt.Errorf("unauthorized: missing claims")
	}
	discordServerID := claims.DiscordServerID()
	if discordServerID == "" {
		return "", fmt.Errorf("virtual airline not found")
	}
	va, err := h.vaSvc.GetByDiscordServerID(r.Context(), discordServerID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VA: %w", err)
	}
	if va == nil {
		return "", fmt.Errorf("virtual airline not found")
	}
	return va.ID, nil
}
