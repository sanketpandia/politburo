package pilots

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// LinkingJob links users in va_user_roles to their Airtable IDs
// using the pilot_at_synced table as a lookup
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
	log.Printf("[PilotLinkingJob] Starting pilot linking at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		log.Printf("[PilotLinkingJob] Error fetching active VAs: %v", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		log.Printf("[PilotLinkingJob] No VAs with active Airtable configs found")
		return nil
	}

	log.Printf("[PilotLinkingJob] Found %d VAs with active Airtable configs", len(vaIDs))

	// Link pilots for each VA
	totalLinked := 0
	for _, vaID := range vaIDs {
		linked, err := j.LinkVAPilots(ctx, vaID)
		if err != nil {
			log.Printf("[PilotLinkingJob] Error linking pilots for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		totalLinked += linked
	}

	log.Printf("[PilotLinkingJob] Completed pilot linking in %s. Total pilots linked: %d",
		time.Since(start).Truncate(time.Millisecond), totalLinked)

	return nil
}

// LinkVAPilots links pilots for a specific VA
func (j *LinkingJob) LinkVAPilots(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[PilotLinkingJob] Linking pilots for VA %s", vaID)

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	// Get callsign prefix from va_configs
	callsignPrefix, ok := j.vaConfigService.GetConfigVal(ctx, vaID, common.ConfigKeyAirtableCallsignColumnPrefix)
	if !ok || callsignPrefix == "" {
		log.Printf("[PilotLinkingJob] VA %s: No callsign prefix configured, skipping", vaName)
		return 0, nil
	}

	log.Printf("[PilotLinkingJob] VA %s: Using callsign prefix '%s'", vaName, callsignPrefix)

	// Get all users in va_user_roles for this VA who don't have airtable_pilot_id set
	users, err := j.pilotRepo.GetUnlinkedUsers(ctx, vaID)
	if err != nil {
		return 0, fmt.Errorf("failed to query unlinked users: %w", err)
	}

	if len(users) == 0 {
		log.Printf("[PilotLinkingJob] VA %s: No users found that need linking", vaName)
		return 0, nil
	}

	log.Printf("[PilotLinkingJob] VA %s: Found %d users to link", vaName, len(users))

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
			log.Printf("[PilotLinkingJob] VA %s: Error looking up callsign '%s': %v", vaName, fullCallsign, err)
			errorCount++
			continue
		}

		if pilotSync == nil {
			log.Printf("[PilotLinkingJob] VA %s: Callsign '%s' not found in pilot_at_synced", vaName, fullCallsign)
			continue
		}

		// Update va_user_roles with airtable_pilot_id
		err = j.pilotRepo.UpdateUserAirtableID(ctx, user.ID, pilotSync.ATID)
		if err != nil {
			log.Printf("[PilotLinkingJob] VA %s: Error updating user %s with airtable_pilot_id: %v", vaName, user.ID, err)
			errorCount++
			continue
		}

		log.Printf("[PilotLinkingJob] VA %s: Linked callsign '%s' to Airtable ID '%s'", vaName, fullCallsign, pilotSync.ATID)
		linkedCount++
	}

	log.Printf("[PilotLinkingJob] VA %s: Completed in %s. Linked: %d, Errors: %d",
		vaName, time.Since(start).Truncate(time.Millisecond), linkedCount, errorCount)

	return linkedCount, nil
}
