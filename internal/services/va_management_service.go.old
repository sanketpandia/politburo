package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/entities"
	"log"
	"time"
)

type VAManagementService struct {
	VARepo   repositories.VARepository
	UserRepo repositories.UserRepository
}

func NewVAManagementService(v repositories.VARepository, u repositories.UserRepository) *VAManagementService {
	return &VAManagementService{
		VARepo:   v,
		UserRepo: u,
	}
}

func (s *VAManagementService) SyncUser(ctx context.Context, userID string, callsign string) (string, error) {
	claims := auth.GetUserClaims(ctx)

	log.Printf("Sync request received: %s", callsign)

	// Check if user registered
	u, err := s.UserRepo.FindUserByDiscordId(ctx, userID)
	if err != nil {
		return fmt.Sprintf("User not registered: %s", err.Error()), err
	}

	tmp, err := s.UserRepo.FindUserMembership(ctx, claims.DiscordServerID(), u.ID)
	if err != nil {
		return fmt.Sprintf("Error querying database: %s", err.Error()), err
	}

	if tmp.Role != nil {
		return "User already synced", fmt.Errorf("user already synced")

	}

	// Register user. Ignore if already present
	user := entities.UserVARole{
		UserID:   u.ID,
		Role:     constants.RolePilot,
		VAID:     claims.ServerID(),
		IsActive: true,
		JoinedAt: time.Now(),
		Callsign: callsign,
	}
	log.Printf("User Object: %v", user)

	e := s.UserRepo.InsertMembership(ctx, &user)
	if e != nil {
		return fmt.Sprintf("Failed to insert: %s", e.Error()), e
	}

	return "Inserted Successfully", nil

}

func (s *VAManagementService) UpdateUserRole(ctx context.Context, userID string, newRole string) (*entities.Membership, error) {
	claims := auth.GetUserClaims(ctx)

	log.Printf("Updating for %s", userID)
	mem, err := s.UserRepo.FindUserMembership(ctx, claims.DiscordServerID(), userID)
	if err != nil || mem == nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user is not yet synced to this VA; please sync before assigning a role")
		}
		return nil, err
	}

	log.Printf("Membership: %v", *mem.UserID)

	return s.UserRepo.UpdateUserRole(ctx, claims.ServerID(), *mem.UserID, newRole)
}
