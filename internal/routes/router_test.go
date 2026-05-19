package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infinite-experiment/politburo/infra/templates"
)

func testRenderer() *templates.Renderer {
	return templates.NewRenderer(
		"templates",
		"templates/partials",
		"templates/layouts/base.html",
	)
}

func TestHandleNotFound_Status(t *testing.T) {
	renderer := testRenderer()
	handler := handleNotFound(renderer)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleNotFound_Body(t *testing.T) {
	renderer := testRenderer()
	handler := handleNotFound(renderer)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `id="error-404-page"`) {
		t.Errorf("expected error-404-page marker in body; got:\n%s", body)
	}
}

func TestHandleMethodNotAllowed_Status(t *testing.T) {
	renderer := testRenderer()
	handler := handleMethodNotAllowed(renderer)

	req := httptest.NewRequest(http.MethodPost, "/healthCheck", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleMethodNotAllowed_Body(t *testing.T) {
	renderer := testRenderer()
	handler := handleMethodNotAllowed(renderer)

	req := httptest.NewRequest(http.MethodPost, "/healthCheck", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `id="error-405-page"`) {
		t.Errorf("expected error-405-page marker in body; got:\n%s", body)
	}
}
