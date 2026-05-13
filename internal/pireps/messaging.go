package pireps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/messaging"
	"infinite-experiment/politburo/infra/queue"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// HandlerName is the watermill handler name used as a label in metrics and
// as the Metadata key injected by the router.
const HandlerName = "pirep_sync"

// MessagingHandler processes PIREP sync messages published to TopicPirepSync.
// Each message carries a JSON-encoded queue.PirepQueueItem. On success the
// handler upserts the record into pirep_at_synced via UpsertPirepFromAirtable.
type MessagingHandler struct {
	db        *gorm.DB
	vaRepo    *platformVA.Repository
	pirepRepo *Repository
}

// NewMessagingHandler creates a MessagingHandler with the required dependencies.
func NewMessagingHandler(db *gorm.DB, vaRepo *platformVA.Repository, pirepRepo *Repository) *MessagingHandler {
	return &MessagingHandler{
		db:        db,
		vaRepo:    vaRepo,
		pirepRepo: pirepRepo,
	}
}

// HandlePirepSync is the watermill HandlerFunc for messages on TopicPirepSync.
// It decodes the payload, resolves the VA PIREP schema, then delegates to
// UpsertPirepFromAirtable. Returning an error causes the PoisonQueueMiddleware
// to route the message to TopicPirepPoison instead of requeueing it.
func (h *MessagingHandler) HandlePirepSync(msg *message.Message) error {
	// Stamp the handler name so middleware can label metrics correctly.
	msg.Metadata.Set("handler_name", HandlerName)

	var item queue.PirepQueueItem
	if err := json.Unmarshal(msg.Payload, &item); err != nil {
		return fmt.Errorf("decode PirepQueueItem: %w", err)
	}

	ctx := msg.Context()

	pirepSchema, err := h.vaRepo.GetAirtableSchema(ctx, item.VATID, "pirep")
	if err != nil {
		return fmt.Errorf("get schema for VA %s: %w", item.VATID, err)
	}
	if pirepSchema == nil {
		// No schema configured — log and acknowledge without erroring so we do
		// not spam the poison queue during a configuration gap.
		logging.Info("pirep messaging handler: no schema for VA, skipping",
			"va_id", item.VATID,
			"record_id", item.AirtableRecordID,
		)
		return nil
	}

	entitySchema := pirepSchema.ToEntitySchema("pirep")

	if err := UpsertPirepFromAirtable(
		ctx,
		h.db,
		h.pirepRepo,
		item.VATID,
		item.AirtableRecordID,
		item.Fields,
		item.CreatedTime,
		entitySchema,
	); err != nil {
		return fmt.Errorf("upsert PIREP %s: %w", item.AirtableRecordID, err)
	}

	return nil
}

// RegisterPirepHandlers adds the PIREP sync handler to the given watermill
// router. The subscriber must already be configured for the correct consumer
// group. This function is called during application wiring in initFeatures.
func RegisterPirepHandlers(
	router *message.Router,
	sub message.Subscriber,
	h *MessagingHandler,
) {
	router.AddNoPublisherHandler(
		HandlerName,
		messaging.TopicPirepSync,
		sub,
		h.HandlePirepSync,
	)
	logging.Info("watermill: registered handler",
		"handler", HandlerName,
		"topic", messaging.TopicPirepSync,
	)
}

// PublishPirepItem serialises a PirepQueueItem and publishes it to
// TopicPirepSync using the provided watermill Publisher.
func PublishPirepItem(ctx context.Context, pub message.Publisher, item *queue.PirepQueueItem) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal PirepQueueItem: %w", err)
	}

	msg := message.NewMessage(item.AirtableRecordID, payload)

	if err := pub.Publish(messaging.TopicPirepSync, msg); err != nil {
		return fmt.Errorf("publish to %s: %w", messaging.TopicPirepSync, err)
	}
	return nil
}
