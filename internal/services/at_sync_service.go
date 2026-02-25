package services

import (
	"infinite-experiment/politburo/infra/cache"
	"context"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/entities"
	"log"
)

type AtSyncService struct {
	cache *cache.CacheService
	repo  *repositories.SyncRepository
}

func NewAtSyncService(cache *cache.CacheService, repo *repositories.SyncRepository) *AtSyncService {
	return &AtSyncService{
		cache: cache,
		repo:  repo,
	}
}

func (svc *AtSyncService) ParseAndUpsertRecords(
	ctx context.Context,
	records []map[string]interface{},
	event string,
	serverID string,
	atSvc *common.AirtableApiService,
) error {
	for _, rec := range records {
		fields, atID, err := atSvc.ExtractFieldsFromRecord(ctx, serverID, event, rec)
		if err != nil {
			// log and continue
			continue
		}

		switch event {
		case common.ATTypePilot:
			p := repositories.PilotATSynced{
				ATID:       atID,
				ServerID:   serverID,
				Registered: false,
				Callsign:   fields["Callsign"],
			}
			if err := svc.repo.UpsertPilot(ctx, &p); err != nil {
				// log error
				continue
			}

		case common.ATTypeRoute:
			r := entities.RouteATSynced{
				ATID:        atID,
				ServerID:    serverID,
				Origin:      fields["Arr ICAO"],
				Destination: fields["Dep ICAO"],
				Route:       fields["Route"],
			}
			log.Printf("Route: %v", r)

			if err := svc.repo.UpsertRoute(ctx, &r); err != nil {
				// log error
				continue
			}
			// case common.ATTypePIREP:
			// 	... your logic here
		}
	}
	return nil
}
