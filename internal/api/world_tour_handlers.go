package api

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos/requests"
	"infinite-experiment/politburo/internal/services"

	"github.com/go-chi/chi/v5"
)

// WorldTourHandlers handles all World Tour endpoints (admin + bot)
type WorldTourHandlers struct {
	worldTourSvc *services.WorldTourService
}

// NewWorldTourHandlers creates a new WorldTourHandlers instance
func NewWorldTourHandlers(deps *Dependencies) *WorldTourHandlers {
	return &WorldTourHandlers{
		worldTourSvc: deps.Services.WorldTour,
	}
}

// ========================================
// BOT ENDPOINTS (for Discord bot integration)
// ========================================

// GetActiveTour handles getting the active world tour for a VA
func (h *WorldTourHandlers) GetActiveTour() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get VA ID from query parameter (for Discord bot)
		vaID := r.URL.Query().Get("va_id")
		if vaID == "" {
			common.RespondError(w, initTime, nil, "va_id query parameter is required", http.StatusBadRequest)
			return
		}

		activeTour, err := h.worldTourSvc.GetActiveTour(r.Context(), vaID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get active tour", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Active tour retrieved successfully", activeTour)
	}
}

// GetUserProgress handles getting a user's progress in a world tour
func (h *WorldTourHandlers) GetUserProgress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		tourID := chi.URLParam(r, "tour_id")
		userID := chi.URLParam(r, "user_id")

		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}
		if userID == "" {
			common.RespondError(w, initTime, nil, "User ID is required", http.StatusBadRequest)
			return
		}

		progress, err := h.worldTourSvc.GetUserProgress(r.Context(), tourID, userID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get user progress", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "User progress retrieved successfully", progress)
	}
}

// GetTourLeg handles getting a specific leg by number
func (h *WorldTourHandlers) GetTourLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		tourID := chi.URLParam(r, "tour_id")
		legNumberStr := chi.URLParam(r, "leg_number")

		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}
		if legNumberStr == "" {
			common.RespondError(w, initTime, nil, "Leg number is required", http.StatusBadRequest)
			return
		}

		legNumber, err := strconv.Atoi(legNumberStr)
		if err != nil {
			common.RespondError(w, initTime, err, "Invalid leg number", http.StatusBadRequest)
			return
		}

		leg, err := h.worldTourSvc.GetLegByNumber(r.Context(), tourID, legNumber)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get tour leg", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Tour leg retrieved successfully", leg)
	}
}

// GetTourLeaderboard handles getting the leaderboard for a world tour
func (h *WorldTourHandlers) GetTourLeaderboard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		tourID := chi.URLParam(r, "tour_id")
		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}

		leaderboard, err := h.worldTourSvc.GetTourLeaderboard(r.Context(), tourID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get tour leaderboard", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Tour leaderboard retrieved successfully", leaderboard)
	}
}

// ValidateRoute handles validating a route for world tour participation
func (h *WorldTourHandlers) ValidateRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context for VA ID
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req requests.ValidateRouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Route == "" {
			common.RespondError(w, initTime, nil, "Route is required", http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			common.RespondError(w, initTime, nil, "User ID is required", http.StatusBadRequest)
			return
		}

		validation, err := h.worldTourSvc.ValidateRoute(
			r.Context(),
			claims.ServerID(),
			req.Route,
			req.UserID,
		)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to validate route", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Route validation completed", validation)
	}
}

// ========================================
// ADMIN ENDPOINTS (for VA administrators)
// ========================================

// CreateTour handles creating a new world tour
func (h *WorldTourHandlers) CreateTour() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get user claims from auth middleware
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request body
		var req requests.CreateWorldTourRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.Name == "" {
			common.RespondError(w, initTime, nil, "Tour name is required", http.StatusBadRequest)
			return
		}
		if req.FlightModeKey == "" {
			common.RespondError(w, initTime, nil, "Flight mode key is required", http.StatusBadRequest)
			return
		}

		// Create world tour
		tour, err := h.worldTourSvc.CreateTour(
			r.Context(),
			claims.ServerID(),
			claims.UserID(),
			&req,
		)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to create world tour", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "World tour created successfully", tour, http.StatusCreated)
	}
}

// GetTours handles getting all world tours for a VA
func (h *WorldTourHandlers) GetTours() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tours, err := h.worldTourSvc.GetToursByVA(r.Context(), claims.ServerID())
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get world tours", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "World tours retrieved successfully", tours)
	}
}

// UpdateTour handles updating an existing world tour
func (h *WorldTourHandlers) UpdateTour() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "id")
		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}

		var req requests.UpdateWorldTourRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		tour, err := h.worldTourSvc.UpdateTour(r.Context(), tourID, &req)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to update world tour", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "World tour updated successfully", tour)
	}
}

// DeleteTour handles deleting a world tour
func (h *WorldTourHandlers) DeleteTour() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "id")
		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}

		err := h.worldTourSvc.DeleteTour(r.Context(), tourID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to delete world tour", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "World tour deleted successfully", nil)
	}
}

// AddLeg handles adding a leg to a world tour
func (h *WorldTourHandlers) AddLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "tour_id")
		if tourID == "" {
			common.RespondError(w, initTime, nil, "Tour ID is required", http.StatusBadRequest)
			return
		}

		var req requests.AddTourLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.Name == "" {
			common.RespondError(w, initTime, nil, "Leg name is required", http.StatusBadRequest)
			return
		}
		if req.RouteName == "" {
			common.RespondError(w, initTime, nil, "Route name is required", http.StatusBadRequest)
			return
		}
		if req.LegNumber <= 0 {
			common.RespondError(w, initTime, nil, "Leg number must be positive", http.StatusBadRequest)
			return
		}

		leg, err := h.worldTourSvc.AddLeg(r.Context(), tourID, claims.ServerID(), &req)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to add tour leg", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Tour leg added successfully", leg, http.StatusCreated)
	}
}

// UpdateLeg handles updating an existing tour leg
func (h *WorldTourHandlers) UpdateLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			common.RespondError(w, initTime, nil, "Leg ID is required", http.StatusBadRequest)
			return
		}

		var req requests.UpdateTourLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		leg, err := h.worldTourSvc.UpdateLeg(r.Context(), legID, claims.ServerID(), &req)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to update tour leg", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Tour leg updated successfully", leg)
	}
}

// DeleteLeg handles deleting a tour leg
func (h *WorldTourHandlers) DeleteLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			common.RespondError(w, initTime, nil, "Leg ID is required", http.StatusBadRequest)
			return
		}

		err := h.worldTourSvc.DeleteLeg(r.Context(), legID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to delete tour leg", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Tour leg deleted successfully", nil)
	}
}
