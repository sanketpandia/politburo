package httpdto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/platform/validation"
)

func TestWriteSuccess(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteSuccess(rr, time.Now(), map[string]string{"message": "ok"}, http.StatusCreated)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var response Response[map[string]string]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || response.Result["message"] != "ok" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteError(rr, time.Now(), "NOT_FOUND", "missing", http.StatusNotFound)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	var response Response[any]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" || response.Error == nil || response.Error.Code != "NOT_FOUND" || response.Error.Message != "missing" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWriteValidationError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteValidationError(rr, time.Now(), &validation.ValidationError{Fields: []validation.FieldError{{Field: "ifc_id", Message: "ifc_id is required"}}, StatusCode: http.StatusUnprocessableEntity})

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}

	var response struct {
		Status string `json:"status"`
		Error  struct {
			Code    string                  `json:"code"`
			Message string                  `json:"message"`
			Fields  []validation.FieldError `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" || response.Error.Code != "VALIDATION_FAILED" || len(response.Error.Fields) != 1 || response.Error.Fields[0].Field != "ifc_id" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
