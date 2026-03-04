package middleware

import (
	context "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"net/http"
)

func IsMemberMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			claims := context.GetUserClaims(r.Context())

			// Check permissions BEFORE calling next handler
			if claims.Role() == "" && !context.IsGodMode(r) {
				common.RespondPermissionDenied(w, "member (pilot)")
				return
			}

			// Only call next handler ONCE
			next.ServeHTTP(w, r)
		})
	}
}
