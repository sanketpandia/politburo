package memberships

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/platform/validation"
)

// Handler provides HTTP handlers for membership-related endpoints
type Handler struct {
	svc         membershipsHandlerService
	pilotRepo   pilotCallsignSampler
	vaConfigSvc vaConfigReader
}

type membershipsHandlerService interface {
	GetUserStatus(ctx context.Context, userID string, vaID string) (*UserDetailResponse, error)
	JoinVA(ctx context.Context, discordUserID string, discordServerID string, callsign string) (*JoinVAResponse, *MembershipError)
}

type pilotCallsignSampler interface {
	GetSampleCallsigns(ctx context.Context, vaID string, limit int) ([]string, error)
}

type vaConfigReader interface {
	GetAllConfigValues(ctx context.Context, vaID string) (map[string]string, error)
}

// NewHandler creates a new memberships handler
func NewHandler(svc membershipsHandlerService, pilotRepo pilotCallsignSampler, vaConfigSvc vaConfigReader) *Handler {
	return &Handler{
		svc:         svc,
		pilotRepo:   pilotRepo,
		vaConfigSvc: vaConfigSvc,
	}
}

// GetUserStatus handles GET /api/v1/user/status
// Returns comprehensive user status including all VA affiliations and current VA context
func (h *Handler) GetUserStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Extract claims (populated by auth middleware)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /user/status - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		// Get user ID and VA ID from claims
		userID := claims.UserID() // Use user ID, not discord ID
		vaID := claims.ServerID() // VA ID from claims

		// If user ID is empty, user doesn't exist
		if userID == "" {
			logging.Warn("User status request with empty user ID", "va_id", vaID)
			httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
			return
		}

		logging.Info("User status request", "user_id", userID, "va_id", vaID, "endpoint", "/user/status")

		// Call service
		userStatus, err := h.svc.GetUserStatus(r.Context(), userID, vaID)
		if err != nil {
			// Check if user was not found
			if errors.Is(err, platformMemberships.ErrUserNotFound) {
				logging.Warn("User not found", "user_id", userID)
				httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
				return
			}

			logging.Error("Failed to fetch user status", "error", err, "user_id", userID)
			httpdto.WriteError(w, initTime, "FETCH_FAILED", "Failed to fetch user status", http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, userStatus, http.StatusOK)
	}
}

// JoinVA handles POST /api/v1/memberships/join
// Allows authenticated user to join a VA with a callsign
func (h *Handler) JoinVA() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// 1. Extract and validate claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /memberships/join - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		// 2. Check if user is already a member (has a role in this VA)
		if claims.Role() != "" {
			logging.Warn("User already has membership in this VA",
				"discord_user_id", claims.DiscordUserID(),
				"va_id", claims.ServerID(),
				"existing_role", claims.Role())
			httpdto.WriteError(w, initTime, "ALREADY_MEMBER", "You are already a member of this VA", http.StatusConflict)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// 3. Decode and validate request body
		var req JoinVARequest
		if decodeErr, ve := validation.DecodeAndValidate(r, &req); decodeErr != nil {
			logging.Warn("Invalid request body", "error", decodeErr)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		} else if ve != nil {
			httpdto.WriteValidationError(w, initTime, ve)
			return
		}

		logging.Info("Membership join request", "discord_user_id", discordUserID, "callsign", req.Callsign)

		// 4. Call service
		result, svcErr := h.svc.JoinVA(r.Context(), discordUserID, discordServerID, req.Callsign)

		if svcErr != nil {
			logging.Error("Failed to join VA", "error", svcErr, "discord_user_id", discordUserID)

			// CALLSIGN_NOT_IN_AIRTABLE gets an enriched message with sample callsigns
			// and a link to the Airtable base — this enrichment belongs in the handler
			// because it requires additional repo/config calls.
			if svcErr.Code == "CALLSIGN_NOT_IN_AIRTABLE" {
				vaID := claims.ServerID()
				message := svcErr.Message

				if h.pilotRepo != nil && vaID != "" {
					if samples, sampErr := h.pilotRepo.GetSampleCallsigns(r.Context(), vaID, 3); sampErr == nil && len(samples) > 0 {
						message += fmt.Sprintf(" Sample callsigns from the database: %s", strings.Join(samples, ", "))
					}
				}

				if h.vaConfigSvc != nil && vaID != "" {
					if configs, cfgErr := h.vaConfigSvc.GetAllConfigValues(r.Context(), vaID); cfgErr == nil {
						if baseID, ok := configs[platformVA.ConfigKeyAirtableVABase]; ok && baseID != "" {
							message += fmt.Sprintf(" You can find your callsign in the Airtable: https://airtable.com/%s", baseID)
						}
					}
				}

				httpdto.WriteError(w, initTime, svcErr.Code, message, svcErr.StatusCode)
				return
			}

			httpdto.WriteError(w, initTime, svcErr.Code, svcErr.Message, svcErr.StatusCode)
			return
		}

		logging.Info("User joined VA successfully", "user_id", result.UserID, "va_id", result.VAID)
		httpdto.WriteSuccess(w, initTime, result, http.StatusCreated)
	}
}
