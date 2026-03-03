package events

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Service handles business logic for events
type Service struct {
	repo *Repository
}

// NewService creates a new event service
func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// Request/Response DTOs

// CreateEventRequest represents the request to create a new event
type CreateEventRequest struct {
	Name        string                  `json:"name"`
	Description *string                 `json:"description,omitempty"`
	Status      string                  `json:"status,omitempty"`
	FlightMode  *string                 `json:"flight_mode,omitempty"`
	StartDate   *time.Time              `json:"start_date,omitempty"`
	EndDate     *time.Time              `json:"end_date,omitempty"`
	Legs        []CreateEventLegRequest `json:"legs,omitempty"`
}

// UpdateEventRequest represents the request to update an event
type UpdateEventRequest struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Status      *string    `json:"status,omitempty"`
	FlightMode  *string    `json:"flight_mode,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
}

// CreateEventLegRequest represents the request to create a new leg
type CreateEventLegRequest struct {
	Origin       string  `json:"origin"`
	Destination  string  `json:"destination"`
	RouteATID    *string `json:"route_at_id,omitempty"`
	Description  *string `json:"description,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
}

// UpdateEventLegRequest represents the request to update a leg
type UpdateEventLegRequest struct {
	Origin       *string `json:"origin,omitempty"`
	Destination  *string `json:"destination,omitempty"`
	RouteATID    *string `json:"route_at_id,omitempty"`
	Description  *string `json:"description,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
}

// EventResponse represents an event in API responses
type EventResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	Status      string             `json:"status"`
	FlightMode  *string            `json:"flight_mode,omitempty"`
	StartDate   *time.Time         `json:"start_date,omitempty"`
	EndDate     *time.Time         `json:"end_date,omitempty"`
	IsActive    bool               `json:"is_active"`
	Legs        []EventLegResponse `json:"legs,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	CreatedByID *string            `json:"created_by_id,omitempty"`
	UpdatedByID *string            `json:"updated_by_id,omitempty"`
}

// EventSummaryResponse represents a summary of an event
type EventSummaryResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	LegCount  int        `json:"leg_count"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// EventLegResponse represents a leg in API responses
type EventLegResponse struct {
	ID             string                 `json:"id"`
	EventID        string                 `json:"event_id"`
	LegNumber      int                    `json:"leg_number"`
	Origin         string                 `json:"origin"`
	Destination    string                 `json:"destination"`
	RouteATID      *string                `json:"route_at_id,omitempty"`
	Description    *string                `json:"description,omitempty"`
	ThumbnailURL   *string                `json:"thumbnail_url,omitempty"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	CreatedByID    *string                `json:"created_by_id,omitempty"`
	UpdatedByID    *string                `json:"updated_by_id,omitempty"`
}

// Service methods

// CreateEvent creates a new event with validation
func (s *Service) CreateEvent(ctx context.Context, vaID, createdByID string, req CreateEventRequest) (*Event, error) {
	// Validate
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Set default status if not provided
	status := req.Status
	if status == "" {
		status = "draft"
	}

	event := &Event{
		VAID:        vaID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Status:      status,
		FlightMode:  req.FlightMode,
		CreatedByID: &createdByID,
	}

	// Handle optional dates
	if req.StartDate != nil {
		event.StartDate = sql.NullTime{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		event.EndDate = sql.NullTime{Time: *req.EndDate, Valid: true}
	}

	// Create legs if provided
	if len(req.Legs) > 0 {
		event.Legs = make([]EventLeg, 0, len(req.Legs))
		for i, legReq := range req.Legs {
			leg := EventLeg{
				LegNumber:    i + 1, // Auto-assign leg numbers starting from 1
				Origin:       strings.ToUpper(strings.TrimSpace(legReq.Origin)),
				Destination:  strings.ToUpper(strings.TrimSpace(legReq.Destination)),
				RouteATID:    legReq.RouteATID,
				Description:  legReq.Description,
				ThumbnailURL: legReq.ThumbnailURL,
				CreatedByID:  &createdByID,
			}
			event.Legs = append(event.Legs, leg)
		}
	}

	// Validate: Only one active multi-leg event at a time
	if status == "active" && len(req.Legs) > 1 {
		existing, err := s.GetActiveMultiLegEvent(ctx, vaID)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing multi-leg event: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("cannot activate multi-leg event: another active multi-leg event exists (ID: %s)", existing.ID)
		}
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Reload to get all fields populated
	return s.repo.GetByID(ctx, event.ID)
}

// UpdateEvent updates an existing event with validation
func (s *Service) UpdateEvent(ctx context.Context, eventID, updatedByID string, req UpdateEventRequest) (*Event, error) {
	// Get existing event
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}

	// Update fields if provided
	if req.Name != nil {
		event.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		event.Description = req.Description
	}
	if req.Status != nil {
		event.Status = *req.Status
	}
	if req.FlightMode != nil {
		event.FlightMode = req.FlightMode
	}
	if req.StartDate != nil {
		event.StartDate = sql.NullTime{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		event.EndDate = sql.NullTime{Time: *req.EndDate, Valid: true}
	}

	event.UpdatedByID = &updatedByID

	// Validate updated event
	if err := s.validateEvent(event); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return s.repo.GetByID(ctx, eventID)
}

// DeleteEvent deletes an event
func (s *Service) DeleteEvent(ctx context.Context, eventID string) error {
	// Verify event exists
	exists, err := s.repo.Exists(ctx, eventID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("event not found")
	}

	return s.repo.Delete(ctx, eventID)
}

// GetEvent retrieves a single event by ID
func (s *Service) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}
	return event, nil
}

// GetEventSummary retrieves a summary of an event
func (s *Service) GetEventSummary(ctx context.Context, eventID string) (*EventSummaryResponse, error) {
	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}

	var startDate *time.Time
	var endDate *time.Time
	if event.StartDate.Valid {
		startDate = &event.StartDate.Time
	}
	if event.EndDate.Valid {
		endDate = &event.EndDate.Time
	}

	return &EventSummaryResponse{
		ID:        event.ID,
		Name:      event.Name,
		Status:    event.Status,
		LegCount:  len(event.Legs),
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  event.IsActive(),
		CreatedAt: event.CreatedAt,
		UpdatedAt: event.UpdatedAt,
	}, nil
}

// ListEvents retrieves all events for a VA with optional filters
func (s *Service) ListEvents(ctx context.Context, vaID string, status *string, activeOnly *bool) ([]Event, error) {
	if status != nil {
		return s.repo.GetByStatus(ctx, vaID, *status)
	}
	if activeOnly != nil && *activeOnly {
		return s.repo.GetActiveByVA(ctx, vaID)
	}
	return s.repo.GetByVA(ctx, vaID)
}

// GetActiveEvents retrieves only active events for a VA
func (s *Service) GetActiveEvents(ctx context.Context, vaID string) ([]Event, error) {
	return s.repo.GetActiveByVA(ctx, vaID)
}

// GetActiveMultiLegEvent retrieves the active multi-leg event for a VA (if any)
// Returns the active event with more than 1 leg, or nil if none exists
func (s *Service) GetActiveMultiLegEvent(ctx context.Context, vaID string) (*Event, error) {
	events, err := s.repo.GetActiveByVA(ctx, vaID)
	if err != nil {
		return nil, err
	}

	// Find event with more than 1 leg
	for _, event := range events {
		if len(event.Legs) > 1 {
			return &event, nil
		}
	}

	return nil, nil // No active multi-leg event
}

// UpdateEventStatus updates the status of an event
func (s *Service) UpdateEventStatus(ctx context.Context, eventID, updatedByID, status string) (*Event, error) {
	// Validate status
	if status != "draft" && status != "active" && status != "completed" && status != "cancelled" {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	event, err := s.repo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found: %w", err)
	}

	// Validate: Only one active multi-leg event at a time
	if status == "active" && len(event.Legs) > 1 {
		existing, err := s.GetActiveMultiLegEvent(ctx, event.VAID)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing multi-leg event: %w", err)
		}
		if existing != nil && existing.ID != eventID {
			return nil, fmt.Errorf("cannot activate multi-leg event: another active multi-leg event exists (ID: %s)", existing.ID)
		}
	}

	event.Status = status
	event.UpdatedByID = &updatedByID

	if err := s.repo.Update(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to update event status: %w", err)
	}

	return s.repo.GetByID(ctx, eventID)
}

// CreateEventLeg creates a new leg for an event
func (s *Service) CreateEventLeg(ctx context.Context, eventID, createdByID string, req CreateEventLegRequest) (*EventLeg, error) {
	// Validate
	if err := s.validateCreateLegRequest(req); err != nil {
		return nil, err
	}

	// Get next leg number
	legNumber, err := s.repo.GetNextLegNumber(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next leg number: %w", err)
	}

	leg := &EventLeg{
		EventID:      eventID,
		LegNumber:    legNumber,
		Origin:       strings.ToUpper(strings.TrimSpace(req.Origin)),
		Destination:  strings.ToUpper(strings.TrimSpace(req.Destination)),
		RouteATID:    req.RouteATID,
		Description:  req.Description,
		ThumbnailURL: req.ThumbnailURL,
		CreatedByID:  &createdByID,
	}

	if err := s.repo.CreateLeg(ctx, leg); err != nil {
		return nil, fmt.Errorf("failed to create leg: %w", err)
	}

	return s.repo.GetLegByID(ctx, leg.ID)
}

// UpdateEventLeg updates an existing leg
func (s *Service) UpdateEventLeg(ctx context.Context, legID, updatedByID string, req UpdateEventLegRequest) (*EventLeg, error) {
	leg, err := s.repo.GetLegByID(ctx, legID)
	if err != nil {
		return nil, fmt.Errorf("leg not found: %w", err)
	}

	// Update fields if provided
	if req.Origin != nil {
		leg.Origin = strings.ToUpper(strings.TrimSpace(*req.Origin))
	}
	if req.Destination != nil {
		leg.Destination = strings.ToUpper(strings.TrimSpace(*req.Destination))
	}
	if req.RouteATID != nil {
		leg.RouteATID = req.RouteATID
	}
	if req.Description != nil {
		leg.Description = req.Description
	}
	if req.ThumbnailURL != nil {
		leg.ThumbnailURL = req.ThumbnailURL
	}

	leg.UpdatedByID = &updatedByID

	// Validate updated leg
	if err := s.validateLeg(leg); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateLeg(ctx, leg); err != nil {
		return nil, fmt.Errorf("failed to update leg: %w", err)
	}

	return s.repo.GetLegByID(ctx, legID)
}

// UpdateEventLegAdditionalData updates the additional_data field of an event leg
func (s *Service) UpdateEventLegAdditionalData(ctx context.Context, legID, updatedByID string, additionalData AdditionalData) (*EventLeg, error) {
	leg, err := s.repo.GetLegByID(ctx, legID)
	if err != nil {
		return nil, fmt.Errorf("leg not found: %w", err)
	}

	// Initialize AdditionalData if nil
	if leg.AdditionalData == nil {
		leg.AdditionalData = AdditionalData{}
	}

	// Merge the provided data into existing data
	if len(additionalData) > 0 {
		for k, v := range additionalData {
			leg.AdditionalData[k] = v
		}
	}

	leg.UpdatedByID = &updatedByID

	if err := s.repo.UpdateLeg(ctx, leg); err != nil {
		return nil, fmt.Errorf("failed to update leg additional data: %w", err)
	}

	return s.repo.GetLegByID(ctx, legID)
}

// DeleteEventLeg deletes a leg
func (s *Service) DeleteEventLeg(ctx context.Context, legID string) error {
	_, err := s.repo.GetLegByID(ctx, legID)
	if err != nil {
		return fmt.Errorf("leg not found: %w", err)
	}

	return s.repo.DeleteLeg(ctx, legID)
}

// GetEventLegs retrieves all legs for an event
func (s *Service) GetEventLegs(ctx context.Context, eventID string) ([]EventLeg, error) {
	return s.repo.GetLegsByEvent(ctx, eventID)
}

// GetEventLeg retrieves a single leg by ID
func (s *Service) GetEventLeg(ctx context.Context, legID string) (*EventLeg, error) {
	leg, err := s.repo.GetLegByID(ctx, legID)
	if err != nil {
		return nil, fmt.Errorf("leg not found: %w", err)
	}
	return leg, nil
}

// Validation methods

// validateCreateRequest validates the create request
func (s *Service) validateCreateRequest(req CreateEventRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if len(strings.TrimSpace(req.Name)) > 255 {
		return fmt.Errorf("name must be 255 characters or less")
	}

	if req.Status != "" && req.Status != "draft" && req.Status != "active" && req.Status != "completed" && req.Status != "cancelled" {
		return fmt.Errorf("invalid status: %s", req.Status)
	}

	// Both dates are optional, but if both provided, start must be before end
	if req.StartDate != nil && req.EndDate != nil {
		if !req.StartDate.Before(*req.EndDate) {
			return fmt.Errorf("start_date must be before end_date")
		}
	}

	// Validate legs if provided
	for i, leg := range req.Legs {
		if err := s.validateCreateLegRequest(leg); err != nil {
			return fmt.Errorf("leg %d: %w", i+1, err)
		}
	}

	return nil
}

// validateEvent validates an event object
func (s *Service) validateEvent(event *Event) error {
	if strings.TrimSpace(event.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(strings.TrimSpace(event.Name)) > 255 {
		return fmt.Errorf("name must be 255 characters or less")
	}

	if event.Status != "draft" && event.Status != "active" && event.Status != "completed" && event.Status != "cancelled" {
		return fmt.Errorf("invalid status: %s", event.Status)
	}

	// Both dates are optional, but if both provided, start must be before end
	if event.StartDate.Valid && event.EndDate.Valid {
		if !event.StartDate.Time.Before(event.EndDate.Time) {
			return fmt.Errorf("start_date must be before end_date")
		}
	}

	return nil
}

// validateCreateLegRequest validates the create leg request
func (s *Service) validateCreateLegRequest(req CreateEventLegRequest) error {
	origin := strings.ToUpper(strings.TrimSpace(req.Origin))
	destination := strings.ToUpper(strings.TrimSpace(req.Destination))

	if origin == "" {
		return fmt.Errorf("origin is required")
	}

	if destination == "" {
		return fmt.Errorf("destination is required")
	}

	if len(origin) < 2 || len(origin) > 4 {
		return fmt.Errorf("origin must be 2-4 characters (ICAO code)")
	}

	if len(destination) < 2 || len(destination) > 4 {
		return fmt.Errorf("destination must be 2-4 characters (ICAO code)")
	}

	return nil
}

// validateLeg validates a leg object
func (s *Service) validateLeg(leg *EventLeg) error {
	origin := strings.ToUpper(strings.TrimSpace(leg.Origin))
	destination := strings.ToUpper(strings.TrimSpace(leg.Destination))

	if origin == "" {
		return fmt.Errorf("origin cannot be empty")
	}

	if destination == "" {
		return fmt.Errorf("destination cannot be empty")
	}

	if len(origin) < 2 || len(origin) > 4 {
		return fmt.Errorf("origin must be 2-4 characters (ICAO code)")
	}

	if len(destination) < 2 || len(destination) > 4 {
		return fmt.Errorf("destination must be 2-4 characters (ICAO code)")
	}

	return nil
}

// ToResponse converts an Event to EventResponse
func (s *Service) ToResponse(event *Event) EventResponse {
	var startDate *time.Time
	var endDate *time.Time

	if event.StartDate.Valid {
		startDate = &event.StartDate.Time
	}
	if event.EndDate.Valid {
		endDate = &event.EndDate.Time
	}

	legs := make([]EventLegResponse, 0, len(event.Legs))
	for _, leg := range event.Legs {
		legs = append(legs, s.ToLegResponse(&leg))
	}

	return EventResponse{
		ID:          event.ID,
		Name:        event.Name,
		Description: event.Description,
		Status:      event.Status,
		FlightMode:  event.FlightMode,
		StartDate:   startDate,
		EndDate:     endDate,
		IsActive:    event.IsActive(),
		Legs:        legs,
		CreatedAt:   event.CreatedAt,
		UpdatedAt:   event.UpdatedAt,
		CreatedByID: event.CreatedByID,
		UpdatedByID: event.UpdatedByID,
	}
}

// ToLegResponse converts an EventLeg to EventLegResponse
func (s *Service) ToLegResponse(leg *EventLeg) EventLegResponse {
	additionalData := make(map[string]interface{})
	if leg.AdditionalData != nil {
		additionalData = map[string]interface{}(leg.AdditionalData)
	}

	return EventLegResponse{
		ID:             leg.ID,
		EventID:        leg.EventID,
		LegNumber:      leg.LegNumber,
		Origin:         leg.Origin,
		Destination:    leg.Destination,
		RouteATID:      leg.RouteATID,
		Description:    leg.Description,
		ThumbnailURL:   leg.ThumbnailURL,
		AdditionalData: additionalData,
		CreatedAt:      leg.CreatedAt,
		UpdatedAt:      leg.UpdatedAt,
		CreatedByID:    leg.CreatedByID,
		UpdatedByID:    leg.UpdatedByID,
	}
}
