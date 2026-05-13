package messaging

import (
	"github.com/ThreeDotsLabs/watermill/message"
	"infinite-experiment/politburo/infra/metrics"
	"go.uber.org/zap"
)

// NewRouter creates a watermill Router pre-configured with the standard
// middleware stack: PoisonQueueMiddleware (outermost) → MetricsMiddleware → handler.
// This ordering ensures MetricsMiddleware observes the real error before
// PoisonQueueMiddleware suppresses it.
// Callers add handlers via the returned *message.Router then call Run(ctx).
func NewRouter(reg *metrics.MetricsRegistry, publisher message.Publisher, logger *zap.Logger) (*message.Router, error) {
	router, err := message.NewRouter(
		message.RouterConfig{},
		NewZapLogger(logger),
	)
	if err != nil {
		return nil, err
	}

	// Middleware execution order (outermost first):
	// PoisonQueueMiddleware → MetricsMiddleware → handler
	//
	// MetricsMiddleware is registered second so it executes closest to the
	// handler and can observe the real error before PoisonQueueMiddleware
	// suppresses it and routes the message to TopicPirepPoison.
	router.AddMiddleware(
		PoisonQueueMiddleware(publisher, reg),
		MetricsMiddleware(reg),
	)

	return router, nil
}
