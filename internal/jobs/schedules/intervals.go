// Package schedules contains the cron intervals used by scheduled jobs.
package schedules

import gamesessions "infinite-experiment/politburo/internal/game/sessions"

const (
	// SessionsSync refreshes the active Infinite Flight sessions every five minutes.
	SessionsSync = gamesessions.RefreshSchedule
)
