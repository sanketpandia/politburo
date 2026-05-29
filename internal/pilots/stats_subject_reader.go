package pilots

import (
	"context"
	"fmt"

	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
)

type statsSubjectReader struct {
	membershipsSvc *platformMemberships.Service
}

func newStatsSubjectReader(membershipsSvc *platformMemberships.Service) *statsSubjectReader {
	return &statsSubjectReader{membershipsSvc: membershipsSvc}
}

func (r *statsSubjectReader) GetSubject(ctx context.Context, userDiscordID, vaID string) (*platformMemberships.PilotStatsSubject, error) {
	if r == nil || r.membershipsSvc == nil {
		return nil, fmt.Errorf("stats subject reader is not configured")
	}
	return r.membershipsSvc.GetPilotStatsSubject(ctx, userDiscordID, vaID)
}
