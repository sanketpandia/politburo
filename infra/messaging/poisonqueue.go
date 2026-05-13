package messaging

import (
	"github.com/ThreeDotsLabs/watermill/message"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
)

// PoisonQueueMiddleware returns a watermill handler middleware that catches
// handler errors, publishes the offending message to TopicPirepPoison, and
// suppresses the error so the router does not attempt infinite redelivery.
// The original error is logged at Error level; the message is not re-queued.
func PoisonQueueMiddleware(publisher message.Publisher, reg *metrics.MetricsRegistry) func(message.HandlerFunc) message.HandlerFunc {
	return func(next message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			out, err := next(msg)
			if err == nil {
				return out, nil
			}

			handlerName := msg.Metadata.Get("handler_name")
			if handlerName == "" {
				handlerName = "unknown"
			}

			logging.Error("watermill handler failed; routing to poison queue",
				"handler", handlerName,
				"message_uuid", msg.UUID,
				"error", err,
			)

			// Copy message to poison topic so it can be inspected manually.
			poison := msg.Copy()
			poison.Metadata.Set("original_error", err.Error())

			if pubErr := publisher.Publish(TopicPirepPoison, poison); pubErr != nil {
				logging.Error("failed to publish to poison queue",
					"handler", handlerName,
					"message_uuid", msg.UUID,
					"error", pubErr,
				)
			} else {
				if reg != nil {
					reg.WatermillPoisonTotal.WithLabelValues(handlerName).Inc()
				}
			}

			// Return nil error so watermill acknowledges the message and does
			// not redeliver it. The message is preserved in the poison topic.
			return nil, nil
		}
	}
}
