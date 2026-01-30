package sync

import (
	"database/sql"
	"time"
)

// RouteATSynced represents a route synchronized from Airtable with coordinate enrichment
type RouteATSynced struct {
	ID             string          `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ATID           string          `gorm:"column:at_id;type:varchar(20);not null"`
	ServerID       string          `gorm:"column:server_id;type:uuid;not null"`
	Origin         string          `gorm:"column:origin;type:varchar(10)"`
	Destination    string          `gorm:"column:destination;type:varchar(10)"`
	Route          string          `gorm:"column:route;type:text"`
	OriginLat      sql.NullFloat64 `gorm:"column:origin_lat;type:numeric(10,6)"`
	OriginLon      sql.NullFloat64 `gorm:"column:origin_lon;type:numeric(10,6)"`
	DestinationLat sql.NullFloat64 `gorm:"column:destination_lat;type:numeric(10,6)"`
	DestinationLon sql.NullFloat64 `gorm:"column:destination_lon;type:numeric(10,6)"`
	CreatedAt      time.Time       `gorm:"column:created_at;default:now()"`
	UpdatedAt      time.Time       `gorm:"column:updated_at;default:now()"`
}

// TableName specifies the table name for RouteATSynced
func (RouteATSynced) TableName() string {
	return "routes_at_synced"
}

// VASyncHistory tracks the last sync time for each VA and sync event type
type VASyncHistory struct {
	ID         string     `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VAID       string     `gorm:"column:va_id;type:uuid;not null"`
	Event      string     `gorm:"column:event;type:varchar(50);not null"`
	LastSyncAt *time.Time `gorm:"column:last_sync_at"`
	CreatedAt  time.Time  `gorm:"column:created_at;default:now()"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;default:now()"`
}

// TableName specifies the table name for VASyncHistory
func (VASyncHistory) TableName() string {
	return "va_sync_history"
}
