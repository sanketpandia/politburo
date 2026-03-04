package auth

import (
	"context"
	"infinite-experiment/politburo/internal/platform/claims"
	"infinite-experiment/politburo/internal/platform/roles"
	"log"
	"net/http"
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

// IsGodMode checks if the request has god-mode access
// Requires both:
//   - Discord user ID (from claims) matches GOD_MODE env variable
//   - X-God-Mode-Key header matches GOD_MODE_KEY env variable
// Returns true only if both conditions are met
func IsGodMode(r *http.Request) bool {
	claims := GetUserClaims(r.Context())
	if claims == nil {
		return false
	}

	discordUserID := claims.DiscordUserID()
	godModeKeyHeader := r.Header.Get("X-God-Mode-Key")
	
	godModeUserID := os.Getenv("GOD_MODE")
	godModeKey := os.Getenv("GOD_MODE_KEY")
	
	log.Printf("IsGodMode: GOD_MODE user=%s, input user=%s, GOD_MODE_KEY=%s, header key=%s", 
		godModeUserID, discordUserID, godModeKey, godModeKeyHeader)
	
	// Both must match
	userMatches := godModeUserID != "" && discordUserID == godModeUserID
	keyMatches := godModeKey != "" && godModeKeyHeader != "" && godModeKey == godModeKeyHeader
	
	log.Printf("IsGodMode: userMatches=%v, keyMatches=%v, result=%v", userMatches, keyMatches, userMatches && keyMatches)
	
	return userMatches && keyMatches
}

// IsGodModeWithKey is deprecated - use IsGodMode(r *http.Request) instead
// Kept for backward compatibility
func IsGodModeWithKey(discordUserID string, godModeKeyHeader string) bool {
	godModeUserID := os.Getenv("GOD_MODE")
	godModeKey := os.Getenv("GOD_MODE_KEY")
	
	log.Printf("IsGodModeWithKey: GOD_MODE user=%s, input user=%s, GOD_MODE_KEY=%s, header key=%s", 
		godModeUserID, discordUserID, godModeKey, godModeKeyHeader)
	
	// Both must match
	userMatches := godModeUserID != "" && discordUserID == godModeUserID
	keyMatches := godModeKey != "" && godModeKeyHeader != "" && godModeKey == godModeKeyHeader
	
	log.Printf("IsGodModeWithKey: userMatches=%v, keyMatches=%v, result=%v", userMatches, keyMatches, userMatches && keyMatches)
	
	return userMatches && keyMatches
}
