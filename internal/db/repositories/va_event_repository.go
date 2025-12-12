package repositories

import (
	"context"
	"time"

	"infinite-experiment/politburo/internal/models/gorm"

	gormdb "gorm.io/gorm"
)

// VAEventRepository handles database operations for VA events
type VAEventRepository struct {
	db *gormdb.DB
}

// NewVAEventRepository creates a new VA event repository
func NewVAEventRepository(db *gormdb.DB) *VAEventRepository {
	return &VAEventRepository{
		db: db,
	}
}

// calculateIsActiveForAll calculates IsActive for all events in the slice
func calculateIsActiveForAll(events []gorm.VAEvent) {
	for i := range events {
		events[i].CalculateIsActive()
	}
}

// Create creates a new event
func (r *VAEventRepository) Create(ctx context.Context, event *gorm.VAEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// Update updates an existing event
func (r *VAEventRepository) Update(ctx context.Context, event *gorm.VAEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// Delete deletes an event by ID
func (r *VAEventRepository) Delete(ctx context.Context, eventID string) error {
	return r.db.WithContext(ctx).Delete(&gorm.VAEvent{}, "id = ?", eventID).Error
}

// GetByID retrieves an event by ID
func (r *VAEventRepository) GetByID(ctx context.Context, eventID string) (*gorm.VAEvent, error) {
	var event gorm.VAEvent
	err := r.db.WithContext(ctx).Where("id = ?", eventID).First(&event).Error
	if err != nil {
		return nil, err
	}
	event.CalculateIsActive()
	return &event, nil
}

// GetByVA retrieves all events for a virtual airline, ordered by start_date descending (NULLs last)
func (r *VAEventRepository) GetByVA(ctx context.Context, vaID string) ([]gorm.VAEvent, error) {
	var events []gorm.VAEvent
	err := r.db.WithContext(ctx).
		Where("va_id = ?", vaID).
		Order("start_date DESC NULLS LAST, created_at DESC").
		Find(&events).Error
	if err != nil {
		return events, err
	}
	calculateIsActiveForAll(events)
	return events, nil
}

// GetActiveEvents retrieves all currently active events for a virtual airline
// An event is active if:
// - (start_date IS NULL OR NOW() >= start_date) AND
// - (end_date IS NULL OR NOW() <= end_date)
func (r *VAEventRepository) GetActiveEvents(ctx context.Context, vaID string) ([]gorm.VAEvent, error) {
	var events []gorm.VAEvent
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND (start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)", vaID, now, now).
		Order("start_date DESC NULLS LAST").
		Find(&events).Error
	if err != nil {
		return events, err
	}
	calculateIsActiveForAll(events)
	return events, nil
}

// GetEventsByDateRange retrieves events within a date range for a VA
func (r *VAEventRepository) GetEventsByDateRange(ctx context.Context, vaID string, startDate, endDate time.Time) ([]gorm.VAEvent, error) {
	var events []gorm.VAEvent
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND start_date >= ? AND end_date <= ?", vaID, startDate, endDate).
		Order("start_date ASC").
		Find(&events).Error
	if err != nil {
		return events, err
	}
	calculateIsActiveForAll(events)
	return events, nil
}

// GetEventsByRoute retrieves all events for a specific predefined route in a VA
func (r *VAEventRepository) GetEventsByRoute(ctx context.Context, vaID string, route string) ([]gorm.VAEvent, error) {
	var events []gorm.VAEvent
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND LOWER(predefined_route) = LOWER(?)", vaID, route).
		Order("start_date DESC").
		Find(&events).Error
	if err != nil {
		return events, err
	}
	calculateIsActiveForAll(events)
	return events, nil
}

// GetCountByVA gets the total count of events for a VA
func (r *VAEventRepository) GetCountByVA(ctx context.Context, vaID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&gorm.VAEvent{}).
		Where("va_id = ?", vaID).
		Count(&count).Error
	return count, err
}

// GetCountActiveByVA gets the count of currently active events for a VA
// An event is active if:
// - (start_date IS NULL OR NOW() >= start_date) AND
// - (end_date IS NULL OR NOW() <= end_date)
func (r *VAEventRepository) GetCountActiveByVA(ctx context.Context, vaID string) (int64, error) {
	var count int64
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&gorm.VAEvent{}).
		Where("va_id = ? AND (start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)", vaID, now, now).
		Count(&count).Error
	return count, err
}

// Exists checks if an event exists
func (r *VAEventRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&gorm.VAEvent{}).
		Where("id = ?", eventID).
		Count(&count).Error
	return count > 0, err
}
