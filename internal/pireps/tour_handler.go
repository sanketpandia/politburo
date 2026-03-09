package pireps

import (
	"encoding/json"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

// TourHandler handles HTTP requests for tour PIREP submission.
type TourHandler struct {
	tourSvc *TourPirepService
}

// NewTourHandler creates a new TourHandler.
func NewTourHandler(tourSvc *TourPirepService) *TourHandler {
	return &TourHandler{tourSvc: tourSvc}
}

// SubmitTourPirep handles POST /api/v1/pireps/submit for tour mode.
func (h *TourHandler) SubmitTourPirep() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// ── Auth ──────────────────────────────────────────────────────
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Tour PIREP submit: missing claims")
			httpdto.WriteError(w, start, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()
		discordUserID := claims.DiscordUserID()

		if vaDiscordServerID == "" {
			logging.Warn("Tour PIREP submit: VA not found in claims")
			httpdto.WriteError(w, start, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		// ── Parse body ───────────────────────────────────────────────
		var submitRequest dtos.PirepSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&submitRequest); err != nil {
			logging.Warn("Tour PIREP submit: invalid request body", "error", err)
			httpdto.WriteError(w, start, "BAD_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// ── Delegate to service ──────────────────────────────────────
		result, svcErr := h.tourSvc.Submit(r.Context(), vaDiscordServerID, discordUserID, &submitRequest)
		if svcErr != nil {
			httpdto.WriteError(w, start, svcErr.Code, svcErr.Message, svcErr.StatusCode)
			return
		}

		httpdto.WriteSuccess(w, start, result, http.StatusOK)
	}
}
