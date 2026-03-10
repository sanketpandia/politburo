package pilots

import (
	"database/sql/driver"
	"fmt"
)

// PilotType mirrors the Postgres ENUM 'pilot_type'
type PilotType string

const (
	PilotTypeRegular    PilotType = "regular"
	PilotTypeCareerMode PilotType = "career_mode"
)

// Stringer – convenient for fmt / logs
func (p PilotType) String() string { return string(p) }

/* ---------- DB adapters so sqlx (or database/sql) scans/values cleanly ---------- */

// Scan implements the sql.Scanner interface
func (p *PilotType) Scan(src interface{}) error {
	if src == nil {
		*p = PilotTypeRegular // Default to regular
		return nil
	}
	switch v := src.(type) {
	case string:
		*p = PilotType(v)
	case []byte:
		*p = PilotType(v)
	default:
		return fmt.Errorf("PilotType: cannot scan type %T", src)
	}
	return nil
}

// Value implements the driver.Valuer interface
func (p PilotType) Value() (driver.Value, error) { return string(p), nil }
