package common

import (
	"errors"

	"infinite-experiment/politburo/internal/models/dtos"
)

// ErrLiveAPIServiceNotImplemented marks the retired internal/common LiveAPIService boundary.
//
// Historical behavior removed from this type:
//   - Built ad hoc HTTP GET/POST requests to Infinite Flight Live API using IF_API_BASE_URL and IF_API_KEY.
//   - Decoded upstream responses directly into internal/models/dtos compatibility structs.
//   - Exposed sessions, session flights, flight route, flight plan, aircraft liveries, user stats,
//     user flights, user grade, ATC, ATIS, and world status methods.
//   - GetUserFlight searched every active session for a matching callsign and returned a partial
//     FlightData value for caller-side enrichment.
//
// New code must use infra/liveapi.Client as the canonical generated-client-backed boundary.
// Feature-facing adapters should live in infra/providers when they need provider-specific error
// semantics or context-shaped interfaces.
var ErrLiveAPIServiceNotImplemented = errors.New("internal/common.LiveAPIService is retired; use infra/liveapi.Client or infra/providers.LiveAPIProvider")

// LiveAPIService is a compile-time compatibility stub for legacy constructors.
// It intentionally performs no network calls.
type LiveAPIService struct{}

// NewLiveAPIService returns a retired LiveAPI compatibility stub.
func NewLiveAPIService() *LiveAPIService { return &LiveAPIService{} }

func (svc *LiveAPIService) GetUserGrade(userID string) (*dtos.UserGradeResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetUserByIfcId(ifcId string) (*dtos.UserStatsResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetSessions() (*dtos.SessionsResponse, error) {
	return nil, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetFlightRoute(flightID string, sessionId string) (*dtos.FlightRouteResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetATC() (*dtos.ATCResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetFlights(sId string) (*dtos.FlightsResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetAircraftLiveries() (*dtos.AircraftLiveriesResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetUserFlights(userID string, page int) (*dtos.UserFlightsResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetWorldStatus() (*dtos.WorldStatusResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetATIS() (*dtos.ATISResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}

func (svc *LiveAPIService) GetFlightPlan(sessionID, flightID string) (*dtos.FlightPlanResponse, int, error) {
	return nil, 0, ErrLiveAPIServiceNotImplemented
}
