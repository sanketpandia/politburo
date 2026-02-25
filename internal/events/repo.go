package events

import (
	"context"

	gormdb "gorm.io/gorm"
)

// Repository handles database operations for events and event legs
type Repository struct {
	db *gormdb.DB
}

// NewRepository creates a new event repository
func NewRepository(db *gormdb.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// Create creates a new event with legs (uses transaction)
func (r *Repository) Create(ctx context.Context, event *Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		// Create event
		if err := tx.Create(event).Error; err != nil {
			return err
		}

		// Create legs if any
		if len(event.Legs) > 0 {
			for i := range event.Legs {
				event.Legs[i].EventID = event.ID
			}
			if err := tx.Create(&event.Legs).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// Update updates an existing event
func (r *Repository) Update(ctx context.Context, event *Event) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// Delete deletes an event by ID (cascades to legs via database)
func (r *Repository) Delete(ctx context.Context, eventID string) error {
	return r.db.WithContext(ctx).Delete(&Event{}, "id = ?", eventID).Error
}

// GetByID retrieves an event by ID with legs
func (r *Repository) GetByID(ctx context.Context, eventID string) (*Event, error) {
	var event Event
	err := r.db.WithContext(ctx).
		Preload("Legs").
		Where("id = ?", eventID).
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetByVA retrieves all events for a virtual airline with legs
func (r *Repository) GetByVA(ctx context.Context, vaID string) ([]Event, error) {
	var events []Event
	err := r.db.WithContext(ctx).
		Preload("Legs").
		Where("va_id = ?", vaID).
		Order("start_date DESC NULLS LAST, created_at DESC").
		Find(&events).Error
	return events, err
}

// GetActiveByVA retrieves all currently active events for a virtual airline
func (r *Repository) GetActiveByVA(ctx context.Context, vaID string) ([]Event, error) {
	var events []Event
	err := r.db.WithContext(ctx).
		Preload("Legs").
		Where("va_id = ? AND status = 'active'", vaID).
		Order("start_date DESC NULLS LAST").
		Find(&events).Error
	return events, err
}

// GetByStatus retrieves events by status for a VA
func (r *Repository) GetByStatus(ctx context.Context, vaID string, status string) ([]Event, error) {
	var events []Event
	err := r.db.WithContext(ctx).
		Preload("Legs").
		Where("va_id = ? AND status = ?", vaID, status).
		Order("start_date DESC NULLS LAST, created_at DESC").
		Find(&events).Error
	return events, err
}

// Exists checks if an event exists
func (r *Repository) Exists(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Event{}).
		Where("id = ?", eventID).
		Count(&count).Error
	return count > 0, err
}

// Leg operations

// CreateLeg creates a new leg
func (r *Repository) CreateLeg(ctx context.Context, leg *EventLeg) error {
	return r.db.WithContext(ctx).Create(leg).Error
}

// UpdateLeg updates an existing leg
func (r *Repository) UpdateLeg(ctx context.Context, leg *EventLeg) error {
	return r.db.WithContext(ctx).Save(leg).Error
}

// DeleteLeg deletes a leg by ID
func (r *Repository) DeleteLeg(ctx context.Context, legID string) error {
	return r.db.WithContext(ctx).Delete(&EventLeg{}, "id = ?", legID).Error
}

// GetLegsByEvent retrieves all legs for an event, ordered by leg_number
func (r *Repository) GetLegsByEvent(ctx context.Context, eventID string) ([]EventLeg, error) {
	var legs []EventLeg
	err := r.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("leg_number ASC").
		Find(&legs).Error
	return legs, err
}

// GetLegByID retrieves a single leg by ID
func (r *Repository) GetLegByID(ctx context.Context, legID string) (*EventLeg, error) {
	var leg EventLeg
	err := r.db.WithContext(ctx).
		Where("id = ?", legID).
		First(&leg).Error
	if err != nil {
		return nil, err
	}
	return &leg, nil
}

// GetNextLegNumber gets the next available leg_number for an event
func (r *Repository) GetNextLegNumber(ctx context.Context, eventID string) (int, error) {
	var maxLegNumber int
	err := r.db.WithContext(ctx).
		Model(&EventLeg{}).
		Where("event_id = ?", eventID).
		Select("COALESCE(MAX(leg_number), 0)").
		Scan(&maxLegNumber).Error
	if err != nil {
		return 0, err
	}
	return maxLegNumber + 1, nil
}
