package middleware

import (
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"log"
	"net/http"
)

func IsGodMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			claims := auth.GetUserClaims(r.Context())
			log.Printf("Discord User ID: %s", claims.DiscordUserID())

			if auth.IsGodMode(r) {
				next.ServeHTTP(w, r)
				return
			}
			common.RespondPermissionDenied(w, "god mode (system administrator)")

		})
	}

}

// IsGodMiddlewareWithKey requires both god-mode Discord user ID and a god-mode key header
// This provides an extra layer of security for sensitive operations
func IsGodMiddlewareWithKey() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			claims := auth.GetUserClaims(r.Context())
			godModeKeyHeader := r.Header.Get("X-God-Mode-Key")
			log.Printf("IsGodMiddlewareWithKey: Discord User ID: %s", claims.DiscordUserID())

			if auth.IsGodModeWithKey(claims.DiscordUserID(), godModeKeyHeader) {
				next.ServeHTTP(w, r)
				return
			}
			common.RespondPermissionDenied(w, "god mode (system administrator) with valid key")

		})
	}

}
