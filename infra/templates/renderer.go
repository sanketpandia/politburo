package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
)

var projectRoot string

func init() {
	// Allow override for containers (e.g. TEMPLATES_ROOT=/app)
	if root := os.Getenv("TEMPLATES_ROOT"); root != "" {
		if _, err := os.Stat(filepath.Join(root, "templates", "layouts", "base.html")); err == nil {
			projectRoot = root
			return
		}
	}

	// Container fallback: app often runs with WORKDIR /app and templates at /app/templates
	if _, err := os.Stat("/app/templates/layouts/base.html"); err == nil {
		projectRoot = "/app"
		return
	}

	// Find project root by looking for go.mod file (local dev)
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	projectRoot = wd
}

// resolvePath resolves a relative path to an absolute path relative to project root
func resolvePath(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(projectRoot, relPath)
}

// Renderer handles template rendering with configurable paths
type Renderer struct {
	BasePath     string // e.g., "vizburo/ui/templates"
	PartialsPath string // e.g., "vizburo/ui/templates/partials"
	LayoutPath   string // e.g., "vizburo/ui/templates/layouts/base.html"
}

// NewRenderer creates a new template renderer
func NewRenderer(basePath, partialsPath, layoutPath string) *Renderer {
	return &Renderer{
		BasePath:     basePath,
		PartialsPath: partialsPath,
		LayoutPath:   layoutPath,
	}
}

// RenderTemplate renders a full page template with base layout
func (r *Renderer) RenderTemplate(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	funcMap := r.getFuncMap()

	// Parse templates with custom functions
	t := template.New("base.html").Funcs(funcMap)

	// Load all partials (only if files exist)
	if r.PartialsPath != "" {
		// Resolve path relative to project root
		resolvedPartialsPath := resolvePath(r.PartialsPath)
		// Check if any files match the pattern first
		// Use filepath.Join to properly construct the glob pattern
		pattern := filepath.Join(resolvedPartialsPath, "*.html")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			logging.Warn("Failed to check partials directory", "error", err, "pattern", pattern)
		} else if len(matches) > 0 {
			// Only parse if files exist
			t, err = t.ParseGlob(pattern)
			if err != nil {
				logging.Error("Failed to load partials", "error", err, "pattern", pattern)
				http.Error(w, "Error loading partials: "+err.Error(), http.StatusInternalServerError)
				return err
			}
			logging.Debug("Loaded partials", "count", len(matches), "pattern", pattern)
		} else {
			logging.Warn("No partials found", "pattern", pattern)
		}
	}

	// Load base layout and page template
	// Resolve paths relative to project root
	resolvedLayoutPath := resolvePath(r.LayoutPath)
	files := []string{resolvedLayoutPath}
	if templateName != "" {
		resolvedBasePath := resolvePath(r.BasePath)
		files = append(files, filepath.Join(resolvedBasePath, templateName))
	}

	// Verify files exist before parsing (for better error messages)
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			// Log both original and resolved paths for debugging
			logging.Error("Template file not found", "file", file, "original_layout", r.LayoutPath, "original_base", r.BasePath, "project_root", projectRoot, "error", err)
			http.Error(w, "Template file not found: "+file, http.StatusInternalServerError)
			return err
		}
	}

	// Log the files being loaded for debugging
	logging.Debug("Loading template files", "files", files, "layout", r.LayoutPath, "base", r.BasePath, "template", templateName)

	t, err := t.ParseFiles(files...)
	if err != nil {
		logging.Error("Failed to parse template files", "error", err, "files", files, "layout", r.LayoutPath, "base", r.BasePath)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		logging.Error("Failed to execute template", "error", err)
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

// RenderPartial renders just the content portion (for HTMX responses)
func (r *Renderer) RenderPartial(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	funcMap := r.getFuncMap()

	// Parse just the template file without base layout
	t := template.New("partial").Funcs(funcMap)

	// Load all partials (only if files exist)
	if r.PartialsPath != "" {
		// Resolve path relative to project root
		resolvedPartialsPath := resolvePath(r.PartialsPath)
		// Check if any files match the pattern first
		// Use filepath.Join to properly construct the glob pattern
		pattern := filepath.Join(resolvedPartialsPath, "*.html")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			logging.Warn("Failed to check partials directory", "error", err, "pattern", pattern)
		} else if len(matches) > 0 {
			// Only parse if files exist
			t, err = t.ParseGlob(pattern)
			if err != nil {
				logging.Error("Failed to load partials", "error", err, "pattern", pattern)
				http.Error(w, "Error loading partials: "+err.Error(), http.StatusInternalServerError)
				return err
			}
			logging.Debug("Loaded partials", "count", len(matches), "pattern", pattern)
		} else {
			logging.Warn("No partials found", "pattern", pattern)
		}
	}

	// Load the specific template
	// Resolve path relative to project root
	resolvedBasePath := resolvePath(r.BasePath)
	t, err := t.ParseFiles(filepath.Join(resolvedBasePath, templateName))
	if err != nil {
		logging.Error("Failed to parse template", "error", err, "template", templateName)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract template name from file path
	parts := strings.Split(templateName, "/")
	fileName := parts[len(parts)-1]
	templateBlockName := strings.TrimSuffix(fileName, ".html")

	// Try to execute the template by its block name first, fall back to "content"
	if err := t.ExecuteTemplate(w, templateBlockName, data); err != nil {
		if err := t.ExecuteTemplate(w, "content", data); err != nil {
			logging.Error("Failed to execute template", "error", err, "template", templateName)
			http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
			return err
		}
	}

	return nil
}

// RenderStandalone renders a standalone template with the error base layout
// Useful for error pages that don't need the full app shell or session data
func (r *Renderer) RenderStandalone(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	funcMap := r.getFuncMap()

	// Load error base layout
	resolvedLayoutPath := resolvePath(r.LayoutPath)
	// Replace base.html with error.html for error pages
	errorLayoutPath := strings.Replace(resolvedLayoutPath, "base.html", "error.html", 1)
	
	// Load page template
	resolvedBasePath := resolvePath(r.BasePath)
	templatePath := filepath.Join(resolvedBasePath, templateName)
	
	// Verify files exist
	files := []string{errorLayoutPath, templatePath}
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			logging.Error("Template file not found", "file", file, "error", err)
			http.Error(w, "Template file not found: "+file, http.StatusInternalServerError)
			return err
		}
	}

	// Parse templates with custom functions
	// Use the layout file's base name as the template name (like RenderTemplate does with base.html)
	layoutBaseName := filepath.Base(errorLayoutPath)
	t := template.New(layoutBaseName).Funcs(funcMap)

	// Parse both layout and page template together
	t, err := t.ParseFiles(files...)
	if err != nil {
		logging.Error("Failed to parse template files", "error", err, "files", files)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	// Set content type only if headers haven't been written yet
	// (caller may have already set headers and status code)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	// Execute the layout template (like RenderTemplate does with base.html)
	// The layout will pull in blocks from the page template automatically
	if err := t.Execute(w, data); err != nil {
		logging.Error("Failed to execute template", "error", err)
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

// getFuncMap returns the standard template function map
func (r *Renderer) getFuncMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"split": func(s string, sep string) []string {
			return strings.Split(s, sep)
		},
		"mod": func(a, b int) int {
			return a % b
		},
		"dict":     DictFunction,
		"json":     JSONFunction,
		"jsEscape": JSEscapeFunction,
		"default": func(defaultValue interface{}, value interface{}) interface{} {
			if value == nil || value == "" || value == false {
				return defaultValue
			}
			return value
		},
		"formatTime": func(timeStr string) string {
			if timeStr == "" {
				return ""
			}
			t, err := time.Parse(time.RFC3339, timeStr)
			if err != nil {
				return timeStr
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"formatFlightTime": func(seconds *float64) string {
			if seconds == nil {
				return ""
			}
			hours := int(*seconds / 3600)
			minutes := int((*seconds - float64(hours*3600)) / 60)
			return fmt.Sprintf("%d:%02d", hours, minutes)
		},
	}
}
