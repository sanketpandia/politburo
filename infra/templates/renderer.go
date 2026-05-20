package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	reloadTemplates bool
	mu              sync.RWMutex
	cache           map[templateCacheKey]*template.Template
}

type templateMode string

const (
	templateModePage       templateMode = "page"
	templateModePartial    templateMode = "partial"
	templateModeStandalone templateMode = "standalone"
)

type templateCacheKey struct {
	mode templateMode
	name string
}

// Option configures template renderer behavior.
type Option func(*Renderer)

// WithReloadTemplates controls whether templates are parsed on every render.
// Local development should enable this so template edits are visible without a restart.
func WithReloadTemplates(enabled bool) Option {
	return func(r *Renderer) {
		r.reloadTemplates = enabled
	}
}

// NewRenderer creates a new template renderer
func NewRenderer(basePath, partialsPath, layoutPath string) *Renderer {
	return NewRendererWithOptions(basePath, partialsPath, layoutPath)
}

// NewRendererWithOptions creates a new template renderer with optional behavior.
func NewRendererWithOptions(basePath, partialsPath, layoutPath string, opts ...Option) *Renderer {
	r := &Renderer{
		BasePath:     basePath,
		PartialsPath: partialsPath,
		LayoutPath:   layoutPath,
		cache:        make(map[templateCacheKey]*template.Template),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RenderTemplate renders a full page template with base layout
func (r *Renderer) RenderTemplate(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	t, err := r.template(templateModePage, templateName)
	if err != nil {
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
	t, err := r.template(templateModePartial, templateName)
	if err != nil {
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
	t, err := r.template(templateModeStandalone, templateName)
	if err != nil {
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

func (r *Renderer) template(mode templateMode, templateName string) (*template.Template, error) {
	key := templateCacheKey{mode: mode, name: templateName}
	if !r.reloadTemplates {
		r.mu.RLock()
		cached := r.cache[key]
		r.mu.RUnlock()
		if cached != nil {
			logging.Debug("Template cache hit", "mode", string(mode), "template", templateName)
			return cached, nil
		}
	}

	parsed, err := r.parseTemplate(mode, templateName)
	if err != nil {
		return nil, err
	}

	if !r.reloadTemplates {
		r.mu.Lock()
		if cached := r.cache[key]; cached != nil {
			r.mu.Unlock()
			return cached, nil
		}
		r.cache[key] = parsed
		r.mu.Unlock()
	}

	return parsed, nil
}

func (r *Renderer) parseTemplate(mode templateMode, templateName string) (*template.Template, error) {
	switch mode {
	case templateModePage:
		return r.parsePageTemplate(templateName)
	case templateModePartial:
		return r.parsePartialTemplate(templateName)
	case templateModeStandalone:
		return r.parseStandaloneTemplate(templateName)
	default:
		return nil, fmt.Errorf("unknown template mode %q", mode)
	}
}

func (r *Renderer) parsePageTemplate(templateName string) (*template.Template, error) {
	t := template.New("base.html").Funcs(r.getFuncMap())

	var err error
	if t, err = r.parseSharedPartials(t); err != nil {
		return nil, err
	}

	resolvedLayoutPath := resolvePath(r.LayoutPath)
	files := []string{resolvedLayoutPath}
	if templateName != "" {
		files = append(files, filepath.Join(resolvePath(r.BasePath), templateName))
	}

	if err := r.verifyFiles(files); err != nil {
		logging.Error("Template file not found", "error", err, "files", files, "original_layout", r.LayoutPath, "original_base", r.BasePath, "project_root", projectRoot)
		return nil, err
	}

	logging.Debug("Loading template files", "mode", string(templateModePage), "files", files, "layout", r.LayoutPath, "base", r.BasePath, "template", templateName)
	t, err = t.ParseFiles(files...)
	if err != nil {
		logging.Error("Failed to parse template files", "mode", string(templateModePage), "error", err, "files", files, "layout", r.LayoutPath, "base", r.BasePath)
		return nil, err
	}

	return t, nil
}

func (r *Renderer) parsePartialTemplate(templateName string) (*template.Template, error) {
	t := template.New("partial").Funcs(r.getFuncMap())

	var err error
	if t, err = r.parseSharedPartials(t); err != nil {
		return nil, err
	}

	templatePath := filepath.Join(resolvePath(r.BasePath), templateName)
	if err := r.verifyFiles([]string{templatePath}); err != nil {
		logging.Error("Template file not found", "mode", string(templateModePartial), "template", templateName, "file", templatePath, "error", err)
		return nil, err
	}

	t, err = t.ParseFiles(templatePath)
	if err != nil {
		logging.Error("Failed to parse template", "mode", string(templateModePartial), "error", err, "template", templateName)
		return nil, err
	}

	return t, nil
}

func (r *Renderer) parseStandaloneTemplate(templateName string) (*template.Template, error) {
	resolvedLayoutPath := resolvePath(r.LayoutPath)
	errorLayoutPath := strings.Replace(resolvedLayoutPath, "base.html", "error.html", 1)
	templatePath := filepath.Join(resolvePath(r.BasePath), templateName)
	files := []string{errorLayoutPath, templatePath}

	if err := r.verifyFiles(files); err != nil {
		logging.Error("Template file not found", "mode", string(templateModeStandalone), "files", files, "error", err)
		return nil, err
	}

	t := template.New(filepath.Base(errorLayoutPath)).Funcs(r.getFuncMap())
	t, err := t.ParseFiles(files...)
	if err != nil {
		logging.Error("Failed to parse template files", "mode", string(templateModeStandalone), "error", err, "files", files)
		return nil, err
	}

	return t, nil
}

func (r *Renderer) parseSharedPartials(t *template.Template) (*template.Template, error) {
	if r.PartialsPath == "" {
		return t, nil
	}

	pattern := filepath.Join(resolvePath(r.PartialsPath), "*.html")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logging.Warn("Failed to check partials directory", "error", err, "pattern", pattern)
		return t, nil
	}
	if len(matches) == 0 {
		logging.Warn("No partials found", "pattern", pattern)
		return t, nil
	}

	t, err = t.ParseGlob(pattern)
	if err != nil {
		logging.Error("Failed to load partials", "error", err, "pattern", pattern)
		return nil, err
	}

	logging.Debug("Loaded partials", "count", len(matches), "pattern", pattern)
	return t, nil
}

func (r *Renderer) verifyFiles(files []string) error {
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return err
		}
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
		"formatDuration": func(seconds interface{}) string {
			// Handle different types: int, int64, float64, *int, *int64, *float64
			var secs int
			switch v := seconds.(type) {
			case int:
				secs = v
			case int64:
				secs = int(v)
			case float64:
				secs = int(v)
			case *int:
				if v == nil {
					return ""
				}
				secs = *v
			case *int64:
				if v == nil {
					return ""
				}
				secs = int(*v)
			case *float64:
				if v == nil {
					return ""
				}
				secs = int(*v)
			default:
				return fmt.Sprintf("%v", seconds)
			}
			hours := secs / 3600
			mins := (secs % 3600) / 60
			return fmt.Sprintf("%dh %dm", hours, mins)
		},
	}
}
