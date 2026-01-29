package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"infinite-experiment/politburo/internal/auth"
)

type Handlers struct {
	deps *Dependencies
	User *UserHandlers
}

// NewHandlers creates a new handlers instance with injected dependencies
func NewHandlers(deps *Dependencies) *Handlers {
	return &Handlers{
		deps: deps,
		User: NewUserHandlers(
			deps.Repo.User,
			deps.Repo.Va,
			deps.Services.User,
			deps.Services.RegV2,
		),
	}
}

// GenerateDashboardLinkHandler generates a presigned URL for dashboard access
func (h *Handlers) GenerateDashboardLinkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get claims from context (set by auth middleware)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Generate presigned URL (15 minute expiry)
		token, err := h.deps.Services.URLSigner.GeneratePresignedURL(
			claims.UserID(),
			claims.ServerID(),
			15*time.Minute,
		)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Get the UI base URL from environment, fallback to current host
		uiBaseURL := os.Getenv("UI_BASE_URL")
		if uiBaseURL == "" {
			// Fallback: construct from request headers
			scheme := r.Header.Get("X-Forwarded-Proto")
			if scheme == "" {
				scheme = "http"
				if r.TLS != nil {
					scheme = "https"
				}
			}
			forwardedHost := r.Header.Get("X-Forwarded-Host")
			if forwardedHost == "" {
				forwardedHost = r.Host
			}
			uiBaseURL = scheme + "://" + forwardedHost
		}

		// Create response
		response := map[string]interface{}{
			"status": true,
			"data": map[string]interface{}{
				"url":        uiBaseURL + "/auth/login?token=" + token,
				"expires_in": 900,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
