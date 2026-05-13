package messaging

import (
	watermillredis "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewPublisher creates a Redis Stream watermill publisher using the provided
// Redis client. The publisher is safe for concurrent use.
func NewPublisher(client *goredis.Client, logger *zap.Logger) (*watermillredis.Publisher, error) {
	pub, err := watermillredis.NewPublisher(
		watermillredis.PublisherConfig{
			Client: client,
		},
		NewZapLogger(logger),
	)
	if err != nil {
		return nil, err
	}
	return pub, nil
}
