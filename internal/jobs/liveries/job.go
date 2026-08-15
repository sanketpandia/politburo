package liveries

import (
	"context"
	"fmt"
	"log/slog"

	"infinite-experiment/politburo/internal/cache"
	gameliveries "infinite-experiment/politburo/internal/game/liveries"
	"infinite-experiment/politburo/internal/infiniteflight"
)

const jobName = "infinite-flight-liveries"

type Job struct {
	client infiniteflight.LiveriesClient
	cache  cache.Store
}

func New(client infiniteflight.LiveriesClient, cacheStore cache.Store) *Job {
	return &Job{client: client, cache: cacheStore}
}

func (j *Job) Name() string {
	return jobName
}

func (j *Job) Run(ctx context.Context) error {
	upstream, err := j.client.GetAircraftLiveries(ctx)
	if err != nil {
		return fmt.Errorf("refresh liveries: %w", err)
	}

	for _, item := range upstream {
		livery := gameliveries.Livery{
			ID: item.ID, AircraftID: item.AircraftID, AircraftName: item.AircraftName, LiveryName: item.LiveryName,
		}
		if err := j.cache.SetJSON(ctx, cache.KeyLivery(livery.ID), livery, gameliveries.CacheTTL); err != nil {
			return fmt.Errorf("cache livery %s: %w", livery.ID, err)
		}
	}
	slog.Info("Infinite Flight liveries refreshed", "liveries", len(upstream))
	return nil
}
