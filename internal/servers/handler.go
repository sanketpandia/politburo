package servers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

type Handler struct {
	regSvc *RegistrationService
}

func NewHandler(regSvc *RegistrationService) *Handler {
	return &Handler{
		regSvc: regSvc,
	}
}

func (h *Handler) InitServer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// 1. Extract and validate claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /server/init - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// 2. Parse request body
		var req InitServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logging.Warn("Invalid request body", "error", err)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// 3. Validate required fields
		if req.VACode == "" {
			httpdto.WriteError(w, initTime, "MISSING_FIELD", "VA code is required", http.StatusBadRequest)
			return
		}

		if req.VAName == "" {
			httpdto.WriteError(w, initTime, "MISSING_FIELD", "VA name is required", http.StatusBadRequest)
			return
		}

		if req.CallsignPrefix == "" && req.CallsignSuffix == "" {
			httpdto.WriteError(w, initTime, "INVALID_CALLSIGN_CONFIG",
				"At least one callsign pattern (prefix or suffix) is required", http.StatusBadRequest)
			return
		}

		logging.Info("Server initialization request", "discord_server_id", discordServerID, "va_code", req.VACode)

		// 4. Call service
		result, err := h.regSvc.InitServer(
			r.Context(),
			discordServerID,
			discordUserID,
			req.VACode,
			req.VAName,
			req.CallsignPrefix,
			req.CallsignSuffix,
		)

		if err != nil {
			logging.Error("Failed to initialize server", "error", err, "discord_server_id", discordServerID)
			h.handleInitServerError(w, initTime, err)
			return
		}

		logging.Info("Server initialized successfully", "discord_server_id", discordServerID, "va_id", result.VAID)
		httpdto.WriteSuccess(w, initTime, result, http.StatusCreated)
	}
}

func (h *Handler) handleInitServerError(w http.ResponseWriter, initTime time.Time, err error) {
	if errors.Is(err, ErrServerAlreadyRegistered) {
		httpdto.WriteError(w, initTime, "SERVER_ALREADY_REGISTERED",
			"This Discord server is already registered as a VA", http.StatusConflict)
		return
	}

	if errors.Is(err, ErrUserNotRegistered) {
		httpdto.WriteError(w, initTime, "USER_NOT_REGISTERED",
			"You must register as a user before initializing a server", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrInvalidCallsignConfig) {
		httpdto.WriteError(w, initTime, "INVALID_CALLSIGN_CONFIG",
			"At least one callsign pattern (prefix or suffix) is required", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrVACreationFailed) {
		httpdto.WriteError(w, initTime, "VA_CREATION_FAILED",
			"Failed to create virtual airline", http.StatusInternalServerError)
		return
	}

	// Default: internal server error
	httpdto.WriteError(w, initTime, "INTERNAL_ERROR",
		"An unexpected error occurred during server initialization", http.StatusInternalServerError)
}
