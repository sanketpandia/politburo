package ui

import (
	"io/fs"
	"net/http"

	appui "infinite-experiment/politburo/internal/ui"
)

type Handler struct {
	renderer *appui.Renderer
}

func NewHandler(renderer *appui.Renderer) *Handler {
	return &Handler{renderer: renderer}
}

// Dashboard is a stub landing page for the rewrite UI surface.
func (h *Handler) Dashboard(w http.ResponseWriter, _ *http.Request) {
	if err := h.renderer.Render(w, "dashboard", map[string]any{
		"Title": "Politburo",
	}); err != nil {
		http.Error(w, "template render failed", http.StatusInternalServerError)
	}
}

// Static serves embedded CSS/JS under /static/.
func Static() http.Handler {
	sub, err := fs.Sub(appui.Assets, "static")
	if err != nil {
		panic("ui static assets missing: " + err.Error())
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}
