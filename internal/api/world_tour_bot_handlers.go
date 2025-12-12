package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/models/dtos/requests"

	"github.com/go-chi/chi/v5"
)

// GetActiveTourHandler handles getting the active world tour for a VA
func (h *Handlers) GetActiveTourHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get VA ID from query parameter (for Discord bot)
		vaID := r.URL.Query().Get("va_id")
		if vaID == "" {
			http.Error(w, "va_id query parameter is required", http.StatusBadRequest)
			return
		}

		activeTour, err := h.deps.Services.WorldTour.GetActiveTour(r.Context(), vaID)
		if err != nil {
			http.Error(w, "Failed to get active tour: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Active tour retrieved successfully",
			Data:    activeTour,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GetUserProgressHandler handles getting a user's progress in a world tour
func (h *Handlers) GetUserProgressHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tourID := chi.URLParam(r, "tour_id")
		userID := chi.URLParam(r, "user_id")

		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		progress, err := h.deps.Services.WorldTour.GetUserProgress(r.Context(), tourID, userID)
		if err != nil {
			http.Error(w, "Failed to get user progress: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "User progress retrieved successfully",
			Data:    progress,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GetTourLegHandler handles getting a specific leg by number
func (h *Handlers) GetTourLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tourID := chi.URLParam(r, "tour_id")
		legNumberStr := chi.URLParam(r, "leg_number")

		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}
		if legNumberStr == "" {
			http.Error(w, "Leg number is required", http.StatusBadRequest)
			return
		}

		legNumber, err := strconv.Atoi(legNumberStr)
		if err != nil {
			http.Error(w, "Invalid leg number", http.StatusBadRequest)
			return
		}

		leg, err := h.deps.Services.WorldTour.GetLegByNumber(r.Context(), tourID, legNumber)
		if err != nil {
			http.Error(w, "Failed to get tour leg: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Tour leg retrieved successfully",
			Data:    leg,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GetTourLeaderboardHandler handles getting the leaderboard for a world tour
func (h *Handlers) GetTourLeaderboardHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tourID := chi.URLParam(r, "tour_id")
		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}

		leaderboard, err := h.deps.Services.WorldTour.GetTourLeaderboard(r.Context(), tourID)
		if err != nil {
			http.Error(w, "Failed to get tour leaderboard: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Tour leaderboard retrieved successfully",
			Data:    leaderboard,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// ValidateRouteHandler handles validating a route for world tour participation
func (h *Handlers) ValidateRouteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get claims from context for VA ID
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req requests.ValidateRouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Route == "" {
			http.Error(w, "Route is required", http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		validation, err := h.deps.Services.WorldTour.ValidateRoute(
			r.Context(),
			claims.ServerID(),
			req.Route,
			req.UserID,
		)
		if err != nil {
			http.Error(w, "Failed to validate route: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Route validation completed",
			Data:    validation,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
