package sync

// Sync event type constants for tracking sync operations in va_sync_history table
const (
	// SyncEventRoutesAT indicates a route sync from Airtable
	SyncEventRoutesAT = "routes_airtable_sync"

	// SyncEventPirepsAT indicates a PIREP sync from Airtable
	SyncEventPirepsAT = "pireps_airtable_sync"
)
