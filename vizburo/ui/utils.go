package ui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

// dictFunction creates a map from key-value pairs for use in templates
func dictFunction(values ...interface{}) map[string]interface{} {
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values)-1; i += 2 {
		key := values[i].(string)
		dict[key] = values[i+1]
	}
	return dict
}

// jsonFunction marshals a Go object to JSON string for use in JavaScript
func jsonFunction(v interface{}) template.JS {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		// Return empty object/array on marshal error to prevent template errors
		if _, ok := v.([]interface{}); ok {
			return template.JS("[]")
		}
		return template.JS("{}")
	}
	return template.JS(jsonBytes)
}

// jsEscapeFunction escapes strings for safe use in JavaScript string literals
func jsEscapeFunction(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// RenderTemplate renders a template with the base layout
func RenderTemplate(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	// Define safe HTML function for templates
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"split": func(s string, sep string) []string {
			return strings.Split(s, sep)
		},
		"mod": func(a, b int) int {
			return a % b
		},
		"dict": dictFunction,
		"json": jsonFunction,
		"jsEscape": jsEscapeFunction,
		"default": func(defaultValue interface{}, value interface{}) interface{} {
			if value == nil || value == "" || value == false {
				return defaultValue
			}
			return value
		},
	}

	// Parse the base template and all dependencies with custom functions
	t := template.New("base.html").Funcs(funcMap)

	// Load all partials from the partials directory
	t, err := t.ParseGlob("vizburo/ui/templates/partials/*.html")
	if err != nil {
		http.Error(w, "Error loading partials: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	// Load base layout and page template
	t, err = t.ParseFiles(
		"vizburo/ui/templates/layouts/base.html",
		"vizburo/ui/templates/"+templateName,
	)
	if err != nil {
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

// RenderPartial renders just the content portion of a template (for HTMX responses)
func RenderPartial(w http.ResponseWriter, templateName string, data map[string]interface{}) error {
	// Define safe HTML function for templates
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"split": func(s string, sep string) []string {
			return strings.Split(s, sep)
		},
		"mod": func(a, b int) int {
			return a % b
		},
		"dict": dictFunction,
		"json": jsonFunction,
		"jsEscape": jsEscapeFunction,
		"default": func(defaultValue interface{}, value interface{}) interface{} {
			if value == nil || value == "" || value == false {
				return defaultValue
			}
			return value
		},
	}

	// Parse just the template file without base layout
	t := template.New("partial").Funcs(funcMap)

	// Load all partials from the partials directory
	t, err := t.ParseGlob("vizburo/ui/templates/partials/*.html")
	if err != nil {
		http.Error(w, "Error loading partials: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	// Load the specific template
	t, err = t.ParseFiles("vizburo/ui/templates/" + templateName)
	if err != nil {
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract template name from file path (e.g., "partials/datasource-config-form.html" -> "datasource-config-form")
	// For partials, try executing the template with a name derived from the filename
	parts := strings.Split(templateName, "/")
	fileName := parts[len(parts)-1]
	templateBlockName := strings.TrimSuffix(fileName, ".html")

	// Try to execute the template by its block name first, fall back to "content" if not found
	if err := t.ExecuteTemplate(w, templateBlockName, data); err != nil {
		// If the specific template name fails, try "content" as fallback
		if err := t.ExecuteTemplate(w, "content", data); err != nil {
			http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
			return err
		}
	}

	return nil
}
