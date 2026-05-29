package pireps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	pirepsgen "infinite-experiment/politburo/internal/api/generated/pireps"
	"infinite-experiment/politburo/internal/pireps"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

type Handlers struct {
	GetPirepConfig       http.HandlerFunc
	SubmitPirep          http.HandlerFunc
	SaveFlightModesConfig http.HandlerFunc
}

type Server struct {
	handlers Handlers
}

var _ pirepsgen.StrictServerInterface = (*Server)(nil)

func NewServer(pirepHandler *pireps.Handler, vaHandler *platformVA.Handler) *Server {
	return &Server{handlers: Handlers{
		GetPirepConfig:       pirepHandler.GetConfig(),
		SubmitPirep:          pirepHandler.Submit(),
		SaveFlightModesConfig: vaHandler.SetFlightModesConfig(),
	}}
}

func (s *Server) SaveFlightModesConfig(ctx context.Context, request pirepsgen.SaveFlightModesConfigRequestObject) (pirepsgen.SaveFlightModesConfigResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/admin/flight-modes/config", request.Body, s.handlers.SaveFlightModesConfig)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusOK:
		response, err := decodeBody[pirepsgen.SuccessEnvelope](body)
		return pirepsgen.SaveFlightModesConfig200JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[pirepsgen.ErrorEnvelope](body)
		return pirepsgen.SaveFlightModesConfig400JSONResponse(response), err
	default:
		return nil, fmt.Errorf("save flight modes config: unexpected status code %d", statusCode)
	}
}

func (s *Server) GetPirepConfig(ctx context.Context, _ pirepsgen.GetPirepConfigRequestObject) (pirepsgen.GetPirepConfigResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodGet, "/api/v1/pireps/config", nil, s.handlers.GetPirepConfig)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusOK:
		response, err := decodeBody[pirepsgen.SuccessEnvelope](body)
		return pirepsgen.GetPirepConfig200JSONResponse(response), err
	case http.StatusNotFound:
		response, err := decodeBody[pirepsgen.ErrorEnvelope](body)
		return pirepsgen.GetPirepConfig404JSONResponse(response), err
	default:
		return nil, fmt.Errorf("get pirep config: unexpected status code %d", statusCode)
	}
}

func (s *Server) SubmitPirep(ctx context.Context, request pirepsgen.SubmitPirepRequestObject) (pirepsgen.SubmitPirepResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/pireps/submit", request.Body, s.handlers.SubmitPirep)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusOK:
		response, err := decodeBody[pirepsgen.SuccessEnvelope](body)
		return pirepsgen.SubmitPirep200JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[pirepsgen.ErrorEnvelope](body)
		return pirepsgen.SubmitPirep400JSONResponse(response), err
	default:
		return nil, fmt.Errorf("submit pirep: unexpected status code %d", statusCode)
	}
}

func (s *Server) serveJSON(ctx context.Context, method string, path string, body any, handler http.HandlerFunc) (int, []byte, error) {
	if handler == nil {
		return 0, nil, fmt.Errorf("no handler configured for %s %s", method, path)
	}

	request, err := newRequest(ctx, method, path, body)
	if err != nil {
		return 0, nil, err
	}

	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder.Code, recorder.Body.Bytes(), nil
}

func newRequest(ctx context.Context, method string, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	request := httptest.NewRequest(method, path, reader)
	request = request.WithContext(ctx)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return request, nil
}

func decodeBody[T any](payload []byte) (T, error) {
	var value T
	if len(payload) == 0 {
		return value, fmt.Errorf("empty response body")
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, fmt.Errorf("decode response body: %w", err)
	}
	return value, nil
}
