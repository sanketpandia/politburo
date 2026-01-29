package api

import (
	"encoding/json"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
)

// UserHandlers consolidates all user-related API handlers
type UserHandlers struct {
	userRepo *repositories.UserRepositoryGORM
	vaRepo   *repositories.VAGormRepository
	userSvc  *services.UserService
	regSvc   *services.RegistrationServiceV2
}

// NewUserHandlers creates a new UserHandlers instance
func NewUserHandlers(
	userRepo *repositories.UserRepositoryGORM,
	vaRepo *repositories.VAGormRepository,
	userSvc *services.UserService,
	regSvc *services.RegistrationServiceV2,
) *UserHandlers {
	return &UserHandlers{
		userRepo: userRepo,
		vaRepo:   vaRepo,
		userSvc:  userSvc,
		regSvc:   regSvc,
	}
}

// GetDetails handles GET /api/v1/user/details
// Returns user details with VA affiliations
func (h *UserHandlers) GetDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		userDiscordID := claims.DiscordUserID()
		vaDiscordServerID := claims.DiscordServerID()

		// Call service to get user details
		userDetails, err := h.userSvc.GetUserDetails(r.Context(), userDiscordID, vaDiscordServerID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to fetch user details", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "User details fetched successfully", userDetails)
	}
}

// InitRegistrationV2 handles POST /api/v1/user/register/init using GORM and provider pattern
// Registers a new user with IFC credentials and links them to the current VA
func (h *UserHandlers) InitRegistrationV2() http.HandlerFunc {
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
		response, err := h.regSvc.InitUserRegistration(
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

// LinkToVA handles POST /api/v1/user/register/link
// Links an existing registered user to the current VA with a callsign
func (h *UserHandlers) LinkToVA() http.HandlerFunc {
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
		response, err := h.regSvc.LinkUserToVA(
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

// InitServerRegistration handles POST /api/v1/server/register/init using GORM and provider pattern
// Initializes a new VA/server registration with admin user
func (h *UserHandlers) InitServerRegistration() http.HandlerFunc {
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
		response, err := h.regSvc.InitServerRegistration(
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

// DeleteAllUsers handles DELETE /api/v1/users/delete
// Deletes all users in the database. Intended for development/testing only (god mode).
//
// @Summary      Delete all users (Test Only)
// @Description  Deletes all users in the database. Intended for development/testing only.
// @Tags         Test
// @Param        X-Discord-Id  header  string  true  "Discord ID"         default(668664447950127154)
// @Param        X-Server-Id   header  string  true  "Discord Server ID"  default(988020008665882624)
// @Param        X-API-Key     header  string  true  "API KEY"            default(API_KEY_123)
// @Produce      json
// @Success      400  {object}  dtos.APIResponse  "Always returns error; not implemented for production use"
// @Router       /api/v1/users/delete [delete]
func (h *UserHandlers) DeleteAllUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		err := h.userRepo.DeleteAllUsers(r.Context())
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to delete users", http.StatusInternalServerError)
			return
		}
		common.RespondSuccess(w, initTime, "All users deleted", nil)
	}
}

// VerifyGodMode handles GET /api/v1/admin/verify-god
// Returns whether the current user has god-mode access
func (h *UserHandlers) VerifyGodMode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Check if user is god-mode user using the common utility
		isGod := auth.IsGodMode(claims.DiscordUserID())

		response := map[string]interface{}{
			"is_god": isGod,
		}

		common.RespondSuccess(w, initTime, "God mode status checked", response)
	}
}
