package auth

import (
	"context"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/db/repositories"
	"log"
	"os"
)

// MakeClaimsFromApi creates API key claims using GORM repository
func MakeClaimsFromApi(ctx context.Context, userRepo *repositories.UserRepositoryGORM, serverId string, userId string) *APIKeyClaims {

	member, err := userRepo.FindUserMembership(ctx, serverId, userId)
	if err != nil {
		// Return a minimal claims object; UUIDs stay empty
		return &APIKeyClaims{
			DiscordUIDVal:      userId,
			DiscordServerIDVal: serverId,
		}
	}

	if member == nil { // no row found
		return &APIKeyClaims{
			DiscordUIDVal:      userId,
			DiscordServerIDVal: serverId,
		}
	}

	var userUUID, vaUUID string
	var role roles.VARole
	if member.UserID != nil {
		userUUID = *member.UserID
	}
	if member.VAID != nil {
		vaUUID = *member.VAID
	}
	if member.Role != nil {
		role = *member.Role
	}

	return &APIKeyClaims{
		UserUUID:           userUUID,
		VaUUID:             vaUUID,
		RoleValue:          role,
		DiscordUIDVal:      userId,
		DiscordServerIDVal: serverId,
	}
}

// IsGodMode checks if the given Discord user ID has god-mode access
// Returns true if GOD_MODE env variable is set and matches the user ID
func IsGodMode(discordUserID string) bool {
	godModeKey := os.Getenv("GOD_MODE")
	log.Printf("GOD_MODE  key: %s | input : %s", godModeKey, discordUserID)
	return godModeKey != "" && discordUserID == godModeKey
}
