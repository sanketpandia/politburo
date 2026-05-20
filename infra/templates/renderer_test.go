package templates

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"infinite-experiment/politburo/infra/logging"
)

var initLoggerOnce sync.Once

func TestRenderTemplateCachesWhenReloadDisabled(t *testing.T) {
	renderer := newTestRenderer(t, false)
	writeTestFile(t, filepath.Join(renderer.BasePath, "pages", "dashboard.html"), `{{define "content"}}first{{end}}`)

	first := httptest.NewRecorder()
	if err := renderer.RenderTemplate(first, "pages/dashboard.html", map[string]interface{}{}); err != nil {
		t.Fatalf("first render failed: %v", err)
	}

	writeTestFile(t, filepath.Join(renderer.BasePath, "pages", "dashboard.html"), `{{define "content"}}second{{end}}`)
	second := httptest.NewRecorder()
	if err := renderer.RenderTemplate(second, "pages/dashboard.html", map[string]interface{}{}); err != nil {
		t.Fatalf("second render failed: %v", err)
	}

	if strings.Contains(second.Body.String(), "second") {
		t.Fatalf("expected cached template content, got %q", second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "first") {
		t.Fatalf("expected first cached render, got %q", second.Body.String())
	}
}

func TestRenderTemplateReloadsWhenEnabled(t *testing.T) {
	renderer := newTestRenderer(t, true)
	writeTestFile(t, filepath.Join(renderer.BasePath, "pages", "dashboard.html"), `{{define "content"}}first{{end}}`)

	first := httptest.NewRecorder()
	if err := renderer.RenderTemplate(first, "pages/dashboard.html", map[string]interface{}{}); err != nil {
		t.Fatalf("first render failed: %v", err)
	}

	writeTestFile(t, filepath.Join(renderer.BasePath, "pages", "dashboard.html"), `{{define "content"}}second{{end}}`)
	second := httptest.NewRecorder()
	if err := renderer.RenderTemplate(second, "pages/dashboard.html", map[string]interface{}{}); err != nil {
		t.Fatalf("second render failed: %v", err)
	}

	if !strings.Contains(second.Body.String(), "second") {
		t.Fatalf("expected reloaded template content, got %q", second.Body.String())
	}
}

func TestRenderPartialFallsBackToContentBlock(t *testing.T) {
	renderer := newTestRenderer(t, false)
	partialPath := filepath.Join(renderer.BasePath, "partials", "nested", "item.html")
	writeTestFile(t, partialPath, `{{define "content"}}fallback content{{end}}`)

	w := httptest.NewRecorder()
	if err := renderer.RenderPartial(w, "partials/nested/item.html", map[string]interface{}{}); err != nil {
		t.Fatalf("partial render failed: %v", err)
	}

	if !strings.Contains(w.Body.String(), "fallback content") {
		t.Fatalf("expected content fallback, got %q", w.Body.String())
	}
}

func TestRenderStandaloneUsesErrorLayout(t *testing.T) {
	renderer := newTestRenderer(t, false)
	writeTestFile(t, filepath.Join(renderer.BasePath, "pages", "unauthorized.html"), `{{define "content"}}denied{{end}}`)

	w := httptest.NewRecorder()
	if err := renderer.RenderStandalone(w, "pages/unauthorized.html", map[string]interface{}{}); err != nil {
		t.Fatalf("standalone render failed: %v", err)
	}

	if !strings.Contains(w.Body.String(), "error-shell: denied") {
		t.Fatalf("expected error layout render, got %q", w.Body.String())
	}
}

func TestRenderTemplateReturnsMissingFileError(t *testing.T) {
	renderer := newTestRenderer(t, false)
	w := httptest.NewRecorder()

	if err := renderer.RenderTemplate(w, "pages/missing.html", map[string]interface{}{}); err == nil {
		t.Fatal("expected missing file error")
	}
}

func newTestRenderer(t *testing.T, reload bool) *Renderer {
	t.Helper()
	initLoggerOnce.Do(func() {
		if err := logging.Init("test"); err != nil {
			t.Fatalf("init logger: %v", err)
		}
	})

	root := t.TempDir()
	basePath := filepath.Join(root, "templates")
	partialsPath := filepath.Join(basePath, "partials")
	layoutsPath := filepath.Join(basePath, "layouts")

	mustMkdirAll(t, filepath.Join(basePath, "pages"))
	mustMkdirAll(t, partialsPath)
	mustMkdirAll(t, layoutsPath)
	writeTestFile(t, filepath.Join(layoutsPath, "base.html"), `layout: {{template "nav" .}} {{template "content" .}}`)
	writeTestFile(t, filepath.Join(layoutsPath, "error.html"), `error-shell: {{template "content" .}}`)
	writeTestFile(t, filepath.Join(partialsPath, "nav.html"), `{{define "nav"}}nav{{end}}`)

	return NewRendererWithOptions(
		basePath,
		partialsPath,
		filepath.Join(layoutsPath, "base.html"),
		WithReloadTemplates(reload),
	)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
