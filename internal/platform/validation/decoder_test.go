package validation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decoderTestPayload struct {
	IfcID string `json:"ifc_id" validate:"required"`
}

func TestDecodeAndValidate_MalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ifc_id":`))
	req.Header.Set("Content-Type", "application/json")

	var payload decoderTestPayload
	decodeErr, validationErr := DecodeAndValidate(req, &payload)

	if decodeErr == nil || validationErr != nil {
		t.Fatalf("expected decode error only, got decodeErr=%v validationErr=%v", decodeErr, validationErr)
	}
}

func TestDecodeAndValidate_MissingRequiredField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	var payload decoderTestPayload
	decodeErr, validationErr := DecodeAndValidate(req, &payload)

	if decodeErr != nil || validationErr == nil || len(validationErr.Fields) != 1 || validationErr.Fields[0].Field != "ifc_id" {
		t.Fatalf("unexpected validation result: decodeErr=%v validationErr=%+v", decodeErr, validationErr)
	}
}

func TestDecodeAndValidate_ValidPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ifc_id":"ifc-user"}`))
	req.Header.Set("Content-Type", "application/json")

	var payload decoderTestPayload
	decodeErr, validationErr := DecodeAndValidate(req, &payload)

	if decodeErr != nil || validationErr != nil || payload.IfcID != "ifc-user" {
		t.Fatalf("unexpected decode result: payload=%+v decodeErr=%v validationErr=%+v", payload, decodeErr, validationErr)
	}
}
