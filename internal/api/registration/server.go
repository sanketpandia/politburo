package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	registrationgen "infinite-experiment/politburo/internal/api/generated/registration"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/memberships"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/servers"
)

type Handlers struct {
	JoinMembership     http.HandlerFunc
	RegisterPilot      http.HandlerFunc
	InitServer         http.HandlerFunc
	GenerateSignedLink http.HandlerFunc
	GetUserStatus      http.HandlerFunc
}

type Server struct {
	handlers Handlers
}

var _ registrationgen.StrictServerInterface = (*Server)(nil)

func NewServer(
	pilotsHandler *pilots.Handler,
	membershipsHandler *memberships.Handler,
	serversHandler *servers.Handler,
	authHandler *auth.Handler,
) *Server {
	return &Server{handlers: Handlers{
		JoinMembership:     membershipsHandler.JoinVA(),
		RegisterPilot:      pilotsHandler.RegisterPilot(),
		InitServer:         serversHandler.InitServer(),
		GenerateSignedLink: authHandler.GenerateSignedLink(),
		GetUserStatus:      membershipsHandler.GetUserStatus(),
	}}
}

func NewServerFromHandlers(handlers Handlers) *Server {
	return &Server{handlers: handlers}
}

func (s *Server) JoinMembership(ctx context.Context, request registrationgen.JoinMembershipRequestObject) (registrationgen.JoinMembershipResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/memberships/join", request.Body, s.handlers.JoinMembership)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusCreated:
		response, err := decodeBody[registrationgen.JoinMembershipResponse](body)
		return registrationgen.JoinMembership201JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.JoinMembership400JSONResponse(response), err
	case http.StatusUnauthorized:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.JoinMembership401JSONResponse(response), err
	case http.StatusNotFound:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.JoinMembership404JSONResponse(response), err
	case http.StatusConflict:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.JoinMembership409JSONResponse(response), err
	case http.StatusUnprocessableEntity:
		response, err := decodeBody[registrationgen.ValidationErrorResponse](body)
		return registrationgen.JoinMembership422JSONResponse(response), err
	case http.StatusInternalServerError:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.JoinMembership500JSONResponse(response), err
	default:
		return nil, fmt.Errorf("join membership: unexpected status code %d", statusCode)
	}
}

func (s *Server) RegisterPilot(ctx context.Context, request registrationgen.RegisterPilotRequestObject) (registrationgen.RegisterPilotResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/pilots/register", request.Body, s.handlers.RegisterPilot)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusCreated:
		response, err := decodeBody[registrationgen.RegisterPilotResponse](body)
		return registrationgen.RegisterPilot201JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.RegisterPilot400JSONResponse(response), err
	case http.StatusUnauthorized:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.RegisterPilot401JSONResponse(response), err
	case http.StatusConflict:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.RegisterPilot409JSONResponse(response), err
	case http.StatusUnprocessableEntity:
		response, err := decodeBody[registrationgen.ValidationErrorResponse](body)
		return registrationgen.RegisterPilot422JSONResponse(response), err
	case http.StatusInternalServerError:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.RegisterPilot500JSONResponse(response), err
	default:
		return nil, fmt.Errorf("register pilot: unexpected status code %d", statusCode)
	}
}

func (s *Server) InitServer(ctx context.Context, request registrationgen.InitServerRequestObject) (registrationgen.InitServerResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/server/init", request.Body, s.handlers.InitServer)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusCreated:
		response, err := decodeBody[registrationgen.InitServerResponse](body)
		return registrationgen.InitServer201JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.InitServer400JSONResponse(response), err
	case http.StatusUnauthorized:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.InitServer401JSONResponse(response), err
	case http.StatusConflict:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.InitServer409JSONResponse(response), err
	case http.StatusUnprocessableEntity:
		response, err := decodeBody[registrationgen.ValidationErrorResponse](body)
		return registrationgen.InitServer422JSONResponse(response), err
	case http.StatusInternalServerError:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.InitServer500JSONResponse(response), err
	default:
		return nil, fmt.Errorf("init server: unexpected status code %d", statusCode)
	}
}

func (s *Server) GenerateSignedLink(ctx context.Context, request registrationgen.GenerateSignedLinkRequestObject) (registrationgen.GenerateSignedLinkResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodPost, "/api/v1/signed-link", request.Body, s.handlers.GenerateSignedLink)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusOK:
		response, err := decodeBody[registrationgen.GenerateSignedLinkResponse](body)
		return registrationgen.GenerateSignedLink200JSONResponse(response), err
	case http.StatusBadRequest:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GenerateSignedLink400JSONResponse(response), err
	case http.StatusUnauthorized:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GenerateSignedLink401JSONResponse(response), err
	case http.StatusNotFound:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GenerateSignedLink404JSONResponse(response), err
	case http.StatusInternalServerError:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GenerateSignedLink500JSONResponse(response), err
	default:
		return nil, fmt.Errorf("generate signed link: unexpected status code %d", statusCode)
	}
}

func (s *Server) GetUserStatus(ctx context.Context, _ registrationgen.GetUserStatusRequestObject) (registrationgen.GetUserStatusResponseObject, error) {
	statusCode, body, err := s.serveJSON(ctx, http.MethodGet, "/api/v1/user/status", nil, s.handlers.GetUserStatus)
	if err != nil {
		return nil, err
	}

	switch statusCode {
	case http.StatusOK:
		response, err := decodeBody[registrationgen.UserStatusResponse](body)
		return registrationgen.GetUserStatus200JSONResponse(response), err
	case http.StatusUnauthorized:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GetUserStatus401JSONResponse(response), err
	case http.StatusNotFound:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GetUserStatus404JSONResponse(response), err
	case http.StatusInternalServerError:
		response, err := decodeBody[registrationgen.ErrorResponse](body)
		return registrationgen.GetUserStatus500JSONResponse(response), err
	default:
		return nil, fmt.Errorf("get user status: unexpected status code %d", statusCode)
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
	request.Host = "example.com"
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func decodeBody[T any](body []byte) (T, error) {
	var response T
	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("decode response: %w", err)
	}
	return response, nil
}
