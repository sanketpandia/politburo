package gorm

import (
	"time"
)

// WorldTour represents a multi-leg tour event for a Virtual Airline
type WorldTour struct {
	ID               string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID             string    `gorm:"column:va_id;type:uuid;not null;index"`
	Name             string    `gorm:"column:name;type:varchar(255);not null"`
	Description      *string   `gorm:"column:description;type:text"`
	DocumentationURL *string   `gorm:"column:documentation_url;type:text"`
	Status           string    `gorm:"column:status;type:varchar(20);default:'draft'"`
	FlightModeKey    string    `gorm:"column:flight_mode_key;type:varchar(100);not null"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime:milli"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:timestamptz;autoUpdateTime:milli"`
	CreatedByID      *string   `gorm:"column:created_by_id;type:uuid"`

	// Computed fields (not stored in database)
	TotalLegs int `gorm:"-"`

	// Relations
	VA        *VA            `gorm:"foreignKey:VAID"`
	CreatedBy *User          `gorm:"foreignKey:CreatedByID"`
	Legs      []WorldTourLeg `gorm:"foreignKey:WorldTourID"`
}

// TableName specifies the table name for WorldTour
func (WorldTour) TableName() string {
	return "world_tours"
}

// IsActive returns true if the tour status is 'active'
func (wt *WorldTour) IsActive() bool {
	return wt.Status == "active"
}

// IsPublished returns true if the tour status is not 'draft'
func (wt *WorldTour) IsPublished() bool {
	return wt.Status != "draft"
}

// WorldTourLeg represents a single leg/segment of a world tour
type WorldTourLeg struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	WorldTourID string    `gorm:"column:world_tour_id;type:uuid;not null;index"`
	LegNumber   int       `gorm:"column:leg_number;not null"`
	Name        string    `gorm:"column:name;type:varchar(255);not null"`
	RouteName   string    `gorm:"column:route_name;type:varchar(255);not null"`
	RouteATID   *string   `gorm:"column:route_at_id;type:varchar(20)"`
	Description *string   `gorm:"column:description;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime:milli"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:timestamptz;autoUpdateTime:milli"`

	// Relations
	WorldTour *WorldTour `gorm:"foreignKey:WorldTourID"`
}

// TableName specifies the table name for WorldTourLeg
func (WorldTourLeg) TableName() string {
	return "world_tour_legs"
}
