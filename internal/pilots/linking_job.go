package pilots

// DEPRECATED: Linking job removed. Validation now happens at API level in JoinVA.
// This file is kept for reference but should be deleted after verification.
// The JoinVA endpoint now validates that callsigns exist in Airtable before allowing membership.

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/common"
	"strings"
	"time"

	"gorm.io/gorm"
)

// LinkingJob links users in va_user_roles to their Airtable IDs
// using the pilot_at_synced table as a lookup
// DEPRECATED: No longer used. Validation happens at API level.
type LinkingJob struct {
	db         *gorm.DB
	vaConfigService *common.VAConfigService
	pilotRepo *Repository
}

// NewLinkingJob creates a new pilot linking job instance
func NewLinkingJob(
	db *gorm.DB,
	vaConfigService *common.VAConfigService,
	pilotRepo *Repository,
) *LinkingJob {
	return &LinkingJob{
		db:         db,
		vaConfigService: vaConfigService,
		pilotRepo: pilotRepo,
	}
}

// Run executes the pilot linking job for all active VAs
func (j *LinkingJob) Run(ctx context.Context) error {
	start := time.Now()
	logging.Info("Pilot linking job starting")

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		logging.Error("Pilot linking job: failed to fetch active VAs", "err", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		logging.Info("Pilot linking job: no VAs with active Airtable configs")
		return nil
	}

	logging.Info("Pilot linking job: VAs found", "count", len(vaIDs))

	// Link pilots for each VA
	totalLinked := 0
	for _, vaID := range vaIDs {
		linked, err := j.LinkVAPilots(ctx, vaID)
		if err != nil {
			logging.Error("Pilot linking job: failed to link VA pilots", "va_id", vaID, "err", err)
			// Continue with other VAs even if one fails
			continue
		}
		totalLinked += linked
	}

	logging.Info("Pilot linking job completed", "duration", time.Since(start).Truncate(time.Millisecond), "total_linked", totalLinked)

	return nil
}

// LinkVAPilots links pilots for a specific VA
func (j *LinkingJob) LinkVAPilots(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	logging.Info("Pilot linking: syncing VA pilots", "va_id", vaID)

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	// Get callsign prefix from va_configs
	callsignPrefix, ok := j.vaConfigService.GetConfigVal(ctx, vaID, common.ConfigKeyAirtableCallsignColumnPrefix)
	if !ok || callsignPrefix == "" {
		logging.Info("Pilot linking: no callsign prefix configured, skipping", "va_name", vaName, "va_id", vaID)
		return 0, nil
	}

	// Get all users in va_user_roles for this VA who don't have airtable_pilot_id set
	users, err := j.pilotRepo.GetUnlinkedUsers(ctx, vaID)
	if err != nil {
		return 0, fmt.Errorf("failed to query unlinked users: %w", err)
	}

	if len(users) == 0 {
		return 0, nil
	}

	logging.Info("Pilot linking: users to link", "va_name", vaName, "va_id", vaID, "count", len(users))

	linkedCount := 0
	errorCount := 0

	for _, user := range users {
		if user.Callsign == "" {
			continue
		}

		// Construct full callsign: prefix + callsign
		fullCallsign := callsignPrefix + strings.TrimSpace(user.Callsign)

		// Look up in pilot_at_synced table
		pilotSync, err := j.pilotRepo.FindByCallsign(ctx, vaID, fullCallsign)
		if err != nil {
			logging.Error("Pilot linking: error looking up callsign", "va_id", vaID, "callsign", fullCallsign, "err", err)
			errorCount++
			continue
		}

		if pilotSync == nil {
			continue
		}

		// Update va_user_roles with airtable_pilot_id
		err = j.pilotRepo.UpdateUserAirtableID(ctx, user.ID, pilotSync.ATID)
		if err != nil {
			logging.Error("Pilot linking: error updating user airtable_pilot_id", "va_id", vaID, "user_id", user.ID, "err", err)
			errorCount++
			continue
		}

		logging.Debug("Pilot linking: linked callsign to Airtable ID", "va_id", vaID, "callsign", fullCallsign, "at_id", pilotSync.ATID)
		linkedCount++
	}

	logging.Info("Pilot linking: VA completed",
		"va_name", vaName,
		"va_id", vaID,
		"duration", time.Since(start).Truncate(time.Millisecond),
		"linked", linkedCount,
		"errors", errorCount,
	)

	return linkedCount, nil
}
