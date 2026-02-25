package api

import (
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/auth"
	"net/http"
	"time"
)

// VerifyGodModeHandler handles GET /api/v1/admin/verify-god
// Returns whether the current user has god-mode access
func VerifyGodModeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Check if user is god-mode user using the common utility
		isGod := auth.IsGodMode(claims.DiscordUserID())

		response := map[string]interface{}{
			"is_god": isGod,
		}

		common.RespondSuccess(w, initTime, "God mode status checked", response)
	}
}
