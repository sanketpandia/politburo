package ui

import (
	"embed"
	"html/template"
)

// Assets contains the reusable UI foundation without registering any pages.
//
//go:embed templates static
var Assets embed.FS

type Renderer struct {
	templates *template.Template
}

func NewRenderer() (*Renderer, error) {
	templates, err := template.ParseFS(Assets, "templates/layouts/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: templates}, nil
}
