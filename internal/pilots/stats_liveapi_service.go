package pilots

import (
	"context"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/providers"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
)

type statsLiveAPIService struct {
	liveAPIProvider *providers.LiveAPIProvider
}

func newStatsLiveAPIService(liveAPIProvider *providers.LiveAPIProvider) *statsLiveAPIService {
	return &statsLiveAPIService{liveAPIProvider: liveAPIProvider}
}

func (s *statsLiveAPIService) Fetch(ctx context.Context, subject *platformMemberships.PilotStatsSubject) (*IFGameStats, error) {
	if subject == nil || subject.IFCommunityID == "" {
		return nil, nil
	}

	logging.Debug("Fetching IF game stats", "ifc_id", subject.IFCommunityID)
	userStatsResp, statusCode, err := s.liveAPIProvider.GetUserByIfcId(ctx, subject.IFCommunityID)
	if err != nil {
		logging.Warn("Failed to fetch user stats from Live API", "ifc_id", subject.IFCommunityID, "status_code", statusCode, "err", err)
		return nil, nil
	}
	if userStatsResp == nil || len(userStatsResp.Result) == 0 {
		return nil, nil
	}

	userStats := userStatsResp.Result[0]
	return &IFGameStats{
		FlightTime:    userStats.FlightTime * 60,
		OnlineFlights: userStats.OnlineFlights,
		LandingCount:  userStats.LandingCount,
		XP:            userStats.XP,
		Grade:         userStats.Grade,
		Violations:    userStats.Violations,
	}, nil
}
