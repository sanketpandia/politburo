package routes

import "database/sql"

// ============================================================================
// GORM Models (with database tags)
// ============================================================================

// RouteATSyncedGORM represents a route record synced from Airtable (GORM)
type RouteATSyncedGORM struct {
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
}

// TableName specifies the table name for GORM
func (RouteATSyncedGORM) TableName() string {
	return "route_at_synced"
}
