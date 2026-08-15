// Package auth holds request claims and context helpers used by middleware.
package auth

import "context"

type contextKey string

const claimsKey contextKey = "politburo.claims"

// Claims is the authenticated caller identity attached to a request.
type Claims struct {
	PbUserID      string
	Role          string
	DsUserID      string
	DsServerID    string
	PbServerID    string
	APIKeyPresent bool
}

func SetClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(Claims)
	return claims, ok
}

func (c Claims) IsMember() bool {
	return c.Role == "pilot" || c.Role == "staff" || c.Role == "admin"
}

func (c Claims) IsStaff() bool {
	return c.Role == "staff" || c.Role == "admin"
}

func (c Claims) IsAdmin() bool {
	return c.Role == "admin"
}
