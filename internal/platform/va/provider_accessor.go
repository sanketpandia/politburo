package va

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/cache"
)

const (
	ProviderTypeAirtable = "airtable"

	ConfigTypeCredentials       = "credentials"
	ConfigTypePilot             = "pilot"
	ConfigTypeRoute             = "route"
	ConfigTypePirep             = "pirep"
	ConfigTypeCareerMode        = "career_mode"
	ConfigTypeFeaturePilotStats = "feature_pilot_stats"

	providerCredsCachePrefix     = "config:airtable_creds:"
	providerSchemaCachePrefix    = "config:airtable_schema:"
	featurePilotStatsCachePrefix = "config:feature_pilot_stats:"
)

type ProviderConfigAccessor struct {
	repo      *Repository
	configSvc *ConfigService
	cache     cache.CacheInterface
}

func NewProviderConfigAccessor(repo *Repository, configSvc *ConfigService, cacheStore cache.CacheInterface) *ProviderConfigAccessor {
	return &ProviderConfigAccessor{
		repo:      repo,
		configSvc: configSvc,
		cache:     cacheStore,
	}
}

func (a *ProviderConfigAccessor) GetBasicConfigValue(ctx context.Context, vaID, key string) (string, bool) {
	if a == nil || a.configSvc == nil {
		return "", false
	}
	return a.configSvc.GetConfigVal(ctx, vaID, key)
}

func (a *ProviderConfigAccessor) GetAirtableCredentials(ctx context.Context, vaID string) (*ProviderCredentials, error) {
	if a == nil || a.repo == nil {
		return nil, fmt.Errorf("provider config accessor not initialized")
	}

	if a.cache != nil {
		cacheKey := providerCredsCachePrefix + vaID
		if cached, found := a.cache.Get(cacheKey); found {
			if creds, ok := cached.(*ProviderCredentials); ok {
				return creds, nil
			}
		}
	}

	creds, err := a.repo.GetAirtableCredentials(ctx, vaID)
	if err != nil || creds == nil {
		return creds, err
	}

	if a.cache != nil {
		a.cache.Set(providerCredsCachePrefix+vaID, creds, 24*time.Hour)
	}

	return creds, nil
}

func (a *ProviderConfigAccessor) GetAirtableSchema(ctx context.Context, vaID, schemaType string) (*SchemaConfig, error) {
	if a == nil || a.repo == nil {
		return nil, fmt.Errorf("provider config accessor not initialized")
	}

	if a.cache != nil {
		cacheKey := providerSchemaCachePrefix + vaID + ":" + schemaType
		if cached, found := a.cache.Get(cacheKey); found {
			if schema, ok := cached.(*SchemaConfig); ok {
				return schema, nil
			}
		}
	}

	schema, err := a.repo.GetAirtableSchema(ctx, vaID, schemaType)
	if err != nil || schema == nil {
		return schema, err
	}

	if a.cache != nil {
		a.cache.Set(providerSchemaCachePrefix+vaID+":"+schemaType, schema, 24*time.Hour)
	}

	return schema, nil
}

func (a *ProviderConfigAccessor) GetFeaturePilotStatsConfig(ctx context.Context, vaID string) (*FeaturePilotStatsConfig, error) {
	if a == nil || a.repo == nil {
		return nil, fmt.Errorf("provider config accessor not initialized")
	}

	if a.cache != nil {
		cacheKey := featurePilotStatsCachePrefix + vaID
		if cached, found := a.cache.Get(cacheKey); found {
			if cfg, ok := cached.(*FeaturePilotStatsConfig); ok {
				return cfg, nil
			}
		}
	}

	featureCfg, err := a.repo.GetFeaturePilotStatsConfig(ctx, vaID)
	if err != nil || featureCfg == nil {
		return featureCfg, err
	}

	if a.cache != nil {
		a.cache.Set(featurePilotStatsCachePrefix+vaID, featureCfg, 20*time.Minute)
	}

	return featureCfg, nil
}
