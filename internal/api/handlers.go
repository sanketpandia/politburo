package api

// DEPRECATED: This file is deprecated in favor of feature-layer handlers
// Handlers are now initialized directly in router.go for clean architecture
// See: internal/memberships/handler.go for the new pattern

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"infinite-experiment/politburo/internal/auth"
)

// Handlers struct is deprecated - handlers are now initialized directly in router.go
type Handlers struct {
	deps *Dependencies
}

// NewHandlers is deprecated - not used in new architecture
func NewHandlers(deps *Dependencies) *Handlers {
	return &Handlers{
		deps: deps,
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
