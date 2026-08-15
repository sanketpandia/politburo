// Package sessions defines the shared cache contract for active game sessions.
package sessions

import (
	"time"

	"infinite-experiment/politburo/internal/infiniteflight"
)

const (
	RefreshSchedule = "0 */5 * * * *"
	RefreshInterval = 5 * time.Minute
	CacheTTL        = 24 * time.Hour
)

type Snapshot struct {
	Result     []infiniteflight.Session `json:"result"`
	LastCached time.Time                `json:"lastCached"`
	History    []Snapshot               `json:"history,omitempty"`
}
