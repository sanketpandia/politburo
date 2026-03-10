package pilots

import "fmt"

// ============================================================================
// Entity Models
// ============================================================================

// PilotATSynced represents a pilot record synced from Airtable (entity)
type PilotATSynced struct {
	ID         string `db:"id"`         // UUID
	ATID       string `db:"at_id"`      // varchar(20)
	Callsign   string `db:"callsign"`   // varchar(20)
	Registered bool   `db:"registered"` // boolean
	ServerID   string `db:"server_id"`
}

// ============================================================================
// GORM Models (with database tags)
// ============================================================================

// PilotATSyncedGORM represents a pilot record synced from Airtable (GORM)
type PilotATSyncedGORM struct {
	ID         string     `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ATID       string     `gorm:"column:at_id;type:varchar(20);not null"`
	Callsign   string     `gorm:"column:callsign;type:varchar(20)"`
	Registered bool       `gorm:"column:registered;default:false"`
	ServerID   string     `gorm:"column:server_id;type:uuid"`
	PilotType  PilotType  `gorm:"column:pilot_type;type:pilot_type;default:'regular'"`
}

// TableName specifies the table name for GORM
func (PilotATSyncedGORM) TableName() string {
	return "pilot_at_synced"
}

// ============================================================================
// API Response DTOs
// ============================================================================

// StatsResponse is the response for GET /api/v1/pilot/stats
// It combines game statistics and provider data into a unified response
type StatsResponse struct {
	// Infinite Flight game statistics (from Live API) - Future implementation
	GameStats *IFGameStats `json:"game_stats,omitempty"`

	// Provider data (Airtable, Google Sheets, etc.)
	ProviderData *ProviderPilotData `json:"provider_data,omitempty"`

	// Career Mode data (if configured)
	CareerModeData *CareerModeData `json:"career_mode_data,omitempty"`

	// Recent PIREPs (flight logs) from synced data
	RecentPIREPs []RecentPIREP `json:"recent_pireps,omitempty"`

	// Metadata about the response
	Metadata StatsMetadata `json:"metadata"`
}

// IFGameStats represents Infinite Flight Live API statistics
// This will be populated in a future implementation
type IFGameStats struct {
	FlightTime    int `json:"flight_time,omitempty"`
	OnlineFlights int `json:"online_flights,omitempty"`
	LandingCount  int `json:"landing_count,omitempty"`
	XP            int `json:"xp,omitempty"`
	Grade         int `json:"grade,omitempty"`
	Violations    int `json:"violations,omitempty"`
}

// ProviderPilotData contains standardized + custom fields from data provider
// Only fields marked as is_user_visible=true in the config will be included
type ProviderPilotData struct {
	// Standardized fields (all optional - only present if configured and available)
	FlightHours  *interface{} `json:"flight_hours,omitempty"`  // Can be int or float depending on provider
	Rank         *string      `json:"rank,omitempty"`          // Pilot rank/category
	JoinDate     *string      `json:"join_date,omitempty"`     // When pilot joined the VA
	LastActivity *string      `json:"last_activity,omitempty"` // Last activity date
	LastFlight   *string      `json:"last_flight,omitempty"`   // Last flight date
	Region       *string      `json:"region,omitempty"`        // Geographic region
	TotalFlights *int         `json:"total_flights,omitempty"` // Number of flights
	Status       *string      `json:"status,omitempty"`        // Active/inactive status

	// All other fields that don't map to standard names
	AdditionalFields map[string]interface{} `json:"additional_fields,omitempty"`
}

// CareerModeData contains career mode specific data from the provider
type CareerModeData struct {
	// Standardized career mode fields (all optional)
	TotalCMHours              *interface{} `json:"total_cm_hours,omitempty"`               // Career mode hours completed
	RequiredHoursToNext       *interface{} `json:"required_hours_to_next,omitempty"`       // Hours needed for next aircraft
	LastActivityCM            *string      `json:"last_activity_cm,omitempty"`             // Last career mode activity
	AssignedRoutes            *interface{} `json:"assigned_routes,omitempty"`              // Assigned flight routes (can be array)
	Aircraft                  *string      `json:"aircraft,omitempty"`                     // Current aircraft
	Airline                   *string      `json:"airline,omitempty"`                      // Current airline
	LastFlownRoute            *string      `json:"last_flown_route,omitempty"`             // Last PIREP route
	LastCareerModePIREP       *interface{} `json:"last_career_mode_pirep,omitempty"`       // Last PIREP log reference (Airtable IDs)
	LastCareerModeFlight      *string      `json:"last_career_mode_flight,omitempty"`      // Last career mode flight route (enriched from route_at_synced)

	// All other career mode fields that don't map to standard names
	AdditionalFields map[string]interface{} `json:"additional_fields,omitempty"`
}

// StatsMetadata provides context about the data source and freshness
type StatsMetadata struct {
	ProviderType       string `json:"provider_type,omitempty"`   // e.g., "airtable", "google_sheets"
	ProviderConfigured bool   `json:"provider_configured"`       // Whether a provider is configured for this VA
	SchemaVersion      string `json:"schema_version,omitempty"`  // Config schema version
	LastFetched        string `json:"last_fetched"`              // ISO 8601 timestamp
	Cached             bool   `json:"cached"`                    // Whether data came from cache
	VAName             string `json:"va_name,omitempty"`         // Name of the virtual airline
}

// RecentPIREP represents a recent PIREP (flight log) record
type RecentPIREP struct {
	ATID          string   `json:"at_id"`                    // Airtable record ID
	Route         string   `json:"route"`                    // Flight route (e.g., "KLAX-KSFO")
	FlightMode    string   `json:"flight_mode,omitempty"`    // Flight mode (e.g., "Casual", "Expert")
	FlightTime    *float64 `json:"flight_time,omitempty"`    // Flight duration in hours
	PilotCallsign string   `json:"pilot_callsign,omitempty"` // Pilot callsign
	Aircraft      string   `json:"aircraft,omitempty"`       // Aircraft type (e.g., "B738")
	Livery        string   `json:"livery,omitempty"`         // Aircraft livery/airline
	ATCreatedTime *string  `json:"at_created_time,omitempty"` // Airtable creation timestamp
}

// ============================================================================
// Internal Service DTOs
// ============================================================================

// MembershipWithAirtable represents user VA membership with Airtable linkage
type MembershipWithAirtable struct {
	UserID            string  `db:"user_id"`
	DiscordID         string  `db:"discord_id"`
	IFCommunityID     string  `db:"if_community_id"`
	AirtablePilotID   *string `db:"airtable_pilot_id"`
	CareerModePilotID *string `db:"career_mode_pilot_id"`
	Callsign          string  `db:"callsign"`
	Role              string  `db:"role"`
	VAName            string  `db:"va_name"`
}

// PilotStatusResponse represents a pilot search result from Airtable
type PilotStatusResponse struct {
	AirtablePilotID string                 `json:"airtable_pilot_id"`
	Callsign        string                 `json:"callsign"`
	FullCallsign    string                 `json:"full_callsign"`
	Role            string                 `json:"role"`
	RawFields       map[string]interface{} `json:"raw_fields"`
	Metadata        PilotStatusMetadata    `json:"metadata"`
}

// PilotStatusMetadata contains metadata about the pilot status response
type PilotStatusMetadata struct {
	SchemaVersion string `json:"schema_version"`
	FetchedAt     string `json:"fetched_at"`
	VAName        string `json:"va_name"`
	ConfigActive  bool   `json:"config_active"`
}

// ============================================================================
// Management Service DTOs
// ============================================================================

// PilotDTO represents pilot data for UI display
type PilotDTO struct {
	ID            string
	UserID        string
	IFCommunityID string
	Callsign      string
	Role          string
	JoinedAt      string // Formatted date
	IsActive      bool
	UpdatedAt     string // Formatted date
	CanRemove     bool   // Whether current user can remove this pilot
	CanChangeRole bool   // Whether current user can change this pilot's role
}

// SearchResult represents a pilot search result
type SearchResult struct {
	Username string
	UserID   string
	Callsign string
	Role     string
}

// ============================================================================
// Repository DTOs
// ============================================================================

// UnlinkedUser represents a user that needs to be linked to Airtable
type UnlinkedUser struct {
	ID       string `gorm:"column:id"`
	Callsign string `gorm:"column:callsign"`
}

// ============================================================================
// Error Types
// ============================================================================

// StatsError represents a pilot stats service error with error code
type StatsError struct {
	Code    string
	Message string
	Err     error
}

func (e *StatsError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *StatsError) Unwrap() error {
	return e.Err
}
