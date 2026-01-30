package jobs

import (
	"infinite-experiment/politburo/internal/common"
	"context"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/services"
	"log"
	"time"
)

func SyncPilotsJob(
	ctx context.Context,
	svc *common.AirtableApiService,
	svc1 *services.AtSyncService,
	lastModified *time.Time,
) {
	start := time.Now()
	log.Printf("[SyncPilotsJob] Starting pilot sync at %s", start.Format(time.RFC3339))

	var (
		offset string
		hasLM  bool
		lm     *time.Time
		count  int
	)

	if lastModified != nil {
		hasLM = true
		lm = lastModified
	}

	log.Printf("[SyncPilotsJob] Has last modified %t, with val %v", hasLM, *lm)

	claims := auth.GetUserClaims(ctx)
	serverID := claims.ServerID()

	for {
		data, err := svc.FetchRecords(ctx, common.ATTypePilot, hasLM, lm, offset)
		if err != nil {
			log.Printf("[SyncPilotsJob] Error fetching records: %v", err)
			return
		}

		if err := svc1.ParseAndUpsertRecords(ctx, data.Results, common.ATTypePilot, serverID, svc); err != nil {
			log.Printf("[SyncPilotsJob] Error parsing records: %v", err)
			return
		}

		count++

		log.Printf("[SyncPilotsJob] Error fetching records: %v", err)
		log.Printf("[SyncPilotsJob] Error fetching records: %v", err)

		if data.Offset == "" {
			break
		}
		offset = data.Offset
	}

	log.Printf("[SyncPilotsJob] Completed pilot sync in %s after %d pages",
		time.Since(start).Truncate(time.Millisecond), count)
}

func SyncRoutesJob(
	ctx context.Context,
	svc *common.AirtableApiService,
	svc1 *services.AtSyncService,
	lastModified *time.Time,
) {
	start := time.Now()
	log.Printf("[SyncRoutesJob] Starting route sync at %s", start.Format(time.RFC3339))

	var (
		offset string
		hasLM  bool
		lm     *time.Time
		count  int
	)

	claims := auth.GetUserClaims(ctx)
	serverId := claims.ServerID()

	if lastModified != nil {
		hasLM = true
		lm = lastModified
	}

	log.Printf("[SyncRoutesJob] Has last modified %t, with val %v", hasLM, lm)

	for {
		data, err := svc.FetchRecords(ctx, common.ATTypeRoute, hasLM, lm, offset)
		if err != nil {
			log.Printf("[SyncRoutesJob] Error fetching records: %v", err)
			return
		}

		log.Printf("[SyncRoutesJob] Row count: %d", len(data.Results))

		if err := svc1.ParseAndUpsertRecords(ctx, data.Results, common.ATTypeRoute, serverId, svc); err != nil {
			log.Printf("[SyncRoutesJob] Error parsing records: %v", err)
			return
		}

		count++

		if data.Offset == "" {
			break
		}
		offset = data.Offset
	}

	log.Printf("[SyncRoutesJob] Completed route sync in %s after %d pages",
		time.Since(start).Truncate(time.Millisecond), count)
}

// func SyncPirepsJob(
// 	ctx context.Context,
// 	svc *common.AirtableApiService,
// 	svc1 *services.AtSyncService,
// 	lastModified *time.Time,
// ) {
// 	start := time.Now()
// 	log.Printf("[SyncPirepsJob] Starting PIREP sync at %s", start.Format(time.RFC3339))

// 	// TODO: Implement similar logic for PIREPs
// }
