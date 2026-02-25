package api

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
	"net/http"
	"time"
)

// InitServerRegistrationHandlerV2 handles POST /api/v1/server/register/init using GORM and provider pattern
func InitServerRegistrationHandlerV2(regServiceV2 *services.RegistrationServiceV2) http.HandlerFunc {
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
		var req dtos.InitServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.VACode == "" {
			common.RespondError(w, initTime, nil, "VA code is required", http.StatusBadRequest)
			return
		}

		if req.VAName == "" {
			common.RespondError(w, initTime, nil, "VA name is required", http.StatusBadRequest)
			return
		}

		// Validate at least one callsign pattern is provided
		if req.CallsignPrefix == "" && req.CallsignSuffix == "" {
			common.RespondError(w, initTime, nil, "At least one of callsign prefix or suffix is required", http.StatusBadRequest)
			return
		}

		// Call service to register server
		response, err := regServiceV2.InitServerRegistration(
			r.Context(),
			discordServerID,
			discordUserID,
			req.VACode,
			req.VAName,
			req.CallsignPrefix,
			req.CallsignSuffix,
		)

		if err != nil {
			// Return response with steps even on error
			if response != nil {
				// Send error response with data included for step-by-step debugging
				common.RespondErrorWithData(w, initTime, err, err.Error(), response, http.StatusBadRequest)
				return
			}
			common.RespondError(w, initTime, err, "Failed to register server", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Server registered successfully", response)
	}
}
