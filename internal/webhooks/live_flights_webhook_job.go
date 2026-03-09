package webhooks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/flights"
	platformVA 	"infinite-experiment/politburo/internal/platform/va"
)

// LiveFlightsWebhookJob runs every 30 minutes, finds VAs with active live_flights webhooks,
// fetches their live flights from cache, and POSTs a Discord-style payload to each webhook URL.
type LiveFlightsWebhookJob struct {
	webhookRepo *platformVA.WebhookRepo
	vaRepo      *platformVA.Repository
	redisCache  *cache.RedisCacheService
	httpClient  *http.Client
}

// NewLiveFlightsWebhookJob creates a new job instance
func NewLiveFlightsWebhookJob(
	webhookRepo *platformVA.WebhookRepo,
	vaRepo *platformVA.Repository,
	redisCache *cache.RedisCacheService,
) *LiveFlightsWebhookJob {
	return &LiveFlightsWebhookJob{
		webhookRepo: webhookRepo,
		vaRepo:      vaRepo,
		redisCache:  redisCache,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name implements scheduler.Job
func (j *LiveFlightsWebhookJob) Name() string {
	return "LiveFlightsWebhookJob"
}

// RunForVA runs the live flights webhook for a single VA (all its active live_flights webhooks).
// Used by the VA Admin UI "Run now" to test without waiting for the 30-min schedule.
func (j *LiveFlightsWebhookJob) RunForVA(ctx context.Context, vaID string) error {
	if j.redisCache == nil {
		return fmt.Errorf("redis cache not available")
	}
	list, err := j.webhookRepo.ListByVA(ctx, vaID)
	if err != nil {
		return fmt.Errorf("list webhooks: %w", err)
	}
	var active []platformVA.VAWebhook
	for _, w := range list {
		if w.WebhookType == WebhookTypeLiveFlights && w.IsActive {
			active = append(active, w)
		}
	}
	if len(active) == 0 {
		return fmt.Errorf("no active live_flights webhooks for this VA")
	}
	snapshotTime := time.Now().UTC()
	for _, w := range active {
		if err := j.sendForWebhook(ctx, &w, snapshotTime); err != nil {
			return fmt.Errorf("send webhook: %w", err)
		}
	}
	return nil
}

// Run implements scheduler.Job
func (j *LiveFlightsWebhookJob) Run(ctx context.Context) error {
	start := time.Now()
	logging.Info("Starting live flights webhook job")

	if j.redisCache == nil {
		logging.Warn("Redis cache not available, skipping live flights webhook job")
		return nil
	}

	active, err := j.webhookRepo.ListActiveByType(ctx, WebhookTypeLiveFlights)
	if err != nil {
		logging.Error("Failed to list active live_flights webhooks", "error", err)
		return err
	}
	if len(active) == 0 {
		logging.Debug("No active live_flights webhooks found")
		return nil
	}

	snapshotTime := time.Now().UTC()
	sent := 0
	for _, w := range active {
		if err := j.sendForWebhook(ctx, &w, snapshotTime); err != nil {
			logging.Warn("Live flights webhook failed for VA",
				"va_id", w.VAID,
				"webhook_id", w.ID,
				"error", err)
			continue
		}
		sent++
	}

	logging.Info("Live flights webhook job completed",
		"total_webhooks", len(active),
		"sent", sent,
		"duration", time.Since(start))
	return nil
}

func (j *LiveFlightsWebhookJob) sendForWebhook(ctx context.Context, w *platformVA.VAWebhook, snapshotTime time.Time) error {
	// Resolve VA name for embed title
	vaName := ""
	if j.vaRepo != nil {
		if va, err := j.vaRepo.GetByID(ctx, w.VAID); err == nil && va != nil {
			vaName = va.Name
		}
	}

	// Get live flights from cache (same as Live Flights page)
	flts, err := flights.GetVALiveFlightsDTOs(j.redisCache, w.VAID)
	if err != nil {
		return fmt.Errorf("get VA live flights: %w", err)
	}

	payload, err := BuildLiveFlightsPayload(vaName, flts, snapshotTime)
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
