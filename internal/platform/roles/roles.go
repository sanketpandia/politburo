package roles

import (
	"database/sql/driver"
	"fmt"
)

// VARole mirrors the Postgres ENUM 'va_role'
type VARole string

const (
	RolePilot          VARole = "pilot"
	RoleAirlineManager VARole = "staff"
	RoleAdmin          VARole = "admin"
)

// Stringer – convenient for fmt / logs
func (r VARole) String() string { return string(r) }

/* ---------- DB adapters so sqlx (or database/sql) scans/values cleanly ---------- */

// Scan implements the sql.Scanner interface
func (r *VARole) Scan(src interface{}) error {
	if src == nil {
		*r = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*r = VARole(v)
	case []byte:
		*r = VARole(v)
	default:
		return fmt.Errorf("VARole: cannot scan type %T", src)
	}
	return nil
}

// Value implements the driver.Valuer interface
func (r VARole) Value() (driver.Value, error) { return string(r), nil }
