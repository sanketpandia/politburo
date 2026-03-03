package middleware

import (
	"net/http"
	"path/filepath"
	"strings"
)

// MimeTypeMiddleware wraps a file server and sets correct MIME types for various file types
// This is especially important for ES modules (.mjs files) which need application/javascript
func MimeTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the file extension
		ext := filepath.Ext(r.URL.Path)

		// Set correct MIME type for .mjs files (ES modules)
		// Browsers require this to be application/javascript for ES modules to work
		if strings.EqualFold(ext, ".mjs") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}

		// Call the wrapped handler
		next.ServeHTTP(w, r)
	})
}
