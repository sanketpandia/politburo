package airports

import (
	"database/sql"
	"time"
)

// Airport represents an airport record with geographic coordinates
type Airport struct {
	ID        string         `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ICAO      string         `gorm:"column:icao;type:varchar(4);not null;uniqueIndex"`
	IATA      string         `gorm:"column:iata;type:varchar(3)"`
	Name      string         `gorm:"column:name;type:text;not null"`
	City      string         `gorm:"column:city;type:varchar(100)"`
	Country   string         `gorm:"column:country;type:varchar(100)"`
	Elevation sql.NullInt64  `gorm:"column:elevation;type:integer"`
	Latitude  float64        `gorm:"column:latitude;type:numeric(10,6);not null"`
	Longitude float64        `gorm:"column:longitude;type:numeric(10,6);not null"`
	Timezone  string         `gorm:"column:timezone;type:varchar(50)"`
	CreatedAt time.Time      `gorm:"column:created_at;default:now()"`
	UpdatedAt time.Time      `gorm:"column:updated_at;default:now()"`
}

// TableName specifies the table name for GORM
func (Airport) TableName() string {
	return "airports"
}
