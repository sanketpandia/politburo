package servers

import (
	"context"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/validation"
)

type Handler struct {
	regSvc serverRegistrationHandlerService
}

type serverRegistrationHandlerService interface {
	InitServer(ctx context.Context, discordServerID string, discordUserID string, vaCode string, vaName string, callsignPrefix string, callsignSuffix string) (*InitServerResponse, *ServerError)
}

func NewHandler(regSvc serverRegistrationHandlerService) *Handler {
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

		// 2. Decode and validate request body
		var req InitServerRequest
		if decodeErr, ve := validation.DecodeAndValidate(r, &req); decodeErr != nil {
			logging.Warn("Invalid request body", "error", decodeErr)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		} else if ve != nil {
			httpdto.WriteValidationError(w, initTime, ve)
			return
		}

		// The callsign-config constraint is not expressible as a single-field validate
		// tag because it requires at least one of two fields — enforce it explicitly.
		if req.CallsignPrefix == "" && req.CallsignSuffix == "" {
			httpdto.WriteError(w, initTime, "INVALID_CALLSIGN_CONFIG",
				"At least one callsign pattern (prefix or suffix) is required", http.StatusBadRequest)
			return
		}

		logging.Info("Server initialization request", "discord_server_id", discordServerID, "va_code", req.VACode)

		// 3. Call service
		result, svcErr := h.regSvc.InitServer(
			r.Context(),
			discordServerID,
			discordUserID,
			req.VACode,
			req.VAName,
			req.CallsignPrefix,
			req.CallsignSuffix,
		)

		if svcErr != nil {
			logging.Error("Failed to initialize server", "error", svcErr, "discord_server_id", discordServerID)
			httpdto.WriteError(w, initTime, svcErr.Code, svcErr.Message, svcErr.StatusCode)
			return
		}

		logging.Info("Server initialized successfully", "discord_server_id", discordServerID, "va_id", result.VAID)
		httpdto.WriteSuccess(w, initTime, result, http.StatusCreated)
	}
}
