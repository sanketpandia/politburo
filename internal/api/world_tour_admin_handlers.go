// DEPRECATED: This file contains legacy world tour admin handlers.
// New code should use WorldTourHandlers (world_tour_handlers.go) which consolidates
// both admin and bot endpoints with standardized response utilities.
// This file will be removed in a future release.

package api

import (
	"encoding/json"
	"net/http"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/models/dtos/requests"

	"github.com/go-chi/chi/v5"
)

// CreateWorldTourHandler handles creating a new world tour
// DEPRECATED: Use WorldTourHandlers.CreateTour() instead
func (h *Handlers) CreateWorldTourHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user claims from auth middleware
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request body
		var req requests.CreateWorldTourRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request (you can use a validation library here)
		if req.Name == "" {
			http.Error(w, "Tour name is required", http.StatusBadRequest)
			return
		}
		if req.FlightModeKey == "" {
			http.Error(w, "Flight mode key is required", http.StatusBadRequest)
			return
		}

		// Create world tour
		tour, err := h.deps.Services.WorldTour.CreateTour(
			r.Context(),
			claims.ServerID(),
			claims.UserID(),
			&req,
		)
		if err != nil {
			http.Error(w, "Failed to create world tour: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return response
		response := &dtos.APIResponse{
			Status:  "success",
			Message: "World tour created successfully",
			Data:    tour,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GetWorldToursHandler handles getting all world tours for a VA
func (h *Handlers) GetWorldToursHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tours, err := h.deps.Services.WorldTour.GetToursByVA(r.Context(), claims.ServerID())
		if err != nil {
			http.Error(w, "Failed to get world tours: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "World tours retrieved successfully",
			Data:    tours,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// UpdateWorldTourHandler handles updating an existing world tour
func (h *Handlers) UpdateWorldTourHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "id")
		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}

		var req requests.UpdateWorldTourRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		tour, err := h.deps.Services.WorldTour.UpdateTour(r.Context(), tourID, &req)
		if err != nil {
			http.Error(w, "Failed to update world tour: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "World tour updated successfully",
			Data:    tour,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// DeleteWorldTourHandler handles deleting a world tour
func (h *Handlers) DeleteWorldTourHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "id")
		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}

		err := h.deps.Services.WorldTour.DeleteTour(r.Context(), tourID)
		if err != nil {
			http.Error(w, "Failed to delete world tour: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "World tour deleted successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// AddTourLegHandler handles adding a leg to a world tour
func (h *Handlers) AddTourLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tourID := chi.URLParam(r, "tour_id")
		if tourID == "" {
			http.Error(w, "Tour ID is required", http.StatusBadRequest)
			return
		}

		var req requests.AddTourLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.Name == "" {
			http.Error(w, "Leg name is required", http.StatusBadRequest)
			return
		}
		if req.RouteName == "" {
			http.Error(w, "Route name is required", http.StatusBadRequest)
			return
		}
		if req.LegNumber <= 0 {
			http.Error(w, "Leg number must be positive", http.StatusBadRequest)
			return
		}

		leg, err := h.deps.Services.WorldTour.AddLeg(r.Context(), tourID, claims.ServerID(), &req)
		if err != nil {
			http.Error(w, "Failed to add tour leg: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Tour leg added successfully",
			Data:    leg,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// UpdateTourLegHandler handles updating an existing tour leg
func (h *Handlers) UpdateTourLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			http.Error(w, "Leg ID is required", http.StatusBadRequest)
			return
		}

		var req requests.UpdateTourLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		leg, err := h.deps.Services.WorldTour.UpdateLeg(r.Context(), legID, claims.ServerID(), &req)
		if err != nil {
			http.Error(w, "Failed to update tour leg: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Tour leg updated successfully",
			Data:    leg,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// DeleteTourLegHandler handles deleting a tour leg
func (h *Handlers) DeleteTourLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			http.Error(w, "Leg ID is required", http.StatusBadRequest)
			return
		}

		err := h.deps.Services.WorldTour.DeleteLeg(r.Context(), legID)
		if err != nil {
			http.Error(w, "Failed to delete tour leg: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := &dtos.APIResponse{
			Status:  "success",
			Message: "Tour leg deleted successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
