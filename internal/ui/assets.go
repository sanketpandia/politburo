package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
)

// Assets contains the reusable UI foundation.
//
//go:embed templates static
var Assets embed.FS

type Renderer struct {
	templates *template.Template
}

func NewRenderer() (*Renderer, error) {
	templates, err := template.ParseFS(Assets, "templates/layouts/*.html", "templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: templates}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.templates.ExecuteTemplate(w, name, data); err != nil {
		return fmt.Errorf("render template %q: %w", name, err)
	}
	return nil
}

func (r *Renderer) RenderTo(w io.Writer, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

// StaticFS returns the embedded static file tree.
func StaticFS() (fs.FS, error) {
	return fs.Sub(Assets, "static")
}
