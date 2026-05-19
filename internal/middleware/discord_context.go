package middleware

import (
	"net/http"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

const (
	DiscordUserIDHeader   = "X-Discord-User-Id"
	DiscordServerIDHeader = "X-Discord-Server-Id"
)

// RequireDiscordBotContextMiddleware requires bot requests to include Discord
// user and server context headers after API-key authentication has succeeded.
func RequireDiscordBotContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			hasDiscordUserID := strings.TrimSpace(r.Header.Get(DiscordUserIDHeader)) != ""
			hasDiscordServerID := strings.TrimSpace(r.Header.Get(DiscordServerIDHeader)) != ""

			if !hasDiscordUserID || !hasDiscordServerID {
				logging.Warn("missing Discord bot context headers",
					"method", r.Method,
					"path", r.URL.Path,
					"has_discord_user_id", hasDiscordUserID,
					"has_discord_server_id", hasDiscordServerID,
				)
				httpdto.WriteError(
					w,
					start,
					"MISSING_DISCORD_CONTEXT",
					"Missing required Discord context headers: X-Discord-User-Id and X-Discord-Server-Id",
					http.StatusForbidden,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
