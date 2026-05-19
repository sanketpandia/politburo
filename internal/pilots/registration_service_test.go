package pilots

import (
	"context"
	"errors"
	"testing"

	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

type fakeRegistrationUsersService struct {
	getUserByIFCId func(ctx context.Context, ifcId string) (*users.User, error)
	registerUser   func(ctx context.Context, discordID string, ifCommunityID string, ifApiID *string, isActive bool) error
}

func (f *fakeRegistrationUsersService) GetUserByIFCId(ctx context.Context, ifcId string) (*users.User, error) {
	return f.getUserByIFCId(ctx, ifcId)
}

func (f *fakeRegistrationUsersService) RegisterUser(ctx context.Context, discordID string, ifCommunityID string, ifApiID *string, isActive bool) error {
	return f.registerUser(ctx, discordID, ifCommunityID, ifApiID, isActive)
}

type fakeRegistrationVAService struct {
	getByDiscordServerID func(ctx context.Context, discordServerID string) (*platformVA.VA, error)
}

func (f *fakeRegistrationVAService) GetByDiscordServerID(ctx context.Context, discordServerID string) (*platformVA.VA, error) {
	return f.getByDiscordServerID(ctx, discordServerID)
}

type fakeLiveAPIProvider struct {
	getUserByIfcId func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error)
	getUserFlights func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error)
}

func (f *fakeLiveAPIProvider) GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
	return f.getUserByIfcId(ctx, ifcId)
}

func (f *fakeLiveAPIProvider) GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
	return f.getUserFlights(ctx, userID, page)
}

func TestRegistrationService_RegisterPilot(t *testing.T) {
	const validIFAPIID = "8f3c6f74-6ecf-4f5d-91e4-d6ec315f5e95"

	tests := []struct {
		name        string
		usersSvc    *fakeRegistrationUsersService
		vaSvc       *fakeRegistrationVAService
		liveAPI     *fakeLiveAPIProvider
		wantCode    string
		wantIsVA    bool
		wantMessage string
	}{
		{
			name: "ifc already registered",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) {
					return &users.User{ID: "existing-user", DiscordID: "discord-existing"}, nil
				},
				registerUser: func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) { return nil, nil }},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) { return nil, 0, nil },
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) { return nil, 0, nil },
			},
			wantCode: "IFC_ID_ALREADY_REGISTERED",
		},
		{
			name: "ifc user not found",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) { return nil, nil },
				registerUser:   func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) { return nil, nil }},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) {
					return &dtos.UserStatsResponse{Result: []dtos.UserStats{}}, httpStatusOK, nil
				},
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) { return nil, 0, nil },
			},
			wantCode: "IFC_USER_NOT_FOUND",
		},
		{
			name: "no recent flights",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) { return nil, nil },
				registerUser:   func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) { return nil, nil }},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) {
					return &dtos.UserStatsResponse{Result: []dtos.UserStats{{UserID: validIFAPIID}}}, httpStatusOK, nil
				},
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) {
					return &dtos.UserFlightsResponse{Flights: []dtos.UserFlightEntry{}}, httpStatusOK, nil
				},
			},
			wantCode: "NO_RECENT_FLIGHTS",
		},
		{
			name: "flight mismatch",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) { return nil, nil },
				registerUser:   func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) { return nil, nil }},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) {
					return &dtos.UserStatsResponse{Result: []dtos.UserStats{{UserID: validIFAPIID}}}, httpStatusOK, nil
				},
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) {
					return &dtos.UserFlightsResponse{Flights: []dtos.UserFlightEntry{{OriginAirport: "EGLL", DestinationAirport: "LFPG"}}}, httpStatusOK, nil
				},
			},
			wantCode: "FLIGHT_MISMATCH",
		},
		{
			name: "registration success without va",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) { return nil, nil },
				registerUser:   func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) { return nil, errors.New("not found") }},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) {
					return &dtos.UserStatsResponse{Result: []dtos.UserStats{{UserID: validIFAPIID}}}, httpStatusOK, nil
				},
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) {
					return &dtos.UserFlightsResponse{Flights: []dtos.UserFlightEntry{{OriginAirport: "KJFK", DestinationAirport: "KLAX"}}}, httpStatusOK, nil
				},
			},
			wantIsVA:    false,
			wantMessage: "Pilot registered successfully",
		},
		{
			name: "registration success with va already registered",
			usersSvc: &fakeRegistrationUsersService{
				getUserByIFCId: func(context.Context, string) (*users.User, error) { return nil, nil },
				registerUser:   func(context.Context, string, string, *string, bool) error { return nil },
			},
			vaSvc: &fakeRegistrationVAService{getByDiscordServerID: func(context.Context, string) (*platformVA.VA, error) {
				return &platformVA.VA{ID: "ee9d3818-aa8f-4a3c-8522-04fd3c891f99", Code: "IFE", Name: "Infinite"}, nil
			}},
			liveAPI: &fakeLiveAPIProvider{
				getUserByIfcId: func(context.Context, string) (*dtos.UserStatsResponse, int, error) {
					return &dtos.UserStatsResponse{Result: []dtos.UserStats{{UserID: validIFAPIID}}}, httpStatusOK, nil
				},
				getUserFlights: func(context.Context, string, int) (*dtos.UserFlightsResponse, int, error) {
					return &dtos.UserFlightsResponse{Flights: []dtos.UserFlightEntry{{OriginAirport: "KJFK", DestinationAirport: "KLAX"}}}, httpStatusOK, nil
				},
			},
			wantIsVA:    true,
			wantMessage: "Pilot registered successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewRegistrationService(tt.usersSvc, tt.vaSvc, tt.liveAPI)
			response, regErr := svc.RegisterPilot(context.Background(), "discord-user", "discord-server", "ifc-user", "KJFK-KLAX")

			if tt.wantCode != "" {
				if regErr == nil || regErr.Code != tt.wantCode {
					t.Fatalf("expected error code %q, got %+v", tt.wantCode, regErr)
				}
				return
			}

			if regErr != nil {
				t.Fatalf("unexpected error: %+v", regErr)
			}
			if response == nil || response.Message != tt.wantMessage || response.IsVARegistered != tt.wantIsVA || !response.Success {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

const httpStatusOK = 200
