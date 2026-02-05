package ui

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"infinite-experiment/politburo/infra/security"
	"infinite-experiment/politburo/infra/session"
	authctx "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/db/repositories"
)

// AuthHandler manages authentication routes
type AuthHandler struct {
	sessionSvc *session.SessionService
	urlSigner  *security.URLSignerService
	userRepo   *repositories.UserRepositoryGORM
	vaRoleRepo *repositories.VAUserRoleRepository
	vaGormRepo *repositories.VAGORMRepository
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	sessionSvc *session.SessionService,
	urlSigner *security.URLSignerService,
	userRepo *repositories.UserRepositoryGORM,
	vaRoleRepo *repositories.VAUserRoleRepository,
	vaGormRepo *repositories.VAGORMRepository,
) *AuthHandler {
	return &AuthHandler{
		sessionSvc: sessionSvc,
		urlSigner:  urlSigner,
		userRepo:   userRepo,
		vaRoleRepo: vaRoleRepo,
		vaGormRepo: vaGormRepo,
	}
}

// TokenLoginHandler handles presigned URL login (?token=...)
func (h *AuthHandler) TokenLoginHandler(w http.ResponseWriter, r *http.Request) {
	// Extract token from query parameter
	token := r.URL.Query().Get("token")

	// No token provided - show login page
	if token == "" {
		data := map[string]interface{}{
			"PageTitle": "Login",
		}
		RenderTemplate(w, "auth/login.html", data)
		return
	}

	// Validate token
	signedToken, err := h.urlSigner.ValidateToken(r.Context(), token)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid or expired token: %v", err), http.StatusUnauthorized)
		return
	}

	// Mark token as used (single-use enforcement)
	err = h.urlSigner.MarkTokenAsUsed(r.Context(), signedToken.TokenID)
	if err != nil {
		http.Error(w, "Failed to process token", http.StatusInternalServerError)
		return
	}

	// Fetch user data from database
	user, err := h.userRepo.GetByID(r.Context(), signedToken.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Fetch all VAs for this user
	vaRoles, err := h.vaRoleRepo.GetAllByUserID(r.Context(), signedToken.UserID)
	if err != nil {
		http.Error(w, "Failed to load user VAs", http.StatusInternalServerError)
		return
	}

	// Convert to VAMembership array
	var virtualAirlines []session.VAMembership
	for _, vaRole := range vaRoles {
		va, err := h.vaGormRepo.GetByID(r.Context(), vaRole.VAID)
		if err != nil {
			continue // Skip VAs that can't be loaded
		}
		virtualAirlines = append(virtualAirlines, session.VAMembership{
			VAID:            va.ID,
			VACode:          va.Code,
			VAName:          va.Name,
			Role:            string(vaRole.Role),
			DiscordServerID: va.DiscordID,
		})
	}

	// Create session with default VA from token
	username := ""
	if user.UserName != nil {
		username = *user.UserName
	}
	sessionID, err := h.sessionSvc.CreateSession(
		r.Context(),
		signedToken.UserID,
		signedToken.VAID,
		user.DiscordID,
		"", // Discord server ID will be set from active VA
		username,
		virtualAirlines,
	)
	if err != nil {
		log.Printf("[TokenLoginHandler] Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	log.Printf("[TokenLoginHandler] Session created: %s for user %s with %d VAs", sessionID, signedToken.UserID, len(virtualAirlines))

	// Set session cookie (7 days, HTTP-only)
	// Get the forwarded host from headers (set by Caddy reverse proxy), fallback to request Host
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	// Extract domain from host (remove port)
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx] // Remove port for cookie domain
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
		Value:    sessionID,
		Path:     "/",
		Domain:   host, // Use forwarded host for proper cookie domain
		HttpOnly: true,
		Secure:   isSecure,             // Only set for HTTPS
		SameSite: http.SameSiteLaxMode, // Lax allows same-site cookies
		MaxAge:   604800,               // 7 days in seconds
	}
	http.SetCookie(w, cookie)

	log.Printf("[TokenLoginHandler] Cookie set: Name=%s, Value=%s, Path=%s, Domain=%s, HttpOnly=%v, Secure=%v, SameSite=%v, MaxAge=%d",
		cookie.Name, cookie.Value, cookie.Path, cookie.Domain, cookie.HttpOnly, cookie.Secure, cookie.SameSite, cookie.MaxAge)
	log.Printf("[TokenLoginHandler] Response headers: %v", w.Header())

	// Redirect to dashboard
	log.Printf("[TokenLoginHandler] Redirecting to /dashboard with status 303")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// LogoutHandler clears session and redirects to login
func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Get session cookie
	cookie, err := r.Cookie("session_id")
	if err == nil {
		// Delete session from Redis
		h.sessionSvc.DeleteSession(r.Context(), cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		MaxAge: -1, // Delete cookie
	})

	// Redirect to login page
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

// SwitchVAHandler switches the active VA and returns updated dashboard content
func (h *AuthHandler) SwitchVAHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body (form data from HTMX)
	r.ParseForm()
	newVAID := r.FormValue("va_id")

	// Get session from context
	sessionData := authctx.GetSessionData(r.Context())
	if sessionData == nil {
		http.Error(w, "No session found", http.StatusUnauthorized)
		return
	}

	session, ok := sessionData.(*session.SessionData)
	if !ok {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	// Update active VA in session
	err := h.sessionSvc.SwitchActiveVA(r.Context(), session.SessionID, newVAID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to switch VA: %v", err), http.StatusBadRequest)
		return
	}

	// Redirect to dashboard via HTMX (backend-driven redirect)
	// HTMX will perform a client-side redirect when it receives this header
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}
