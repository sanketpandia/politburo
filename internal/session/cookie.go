package session

import (
	"net/http"
	"strings"
)

const (
	CookieName   = "session_id"
	CookieMaxAge = 604800
)

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

func SetCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
	http.SetCookie(w, sessionCookie(r, sessionID, CookieMaxAge))
}

func ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, sessionCookie(r, "", -1))
}

func SessionIDFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
