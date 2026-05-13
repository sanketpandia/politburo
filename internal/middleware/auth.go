package middleware

import (
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/internal/auth"
	authCtx "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/apikeys"
	"infinite-experiment/politburo/internal/platform/claims"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/roles"
)

// AuthMiddleware populates UserClaims in the request context.
// It tries three auth methods in order:
//  1. Session cookie  (web dashboard users)
//  2. API key headers (Discord bot and external services)
//
// The dead Bearer/JWT branch has been removed — it was never wired to a real
// token validator and always returned 401.
func AuthMiddleware(
	claimsRepo *claims.Repository,
	keysRepo *apikeys.Repository,
	sessionSvc *session.SessionService,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// CHECK 1: Session cookie (web users)
			if userClaims, ok := tryAuthFromSession(r, sessionSvc); ok {
				ctx := authCtx.SetUserClaims(r.Context(), userClaims.claims)
				ctx = authCtx.SetSessionData(ctx, userClaims.session)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// CHECK 2: API Key (Discord bot and external services)
			if userClaims, ok := tryAuthFromAPIKey(r, keysRepo, claimsRepo); ok {
				ctx := authCtx.SetUserClaims(r.Context(), userClaims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			logging.Debug("auth failed: no valid session cookie or API key",
				"method", r.Method,
				"path", r.URL.Path,
			)
			httpdto.WriteError(w, start, "UNAUTHORIZED", "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// sessionAuthResult bundles the claims and the raw session for context injection.
type sessionAuthResult struct {
	claims  auth.UserClaims
	session interface{} // *session.SessionData — kept as interface{} to avoid import cycle
}

// tryAuthFromSession validates the session_id cookie and returns populated
// claims if a valid, non-expired session with an active VA is found.
// Cookie values are never logged.
func tryAuthFromSession(r *http.Request, sessionSvc *session.SessionService) (sessionAuthResult, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		// No cookie present — not an error, just no session auth.
		return sessionAuthResult{}, false
	}

	sess, err := sessionSvc.GetSession(r.Context(), cookie.Value)
	if err != nil || sess == nil {
		logging.Debug("session not found or error", "method", r.Method, "path", r.URL.Path)
		return sessionAuthResult{}, false
	}

	if time.Now().After(sess.ExpiresAt) {
		logging.Debug("session expired", "method", r.Method, "path", r.URL.Path)
		return sessionAuthResult{}, false
	}

	activeVA := sess.GetActiveVA()
	if activeVA == nil {
		logging.Debug("session has no active VA", "method", r.Method, "path", r.URL.Path)
		return sessionAuthResult{}, false
	}

	return sessionAuthResult{
		claims: &auth.APIKeyClaims{
			UserUUID:           sess.UserID,
			VaUUID:             sess.ActiveVAID,
			RoleValue:          roles.VARole(activeVA.Role),
			DiscordUIDVal:      sess.DiscordID,
			DiscordServerIDVal: activeVA.DiscordServerID,
		},
		session: sess,
	}, true
}

// tryAuthFromAPIKey validates the X-API-Key header and builds claims from the
// X-Server-Id and X-Discord-Id headers.
func tryAuthFromAPIKey(r *http.Request, keysRepo *apikeys.Repository, claimsRepo *claims.Repository) (auth.UserClaims, bool) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		return nil, false
	}

	keyRes, err := keysRepo.GetStatus(r.Context(), apiKey)
	if err != nil || !keyRes.Status {
		logging.Debug("API key invalid or inactive", "method", r.Method, "path", r.URL.Path)
		return nil, false
	}

	serverID := r.Header.Get("X-Server-Id")
	userID := r.Header.Get("X-Discord-Id")

	return auth.MakeClaimsFromApi(r.Context(), claimsRepo, serverID, userID), true
}
