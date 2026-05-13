package messaging

import (
	watermillredis "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewSubscriber creates a Redis Stream watermill subscriber that reads from
// the given consumer group and consumer name. BlockTime and MaxIdleTime are
// set from the package-level constants.
func NewSubscriber(client *goredis.Client, consumerGroup, consumerName string, logger *zap.Logger) (*watermillredis.Subscriber, error) {
	sub, err := watermillredis.NewSubscriber(
		watermillredis.SubscriberConfig{
			Client:        client,
			ConsumerGroup: consumerGroup,
			Consumer:      consumerName,
			BlockTime:     DefaultBlockTime,
			MaxIdleTime:   DefaultMaxIdle,
		},
		NewZapLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	return sub, nil
}
