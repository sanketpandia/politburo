package auth

import (
	"fmt"
	"net/url"
	"strings"
)

const DefaultRedirect = "/dashboard"

func NormalizeRedirect(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultRedirect, nil
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "://") {
		return "", fmt.Errorf("redirect must be a relative path")
	}
	return path, nil
}

func FormatLoginURL(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/auth/login?token=" + url.QueryEscape(token)
}
