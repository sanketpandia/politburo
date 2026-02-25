package pireps

import "time"

// PirepATSynced represents a PIREP (flight log) record synced from Airtable
type PirepATSynced struct {
	ID       string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ATID     string `gorm:"column:at_id;type:varchar(20);not null"`
	ServerID string `gorm:"column:server_id;type:uuid;not null"`

	// Core PIREP fields
	Route         string   `gorm:"column:route;type:text"`
	FlightMode    string   `gorm:"column:flight_mode;type:varchar(50)"`
	FlightTime    *float64 `gorm:"column:flight_time;type:numeric(10,2)"`
	PilotCallsign string   `gorm:"column:pilot_callsign;type:varchar(50)"`
	Aircraft      string   `gorm:"column:aircraft;type:varchar(100)"`
	Livery        string   `gorm:"column:livery;type:varchar(100)"`

	// References to synced records (Airtable IDs)
	RouteATID *string `gorm:"column:route_at_id;type:varchar(20)"`
	PilotATID *string `gorm:"column:pilot_at_id;type:varchar(20)"`

	// Airtable metadata
	ATCreatedTime *time.Time `gorm:"column:at_created_time"`

	// Backfilling progress
	BackfillStatus int `gorm:"column:backfill_status;type:integer;not null;default:0"`

	// Timestamps
	CreatedAt time.Time `gorm:"column:created_at;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:now()"`
}

// TableName specifies the table name for GORM
func (PirepATSynced) TableName() string {
	return "pirep_at_synced"
}
