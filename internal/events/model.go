package events

import (
	"database/sql"
	"time"

	platformUsers "infinite-experiment/politburo/internal/platform/users"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// Event represents an event with multiple legs
type Event struct {
	ID          string       `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID        string       `gorm:"column:va_id;type:uuid;not null;index"`
	Name        string       `gorm:"column:name;type:varchar(255);not null"`
	Description *string      `gorm:"column:description;type:text"`
	Status      string       `gorm:"column:status;type:varchar(20);default:'draft'"`
	StartDate   sql.NullTime `gorm:"column:start_date;type:timestamptz;index:idx_events_date_range"`
	EndDate     sql.NullTime `gorm:"column:end_date;type:timestamptz;index:idx_events_date_range"`
	CreatedAt   time.Time    `gorm:"column:created_at;type:timestamptz;autoCreateTime:milli"`
	UpdatedAt   time.Time    `gorm:"column:updated_at;type:timestamptz;autoUpdateTime:milli"`
	CreatedByID *string      `gorm:"column:created_by_id;type:uuid;index"`
	UpdatedByID *string      `gorm:"column:updated_by_id;type:uuid;index"`

	// Relations
	VA        *platformVA.VA      `gorm:"foreignKey:VAID"`
	CreatedBy *platformUsers.User `gorm:"foreignKey:CreatedByID"`
	UpdatedBy *platformUsers.User `gorm:"foreignKey:UpdatedByID"`
	Legs      []EventLeg          `gorm:"foreignKey:EventID"`
}

// TableName specifies the table name for Event
func (Event) TableName() string {
	return "events"
}

// IsActive determines if the event is currently active
func (e *Event) IsActive() bool {
	if e.Status != "active" {
		return false
	}

	now := time.Now().UTC()
	startCondition := !e.StartDate.Valid || now.After(e.StartDate.Time) || now.Equal(e.StartDate.Time)
	endCondition := !e.EndDate.Valid || now.Before(e.EndDate.Time) || now.Equal(e.EndDate.Time)

	return startCondition && endCondition
}

// EventLeg represents a single leg of an event
type EventLeg struct {
	ID           string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	EventID      string    `gorm:"column:event_id;type:uuid;not null;index"`
	LegNumber    int       `gorm:"column:leg_number;not null"`
	Origin       string    `gorm:"column:origin;type:varchar(10);not null"`
	Destination  string    `gorm:"column:destination;type:varchar(10);not null"`
	RouteATID    *string   `gorm:"column:route_at_id;type:varchar(20)"`
	Description  *string   `gorm:"column:description;type:text"`
	ThumbnailURL *string   `gorm:"column:thumbnail_url;type:text"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime:milli"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;autoUpdateTime:milli"`
	CreatedByID  *string   `gorm:"column:created_by_id;type:uuid;index"`
	UpdatedByID  *string   `gorm:"column:updated_by_id;type:uuid;index"`

	// Relations
	Event     *Event              `gorm:"foreignKey:EventID"`
	CreatedBy *platformUsers.User `gorm:"foreignKey:CreatedByID"`
	UpdatedBy *platformUsers.User `gorm:"foreignKey:UpdatedByID"`
}

// TableName specifies the table name for EventLeg
func (EventLeg) TableName() string {
	return "event_legs"
}
