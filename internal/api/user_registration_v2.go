package api

import (
	"context"
	"encoding/json"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/models/dtos"
	"net/http"
	"time"
)

// UserRegistrationService interface for dependency injection
type UserRegistrationService interface {
	InitUserRegistration(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string, callsign *string) (*dtos.InitApiResponse, error)
	LinkUserToVA(ctx context.Context, discordUserID, discordServerID, callsign string) (map[string]interface{}, error)
}

// InitUserRegistrationHandlerV2 handles POST /api/v1/user/register/init using GORM and provider pattern
func InitUserRegistrationHandlerV2(regServiceV2 UserRegistrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// Parse request body
		var req dtos.InitUserRegistrationReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.IfcId == "" {
			common.RespondError(w, initTime, nil, "IFC ID is required", http.StatusBadRequest)
			return
		}

		if req.LastFlight == "" {
			common.RespondError(w, initTime, nil, "Last flight is required", http.StatusBadRequest)
			return
		}

		// Call service to register user
		response, err := regServiceV2.InitUserRegistration(
			r.Context(),
			discordUserID,
			discordServerID,
			req.IfcId,
			req.LastFlight,
			req.Callsign,
		)

		if err != nil {
			// Return response with steps even on error
			if response != nil {
				common.RespondError(w, initTime, err, err.Error(), http.StatusBadRequest)
				return
			}
			common.RespondError(w, initTime, err, "Failed to process registration", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "User registered successfully", response)
	}
}

// LinkUserToVAHandler handles POST /api/v1/user/register/link
// Links an existing registered user to the current VA with a callsign
func LinkUserToVAHandler(regServiceV2 UserRegistrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// Parse request body
		var req dtos.LinkUserToVAReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate callsign
		if req.Callsign == "" {
			common.RespondError(w, initTime, nil, "Callsign is required", http.StatusBadRequest)
			return
		}

		// Call service to link user to VA
		response, err := regServiceV2.LinkUserToVA(
			r.Context(),
			discordUserID,
			discordServerID,
			req.Callsign,
		)

		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusBadRequest)
			return
		}

		common.RespondSuccess(w, initTime, "User linked to VA successfully", response)
	}
}
