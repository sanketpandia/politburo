package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformUsers "infinite-experiment/politburo/internal/platform/users"

	"github.com/go-chi/chi/v5"
)

// tokenSessionCreator is a narrow interface for the token-login path, enabling test injection.
type tokenSessionCreator interface {
	CreateSessionFromToken(ctx context.Context, token string) (*CreateSessionFromTokenResult, error)
}

type handlerService interface {
	GetUserAndVAFromDiscordIDs(ctx context.Context, discordUserID string, discordServerID string) (*platformUsers.User, VAInfo, error)
	GenerateSignedLink(ctx context.Context, userID string, vaID string, redirectTo string, ttl time.Duration) (string, error)
	DestroyAllSessionsByIFCId(ctx context.Context, ifcId string) (int, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// Handler provides HTTP handlers for authentication endpoints
type Handler struct {
	svc      handlerService
	tokenSvc tokenSessionCreator
	renderer *templates.Renderer
}

// NewHandler creates a new auth handler
func NewHandler(svc handlerService, renderer *templates.Renderer) *Handler {
	handler := &Handler{
		svc:      svc,
		renderer: renderer,
	}
	if tokenSvc, ok := svc.(tokenSessionCreator); ok {
		handler.tokenSvc = tokenSvc
	}
	return handler
}

// GetUIBaseURL extracts the UI base URL from environment or request headers
// This is a helper function that can be used by other packages to get the UI base URL
func GetUIBaseURL(r *http.Request) string {
	uiBaseURL := os.Getenv("UI_BASE_URL")
	if uiBaseURL != "" {
		return uiBaseURL
	}

	// Fallback to request headers
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
	return scheme + "://" + forwardedHost
}

// FormatSignedLinkURL formats a token into a complete signed link URL
// This is a helper function that can be used by other packages to format signed links
func FormatSignedLinkURL(baseURL, token string) string {
	return fmt.Sprintf("%s/auth/login?token=%s", baseURL, token)
}

// TokenLogin handles presigned URL login (?token=...)
// This handler is created but not registered yet - will be used in future router
func (h *Handler) TokenLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query parameter
		token := r.URL.Query().Get("token")

		// No token provided — render expired state with empty token prefix.
		if token == "" {
			h.renderExpired(w, "")
			return
		}

		// Create session from token
		result, err := h.tokenSvc.CreateSessionFromToken(r.Context(), token)
		if err != nil {
			logging.Error("Failed to create session from token", "error", err)
			logging.Warn("Auth login rendered expired state", "token_prefix", shortToken(token))
			h.renderExpired(w, shortToken(token))
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
			_ = h.svc.DeleteSession(r.Context(), cookie.Value)
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

		// Get the UI base URL and format the signed link URL
		uiBaseURL := GetUIBaseURL(r)
		signedLinkURL := FormatSignedLinkURL(uiBaseURL, token)

		// Return JSON response
		response := map[string]interface{}{
			"url":         signedLinkURL,
			"expires_in":  int(ttl.Seconds()),
			"redirect_to": req.RedirectTo,
		}

		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// VerifyGodMode handles GET /api/v1/admin/verify-god
// Returns {"is_god": bool} based on whether the authenticated caller has god-mode access.
// Always responds 200 — the bot checks the is_god field rather than the status code.
func (h *Handler) VerifyGodMode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		isGod := IsGodMode(r)

		httpdto.WriteSuccess(w, initTime, map[string]bool{"is_god": isGod}, http.StatusOK)
	}
}

// renderExpired renders the auth-login page in its expired/invalid state.
// tokenShort is the first 6 characters of the token (or empty string when no token was provided).
func (h *Handler) renderExpired(w http.ResponseWriter, tokenShort string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	data := map[string]interface{}{
		"PageTitle":    "Sign-in link expired",
		"TokenExpired": true,
		"TokenShort":   tokenShort,
	}
	if err := h.renderer.RenderStandalone(w, "pages/auth-login.html", data); err != nil {
		logging.Error("Failed to render auth-login page", "error", err)
		http.Error(w, "Sign-in unavailable", http.StatusInternalServerError)
	}
}

// shortToken returns the first 6 characters of t, or t itself if t is shorter.
func shortToken(t string) string {
	if len(t) > 6 {
		return t[:6]
	}
	return t
}

// DestroySessionsByIFCId handles POST /api/v1/admin/sessions/destroy/{ifc_id}
// Destroys all sessions for a user identified by their IFC ID
func (h *Handler) DestroySessionsByIFCId() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Extract IFC ID from path parameter
		ifcId := chi.URLParam(r, "ifc_id")
		if ifcId == "" {
			logging.Warn("DestroySessionsByIFCId: missing IFC ID")
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "IFC ID is required", http.StatusBadRequest)
			return
		}

		logging.Info("Destroying all sessions for user", "ifc_id", ifcId)

		// Destroy all sessions for this user
		deletedCount, err := h.svc.DestroyAllSessionsByIFCId(r.Context(), ifcId)
		if err != nil {
			logging.Error("Failed to destroy sessions", "error", err, "ifc_id", ifcId)
			httpdto.WriteError(w, initTime, "DESTROY_FAILED", err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"ifc_id":        ifcId,
			"deleted_count": deletedCount,
			"message":       fmt.Sprintf("Successfully destroyed %d session(s)", deletedCount),
		}

		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}
