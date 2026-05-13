package messaging

import (
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"infinite-experiment/politburo/infra/metrics"
)

// MetricsMiddleware returns a watermill handler middleware that records
// per-handler duration and error counts using the existing queue metric label
// sets from the MetricsRegistry. The handlerName label matches the name
// registered with the Router.
func MetricsMiddleware(reg *metrics.MetricsRegistry) func(message.HandlerFunc) message.HandlerFunc {
	return func(next message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			start := time.Now()

			out, err := next(msg)

			elapsed := time.Since(start).Seconds()
			handlerName := msg.Metadata.Get("handler_name")
			if handlerName == "" {
				handlerName = "unknown"
			}

			reg.WatermillHandlerDuration.WithLabelValues(handlerName).Observe(elapsed)

			if err != nil {
				reg.WatermillHandlerErrors.WithLabelValues(handlerName).Inc()
			}

			return out, err
		}
	}
}
