package middleware

import (
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/internal/auth"
	authCtx "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/db/repositories"
	"log"
	"net/http"
	"strings"
	"time"
)

func AuthMiddleware(
	userRepo *repositories.UserRepositoryGORM,
	keysRepo *repositories.KeysRepo,
	sessionSvc *session.SessionService,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[AuthMiddleware] Request to %s, Method: %s", r.RequestURI, r.Method)
			log.Printf("[AuthMiddleware] Cookies in request: %v", r.Cookies())

			var claims auth.UserClaims

			// CHECK 1: Session cookie (web users)
			if cookie, err := r.Cookie("session_id"); err == nil {
				sessionID := cookie.Value
				log.Printf("[AuthMiddleware] Found session cookie: %s (length=%d)", sessionID, len(sessionID))
				log.Printf("[AuthMiddleware] Cookie details: Domain=%s, Path=%s, HttpOnly=%v, Secure=%v, SameSite=%v",
					cookie.Domain, cookie.Path, cookie.HttpOnly, cookie.Secure, cookie.SameSite)

				session, err := sessionSvc.GetSession(r.Context(), sessionID)
				if err != nil {
					log.Printf("[AuthMiddleware] ERROR: Failed to get session from Redis: %v", err)
				} else if session != nil {
					log.Printf("[AuthMiddleware] SUCCESS: Session found for user: %s, expires at: %v", session.UserID, session.ExpiresAt)
					if time.Now().Before(session.ExpiresAt) {
						// Valid session - create claims from session data
						activeVA := session.GetActiveVA()
						if activeVA != nil {
							log.Printf("[AuthMiddleware] Active VA found: %s (role=%s)", activeVA.VAName, activeVA.Role)
							claims = &auth.APIKeyClaims{
								UserUUID:           session.UserID,
								VaUUID:             session.ActiveVAID,
								RoleValue:          roles.VARole(activeVA.Role),
								DiscordUIDVal:      session.DiscordID,
								DiscordServerIDVal: activeVA.DiscordServerID,
							}

							log.Printf("[AuthMiddleware] PROCEEDING: Valid session established for user %s", session.UserID)
							// Store session in context for VA switcher
							ctx := authCtx.SetUserClaims(r.Context(), claims)
							ctx = authCtx.SetSessionData(ctx, session)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						} else {
							log.Printf("[AuthMiddleware] ERROR: No active VA in session for user %s", session.UserID)
						}
					} else {
						log.Printf("[AuthMiddleware] ERROR: Session expired at %v (now=%v)", session.ExpiresAt, time.Now())
					}
				}
			} else {
				log.Printf("[AuthMiddleware] DEBUG: No session cookie found: %v (checking other auth methods)", err)
			}

			// CHECK 2: Bearer token (JWT - currently stubbed)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				// Parse JWT and validate
				claims = &auth.JWTClaims{
					UserUUID:  "user123",
					RoleValue: "PILOT",
				}
				http.Error(w, "Unauthorized. JWT not yet implemented", http.StatusUnauthorized)
				return
			}

			// CHECK 3: API Key (Discord bot and external services)
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				serverId := r.Header.Get("X-Server-Id")
				userId := r.Header.Get("X-Discord-Id")

				keyRes, err := keysRepo.GetStatus(r.Context(), apiKey)
				if err != nil {
					http.Error(w, "Unauthorized. Invalid API Key", http.StatusUnauthorized)
					return
				}

				if !keyRes.Status {
					http.Error(w, "Unauthorized. Inactive API Key", http.StatusUnauthorized)
					return
				}

				claims = auth.MakeClaimsFromApi(r.Context(), userRepo, serverId, userId)
				ctx := authCtx.SetUserClaims(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No valid auth found
			http.Error(w, "Unauthorized. Unknown Error", http.StatusUnauthorized)
		})
	}
}
