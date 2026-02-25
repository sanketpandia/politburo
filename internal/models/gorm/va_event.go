package gorm

import (
	"database/sql"
	"time"
)

// VAEvent represents an event with a predefined route for a virtual airline
type VAEvent struct {
	ID              string     `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID            string     `gorm:"column:va_id;type:uuid;not null;index"`
	EventName       string        `gorm:"column:event_name;type:varchar(100);not null"`
	Description     *string       `gorm:"column:description;type:text"`
	PredefinedRoute string        `gorm:"column:predefined_route;type:varchar(20);not null"`
	RouteATID       *string       `gorm:"column:route_at_id;type:varchar(20)"`
	StartDate       sql.NullTime  `gorm:"column:start_date;type:timestamptz;index:idx_va_events_date_range"`
	EndDate         sql.NullTime  `gorm:"column:end_date;type:timestamptz;index:idx_va_events_date_range"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamptz;autoCreateTime:milli"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamptz;autoUpdateTime:milli"`
	CreatedByID     *string    `gorm:"column:created_by;type:uuid;index"`

	// IsActive is calculated at application level based on current time and date range
	// Not stored in database, populated when fetching events
	IsActive bool `gorm:"-"`

	// Relations
	VA        *VA    `gorm:"foreignKey:VAID"`
	CreatedBy *User  `gorm:"foreignKey:CreatedByID"`
}

// TableName specifies the table name for VAEvent
func (VAEvent) TableName() string {
	return "va_events"
}

// CalculateIsActive determines if the event is currently active based on current time
// Logic:
// - If StartDate is NULL: event started immediately
// - If EndDate is NULL: event never expires
// - An event is active if:
//   - (StartDate is NULL OR current time >= StartDate) AND
//   - (EndDate is NULL OR current time <= EndDate)
func (e *VAEvent) CalculateIsActive() {
	now := time.Now().UTC()

	// Check start date condition
	startCondition := !e.StartDate.Valid || now.After(e.StartDate.Time) || now.Equal(e.StartDate.Time)

	// Check end date condition
	endCondition := !e.EndDate.Valid || now.Before(e.EndDate.Time) || now.Equal(e.EndDate.Time)

	e.IsActive = startCondition && endCondition
}
