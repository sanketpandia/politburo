package va

// Sync event type constants for tracking sync operations in va_sync_history table
// These constants define the event names used when recording sync history
const (
	// SyncEventPilotsAT indicates a pilot sync from Airtable
	SyncEventPilotsAT = "PILOT_AT_SYNC"

	// SyncEventRoutesAT indicates a route sync from Airtable
	SyncEventRoutesAT = "ROUTES_AT_SYNC"

	// SyncEventPirepsAT indicates a PIREP sync from Airtable
	SyncEventPirepsAT = "PIREPS_AT_SYNC"
)
