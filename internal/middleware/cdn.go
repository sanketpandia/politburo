package middleware

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// CDNMiddleware sets Cloudflare CDN-friendly cache headers for static assets
// This middleware should be applied to static file routes to enable CDN caching
func CDNMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Cloudflare CDN cache headers
		// Cache for 1 year (31536000 seconds) - static assets should be versioned/hashed
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		
		// Cloudflare-specific headers
		w.Header().Set("CF-Cache-Status", "HIT")
		
		// Set proper MIME types based on file extension
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		switch ext {
		case ".js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case ".mjs":
			// ES modules
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".woff", ".woff2":
			w.Header().Set("Content-Type", "font/woff2")
		case ".ttf":
			w.Header().Set("Content-Type", "font/ttf")
		case ".eot":
			w.Header().Set("Content-Type", "application/vnd.ms-fontobject")
		case ".otf":
			w.Header().Set("Content-Type", "font/otf")
		}

		// Set ETag for cache validation (optional, but good practice)
		// Using a simple ETag based on file path and modification time
		// In production, you might want to use file hash or version
		etag := `"` + strings.ReplaceAll(r.URL.Path, "/", "_") + `"`
		w.Header().Set("ETag", etag)

		// Check if client has cached version
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Set last modified (using current time as approximation)
		// In production, use actual file modification time
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

		// Call next handler
		next.ServeHTTP(w, r)
	})
}
