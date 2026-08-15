package middleware

import (
	"net/http"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/transport/http/response"
)

// SessionLookup resolves a browser session cookie into claims. Nil leaves
// UISessionAuth as a pass-through.
type SessionLookup interface {
	Lookup(r *http.Request) (auth.Claims, bool, error)
}

// UISessionAuth gates dashboard routes on a session cookie when a lookup is
// configured. Unauthenticated browsers are sent to /auth/login.
func UISessionAuth(lookup SessionLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lookup == nil {
				next.ServeHTTP(w, r)
				return
			}
			claims, ok, err := lookup.Lookup(r)
			if err != nil {
				http.Error(w, "session lookup failed", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.SetClaims(r.Context(), claims)))
		})
	}
}

// RequireMember requires claims with a membership role.
func RequireMember() func(http.Handler) http.Handler {
	return requireRole(func(c auth.Claims) bool { return c.IsMember() }, "member")
}

// RequireStaff requires staff or admin role.
func RequireStaff() func(http.Handler) http.Handler {
	return requireRole(func(c auth.Claims) bool { return c.IsStaff() }, "staff")
}

// RequireAdmin requires admin role.
func RequireAdmin() func(http.Handler) http.Handler {
	return requireRole(func(c auth.Claims) bool { return c.IsAdmin() }, "admin")
}

func requireRole(allowed func(auth.Claims) bool, label string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || !allowed(claims) {
				response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "requires "+label+" role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
