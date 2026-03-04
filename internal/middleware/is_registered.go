package middleware

import (
	context "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"net/http"
	"time"
)

func IsRegisteredMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			claims := context.GetUserClaims(r.Context())

			if claims == nil {
				common.RespondError(w, time.Now(), nil, "Unauthorized: missing claims", http.StatusUnauthorized)
				return
			}

			if claims.UserID() == "" && !context.IsGodMode(r) {
				common.RespondError(w, time.Now(), nil, "You must register before accessing this resource. Please use the /register command in Discord to register your account.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
