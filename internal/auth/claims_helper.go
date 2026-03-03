package auth

import (
	"context"
	"infinite-experiment/politburo/internal/platform/claims"
	"infinite-experiment/politburo/internal/platform/roles"
	"log"
	"os"
)

// MakeClaimsFromApi creates API key claims using GORM repository
func MakeClaimsFromApi(ctx context.Context, claimsRepo *claims.Repository, serverId string, userId string) *APIKeyClaims {

	member, err := claimsRepo.GetMembershipByDiscordIDs(ctx, userId, serverId)
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

// IsGodModeWithKey checks if the given Discord user ID has god-mode access
// and validates the provided god-mode key header against GOD_MODE_KEY env variable
// Returns true if both GOD_MODE matches the user ID and GOD_MODE_KEY matches the provided key
func IsGodModeWithKey(discordUserID string, godModeKeyHeader string) bool {
	godModeUserID := os.Getenv("GOD_MODE")
	godModeKey := os.Getenv("GOD_MODE_KEY")
	
	log.Printf("IsGodModeWithKey: GOD_MODE user=%s, input user=%s, GOD_MODE_KEY=%s, header key=%s", 
		godModeUserID, discordUserID, godModeKey, godModeKeyHeader)
	log.Printf("IsGodModeWithKey: GOD_MODE_KEY length=%d, header length=%d, match=%v", 
		len(godModeKey), len(godModeKeyHeader), godModeKey == godModeKeyHeader)
	
	// Both must match
	userMatches := godModeUserID != "" && discordUserID == godModeUserID
	keyMatches := godModeKey != "" && godModeKeyHeader != "" && godModeKey == godModeKeyHeader
	
	log.Printf("IsGodModeWithKey: userMatches=%v, keyMatches=%v, result=%v", userMatches, keyMatches, userMatches && keyMatches)
	
	return userMatches && keyMatches
}
