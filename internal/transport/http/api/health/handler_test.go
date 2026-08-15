package health

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLiveness(t *testing.T) {
	handler := NewHandler(nil, nil, time.Now().Add(-time.Second))
	recorder := httptest.NewRecorder()
	handler.GetLiveness(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s, want ok status", recorder.Body.String())
	}
}
