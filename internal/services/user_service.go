package services

import (
	"context"
	"fmt"
	"log"

	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos/responses"
	"infinite-experiment/politburo/internal/pilots"
)

type UserService struct {
	repo              *repositories.UserRepositoryGORM
	pilotStatsService *pilots.StatsService
}

func NewUserService(repo *repositories.UserRepositoryGORM, pilotStatsService *pilots.StatsService) *UserService {
	return &UserService{
		repo:              repo,
		pilotStatsService: pilotStatsService,
	}
}

func (s *UserService) RegisterUser(ctx context.Context, discordID, ifCommunityID string, ifApiID *string, isActive bool) error {
	_, err := s.repo.InsertUser(ctx, discordID, ifCommunityID, ifApiID, isActive)
	return err
}

// GetUserDetails retrieves user details with VA affiliations and current VA status
func (s *UserService) GetUserDetails(ctx context.Context, userDiscordID, vaDiscordServerID string) (*responses.UserDetailResponse, error) {
	// Fetch user with all VA affiliations
	user, err := s.repo.GetUserWithVAAffiliations(ctx, userDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user details: %w", err)
	}

	// Build affiliations list
	affiliations := make([]responses.VAAffiliation, 0, len(user.UserVARoles))
	var currentVA *responses.CurrentVAStatus

	for _, vaRole := range user.UserVARoles {
		affiliation := responses.VAAffiliation{
			VAID:     vaRole.VAID,
			VAName:   vaRole.VA.Name,
			VACode:   vaRole.VA.Code,
			Role:     string(vaRole.Role),
			IsActive: vaRole.IsActive,
			JoinedAt: vaRole.JoinedAt,
			Callsign: vaRole.Callsign,
		}
		affiliations = append(affiliations, affiliation)

		// Check if this is the current VA (from context)
		// Debug logging
		log.Printf("[GetUserDetails] Comparing VA DiscordID '%s' with context serverID '%s'", vaRole.VA.DiscordID, vaDiscordServerID)

		if vaRole.VA.DiscordID == vaDiscordServerID {
			currentVA = &responses.CurrentVAStatus{
				IsMember: true,
				Role:     string(vaRole.Role),
				IsActive: vaRole.IsActive,
				Callsign: vaRole.Callsign,
			}
			// Note: Pilot stats (including Airtable data) are now fetched via separate /api/v1/pilot/stats endpoint
		}
	}

	// If current VA not found in affiliations, set IsMember to false
	if currentVA == nil {
		currentVA = &responses.CurrentVAStatus{
			IsMember: false,
			IsActive: false,
		}
	}

	// Build response
	response := &responses.UserDetailResponse{
		UserID:        user.ID,
		DiscordID:     user.DiscordID,
		IFCommunityID: user.IFCommunityID,
		IFApiID:       user.IFApiID,
		UserName:      user.UserName,
		IsActive:      user.IsActive,
		CreatedAt:     user.CreatedAt,
		Affiliations:  affiliations,
		CurrentVA:     currentVA,
	}

	return response, nil
}
