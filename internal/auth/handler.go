package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

// Handler provides HTTP handlers for authentication endpoints
type Handler struct {
	svc *Service
}

// NewHandler creates a new auth handler
func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// TokenLogin handles presigned URL login (?token=...)
// This handler is created but not registered yet - will be used in future router
func (h *Handler) TokenLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query parameter
		token := r.URL.Query().Get("token")

		// No token provided - return error (UI rendering handled separately)
		if token == "" {
			http.Error(w, "Token required", http.StatusBadRequest)
			return
		}

		// Create session from token
		result, err := h.svc.CreateSessionFromToken(r.Context(), token)
		if err != nil {
			logging.Error("Failed to create session from token", "error", err)
			http.Error(w, fmt.Sprintf("Invalid or expired token: %v", err), http.StatusUnauthorized)
			return
		}

		// Set session cookie (7 days, HTTP-only)
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}

		// Extract domain from host (remove port)
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			host = host[:idx]
		}

		// Determine if HTTPS is being used
		scheme := r.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
			if r.TLS != nil {
				scheme = "https"
			}
		}
		isSecure := scheme == "https"

		cookie := &http.Cookie{
			Name:     "session_id",
			Value:    result.SessionID,
			Path:     "/",
			Domain:   host,
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   604800, // 7 days in seconds
		}
		http.SetCookie(w, cookie)

		logging.Info("Session cookie set", "session_id", result.SessionID, "redirect_to", result.RedirectTo)

		// Redirect to the specified URL (or default dashboard)
		http.Redirect(w, r, result.RedirectTo, http.StatusSeeOther)
	}
}

// Logout clears session and redirects to login
// This handler is created but not registered yet - will be used in future router
func (h *Handler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie("session_id")
		if err == nil {
			// Delete session from Redis
			h.svc.sessionSvc.DeleteSession(r.Context(), cookie.Value)
		}

		// Clear session cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "session_id",
			MaxAge: -1, // Delete cookie
		})

		// Redirect to login page
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
	}
}

// GenerateSignedLinkRequest represents the request body for generating signed links
type GenerateSignedLinkRequest struct {
	RedirectTo string `json:"redirect_to"`
	TTLMinutes int    `json:"ttl_minutes"`
}

// GenerateSignedLink handles POST /api/v1/signed-link
// Generates a signed link with redirect URL support
func (h *Handler) GenerateSignedLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context (set by AuthMiddleware with API key from bot)
		claims := GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /signed-link - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		discordID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// Parse request body
		var req GenerateSignedLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logging.Warn("Invalid request body", "error", err)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// Default values
		if req.RedirectTo == "" {
			req.RedirectTo = "/dashboard"
		}
		ttl := 15 * time.Minute
		if req.TTLMinutes > 0 {
			ttl = time.Duration(req.TTLMinutes) * time.Minute
		}

		// Lookup user and VA from database
		user, vaInfo, err := h.svc.GetUserAndVAFromDiscordIDs(r.Context(), discordID, discordServerID)
		if err != nil {
			logging.Error("Failed to lookup user or VA", "error", err, "discord_id", discordID, "server_id", discordServerID)
			httpdto.WriteError(w, initTime, "NOT_FOUND", err.Error(), http.StatusNotFound)
			return
		}

		// Generate signed link using service method
		token, err := h.svc.GenerateSignedLink(
			r.Context(),
			user.ID,
			vaInfo.ID,
			req.RedirectTo,
			ttl,
		)
		if err != nil {
			logging.Error("Failed to generate signed link", "error", err)
			httpdto.WriteError(w, initTime, "GENERATION_FAILED", "Failed to generate link", http.StatusInternalServerError)
			return
		}

		// Get the UI base URL from environment, fallback to current request
		uiBaseURL := os.Getenv("UI_BASE_URL")
		if uiBaseURL == "" {
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

		// Return JSON response
		response := map[string]interface{}{
			"url":         fmt.Sprintf("%s/auth/login?token=%s", uiBaseURL, token),
			"expires_in":  int(ttl.Seconds()),
			"redirect_to": req.RedirectTo,
		}

		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}
