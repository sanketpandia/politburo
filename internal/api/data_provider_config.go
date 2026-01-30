package api

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
	"net/http"
	"time"
)

// SaveDataProviderConfigHandler handles POST /api/v1/admin/data-provider/config
func SaveDataProviderConfigHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		userDiscordID := claims.UserID()
		vaServerID := claims.ServerID()

		// Parse request body
		var req dtos.SaveProviderConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Call service to save/update config
		response, err := deps.Services.DataProviderConfig.SaveOrUpdateConfig(r.Context(), vaServerID, &req, userDiscordID)
		if err != nil {
			handleConfigError(w, initTime, err)
			return
		}

		common.RespondSuccess(w, initTime, "Configuration saved successfully", response)
	}
}

// handleConfigError maps config service errors to appropriate HTTP responses
func handleConfigError(w http.ResponseWriter, initTime time.Time, err error) {
	// Check if it's a ConfigError with specific error code
	if configErr, ok := err.(*services.ConfigError); ok {
		statusCode := http.StatusInternalServerError

		switch configErr.Code {
		case constants.ErrCodeConfigMalformed:
			statusCode = http.StatusBadRequest
		case constants.ErrCodeNetworkError:
			statusCode = http.StatusInternalServerError
		}

		message := configErr.Message
		if configErr.Code != "" {
			message = constants.GetErrorMessage(configErr.Code)
		}

		common.RespondError(w, initTime, err, message, statusCode)
		return
	}

	// Default to internal server error for unknown errors
	common.RespondError(w, initTime, err, "An unexpected error occurred", http.StatusInternalServerError)
}
