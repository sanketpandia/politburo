package middleware

import (
	"infinite-experiment/politburo/internal/auth"
	context "infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/platform/roles"
	"net/http"
)

func IsAdminMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			claims := context.GetUserClaims(r.Context())

			if claims.Role() != roles.RoleAdmin.String() && !auth.IsGodMode(r) {
				common.RespondPermissionDenied(w, "admin")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
