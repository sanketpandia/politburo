package va

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"infinite-experiment/politburo/internal/models/gorm"
)

// EventService handles business logic for VA events
type EventService struct {
	eventRepo *EventRepository
}

// NewEventService creates a new VA event service
func NewEventService(eventRepo *EventRepository) *EventService {
	return &EventService{
		eventRepo: eventRepo,
	}
}

// CreateEventRequest represents the request to create a new event
type CreateEventRequest struct {
	EventName       string     `json:"event_name"`
	Description     *string    `json:"description,omitempty"`
	PredefinedRoute string     `json:"predefined_route"`
	RouteATID       *string    `json:"route_at_id,omitempty"`
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	CreatedByID     *string    `json:"created_by_id,omitempty"`
}

// UpdateEventRequest represents the request to update an event
type UpdateEventRequest struct {
	EventName       *string    `json:"event_name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	PredefinedRoute *string    `json:"predefined_route,omitempty"`
	RouteATID       *string    `json:"route_at_id,omitempty"`
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
}

// EventResponse represents an event in API responses
type EventResponse struct {
	ID              string     `json:"id"`
	EventName       string     `json:"event_name"`
	Description     *string    `json:"description,omitempty"`
	PredefinedRoute string     `json:"predefined_route"`
	RouteATID       *string    `json:"route_at_id,omitempty"`
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateEvent creates a new event with validation
func (s *EventService) CreateEvent(ctx context.Context, vaID string, req CreateEventRequest) (*gorm.VAEvent, error) {
	// Validate
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	event := &gorm.VAEvent{
		ID:              "", // GORM will generate UUID via default
		VAID:            vaID,
		EventName:       strings.TrimSpace(req.EventName),
		Description:     req.Description,
		PredefinedRoute: strings.ToUpper(strings.TrimSpace(req.PredefinedRoute)),
		RouteATID:       req.RouteATID,
		CreatedByID:     req.CreatedByID,
	}

	// Handle optional dates
	if req.StartDate != nil {
		event.StartDate = sql.NullTime{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		event.EndDate = sql.NullTime{Time: *req.EndDate, Valid: true}
	}

	if err := s.eventRepo.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return event, nil
}

// UpdateEvent updates an existing event with validation
func (s *EventService) UpdateEvent(ctx context.Context, eventID string, req UpdateEventRequest) (*gorm.VAEvent, error) {
	// Get existing event
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}

	// Update fields if provided
	if req.EventName != nil {
		event.EventName = strings.TrimSpace(*req.EventName)
	}
	if req.Description != nil {
		event.Description = req.Description
	}
	if req.PredefinedRoute != nil {
		event.PredefinedRoute = strings.ToUpper(strings.TrimSpace(*req.PredefinedRoute))
	}
	if req.RouteATID != nil {
		event.RouteATID = req.RouteATID
	}
	if req.StartDate != nil {
		event.StartDate = sql.NullTime{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		event.EndDate = sql.NullTime{Time: *req.EndDate, Valid: true}
	}

	// Validate updated event
	if err := s.validateEvent(event); err != nil {
		return nil, err
	}

	if err := s.eventRepo.Update(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return event, nil
}

// DeleteEvent deletes an event
func (s *EventService) DeleteEvent(ctx context.Context, eventID string) error {
	// Verify event exists
	exists, err := s.eventRepo.Exists(ctx, eventID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("event not found")
	}

	return s.eventRepo.Delete(ctx, eventID)
}

// GetEvent retrieves a single event by ID
func (s *EventService) GetEvent(ctx context.Context, eventID string) (*gorm.VAEvent, error) {
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}
	return event, nil
}

// ListEvents retrieves all events for a VA
func (s *EventService) ListEvents(ctx context.Context, vaID string) ([]gorm.VAEvent, error) {
	events, err := s.eventRepo.GetByVA(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	return events, nil
}

// GetActiveEvents retrieves only active events for a VA
func (s *EventService) GetActiveEvents(ctx context.Context, vaID string) ([]gorm.VAEvent, error) {
	events, err := s.eventRepo.GetActiveEvents(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active events: %w", err)
	}
	return events, nil
}

// GetEventForRoute gets the active event (if any) for a specific route in a VA
func (s *EventService) GetEventForRoute(ctx context.Context, vaID, route string) (*gorm.VAEvent, error) {
	events, err := s.eventRepo.GetEventsByRoute(ctx, vaID, route)
	if err != nil {
		return nil, fmt.Errorf("failed to get events for route: %w", err)
	}

	if len(events) == 0 {
		return nil, nil // No event for this route
	}

	// Return the first active event for this route (using CalculateIsActive logic)
	for _, event := range events {
		event.CalculateIsActive()
		if event.IsActive {
			return &event, nil
		}
	}

	return nil, nil // No active event for this route
}

// validateCreateRequest validates the create request
func (s *EventService) validateCreateRequest(req CreateEventRequest) error {
	if strings.TrimSpace(req.EventName) == "" {
		return fmt.Errorf("event_name is required")
	}

	if strings.TrimSpace(req.PredefinedRoute) == "" {
		return fmt.Errorf("predefined_route is required")
	}

	if len(req.PredefinedRoute) > 20 {
		return fmt.Errorf("predefined_route must be 20 characters or less")
	}

	// Validate route format (basic check for ICAO-ICAO pattern)
	route := strings.ToUpper(strings.TrimSpace(req.PredefinedRoute))
	if !isValidRouteFormat(route) {
		return fmt.Errorf("predefined_route must be in format ICAO-ICAO (e.g., EGLL-EHAM)")
	}

	// Both dates are optional now, but if both provided, start must be before end
	if req.StartDate != nil && req.EndDate != nil {
		if !req.StartDate.Before(*req.EndDate) {
			return fmt.Errorf("start_date must be before end_date")
		}
	}

	return nil
}

// validateEvent validates an event object
func (s *EventService) validateEvent(event *gorm.VAEvent) error {
	if strings.TrimSpace(event.EventName) == "" {
		return fmt.Errorf("event_name cannot be empty")
	}

	if strings.TrimSpace(event.PredefinedRoute) == "" {
		return fmt.Errorf("predefined_route cannot be empty")
	}

	if len(event.PredefinedRoute) > 20 {
		return fmt.Errorf("predefined_route must be 20 characters or less")
	}

	route := strings.ToUpper(strings.TrimSpace(event.PredefinedRoute))
	if !isValidRouteFormat(route) {
		return fmt.Errorf("predefined_route must be in format ICAO-ICAO (e.g., EGLL-EHAM)")
	}

	// Both dates are optional now, but if both provided, start must be before end
	if event.StartDate.Valid && event.EndDate.Valid {
		if !event.StartDate.Time.Before(event.EndDate.Time) {
			return fmt.Errorf("start_date must be before end_date")
		}
	}

	return nil
}

// isValidRouteFormat checks if a route string is in valid format (ICAO-ICAO)
func isValidRouteFormat(route string) bool {
	// Allow for event routes that might not follow strict ICAO format
	// Just check that it contains expected separators and format
	parts := strings.Split(route, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 2 || len(part) > 4 {
			return false
		}
	}

	return true
}

// ToResponse converts a VAEvent to EventResponse
func (s *EventService) ToResponse(event *gorm.VAEvent) EventResponse {
	var startDate *time.Time
	var endDate *time.Time

	if event.StartDate.Valid {
		startDate = &event.StartDate.Time
	}
	if event.EndDate.Valid {
		endDate = &event.EndDate.Time
	}

	return EventResponse{
		ID:              event.ID,
		EventName:       event.EventName,
		Description:     event.Description,
		PredefinedRoute: event.PredefinedRoute,
		RouteATID:       event.RouteATID,
		StartDate:       startDate,
		EndDate:         endDate,
		IsActive:        event.IsActive,
		CreatedAt:       event.CreatedAt,
		UpdatedAt:       event.UpdatedAt,
	}
}
