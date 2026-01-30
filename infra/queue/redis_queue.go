package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisQueueService provides queue functionality using Redis Streams
type RedisQueueService struct {
	client *redis.Client
}

// NewRedisQueueService creates a new Redis queue service
func NewRedisQueueService(client *redis.Client) *RedisQueueService {
	return &RedisQueueService{
		client: client,
	}
}

// PirepQueueItem represents a PIREP record to be processed
type PirepQueueItem struct {
	VATID            string                 `json:"va_id"`
	AirtableRecordID string                 `json:"airtable_record_id"`
	Fields           map[string]interface{} `json:"fields"`
	CreatedTime      string                 `json:"created_time"`
}

// EnqueuePirep adds a PIREP to the processing queue using Redis Stream
func (s *RedisQueueService) EnqueuePirep(ctx context.Context, streamName string, item *PirepQueueItem) error {
	// Serialize the item to JSON
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal PIREP item: %w", err)
	}

	// Add to Redis Stream
	// XADD stream_name * data <json>
	args := &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}

	_, err = s.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to add to stream: %w", err)
	}

	return nil
}

// EnqueuePirepBatch adds multiple PIREPs to the queue in a single operation
func (s *RedisQueueService) EnqueuePirepBatch(ctx context.Context, streamName string, items []*PirepQueueItem) error {
	if len(items) == 0 {
		return nil
	}

	// Use pipeline for batch operations
	pipe := s.client.Pipeline()

	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			log.Printf("[RedisQueue] Warning: failed to marshal item %s: %v", item.AirtableRecordID, err)
			continue
		}

		args := &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(data),
			},
		}
		pipe.XAdd(ctx, args)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil
}

// DequeuePirep reads a PIREP from the queue using consumer group
// Returns (item, messageID, error)
func (s *RedisQueueService) DequeuePirep(ctx context.Context, streamName, groupName, consumerName string, blockTime time.Duration) (*PirepQueueItem, string, error) {
	// Read from stream using consumer group
	// XREADGROUP GROUP group consumer BLOCK milliseconds COUNT 1 STREAMS stream >
	args := &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"}, // ">" means new messages only
		Count:    1,
		Block:    blockTime,
	}

	streams, err := s.client.XReadGroup(ctx, args).Result()
	if err != nil {
		if err == redis.Nil {
			// No messages available (timeout)
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, "", nil
	}

	msg := streams[0].Messages[0]

	// Extract data from message
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		return nil, "", fmt.Errorf("invalid message format: data field missing")
	}

	// Deserialize
	var item PirepQueueItem
	if err := json.Unmarshal([]byte(dataStr), &item); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal PIREP item: %w", err)
	}

	return &item, msg.ID, nil
}

// AckPirep acknowledges successful processing of a message
func (s *RedisQueueService) AckPirep(ctx context.Context, streamName, groupName, messageID string) error {
	return s.client.XAck(ctx, streamName, groupName, messageID).Err()
}

// CreateConsumerGroup creates a consumer group for the stream if it doesn't exist
func (s *RedisQueueService) CreateConsumerGroup(ctx context.Context, streamName, groupName string) error {
	// Try to create group starting from beginning (0)
	// XGROUP CREATE stream group 0 MKSTREAM
	err := s.client.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists" {
		// Group already exists, this is fine
		return nil
	}
	return err
}

// GetQueueLength returns the number of pending messages in the stream
func (s *RedisQueueService) GetQueueLength(ctx context.Context, streamName string) (int64, error) {
	length, err := s.client.XLen(ctx, streamName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return length, nil
}

// GetPendingCount returns the number of pending (unacknowledged) messages for a consumer group
func (s *RedisQueueService) GetPendingCount(ctx context.Context, streamName, groupName string) (int64, error) {
	pending, err := s.client.XPending(ctx, streamName, groupName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get pending count: %w", err)
	}
	return pending.Count, nil
}

// TrimStream removes old processed messages from the stream
// Keeps only the most recent maxLen messages
func (s *RedisQueueService) TrimStream(ctx context.Context, streamName string, maxLen int64) error {
	return s.client.XTrimMaxLen(ctx, streamName, maxLen).Err()
}

// ClaimStalePireps claims messages that have been pending for too long (likely from dead workers)
// Returns the claimed items
func (s *RedisQueueService) ClaimStalePireps(ctx context.Context, streamName, groupName, consumerName string, minIdleTime time.Duration) ([]*PirepQueueItem, []string, error) {
	// Get pending messages
	pending, err := s.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamName,
		Group:  groupName,
		Start:  "-",
		End:    "+",
		Count:  100, // Claim up to 100 stale messages at a time
	}).Result()

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pending) == 0 {
		return nil, nil, nil
	}

	var staleIDs []string
	for _, p := range pending {
		if p.Idle >= minIdleTime {
			staleIDs = append(staleIDs, p.ID)
		}
	}

	if len(staleIDs) == 0 {
		return nil, nil, nil
	}

	// Claim stale messages
	messages, err := s.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   streamName,
		Group:    groupName,
		Consumer: consumerName,
		MinIdle:  minIdleTime,
		Messages: staleIDs,
	}).Result()

	if err != nil {
		return nil, nil, fmt.Errorf("failed to claim stale messages: %w", err)
	}

	// Parse claimed messages
	var items []*PirepQueueItem
	var messageIDs []string
	for _, msg := range messages {
		dataStr, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var item PirepQueueItem
		if err := json.Unmarshal([]byte(dataStr), &item); err != nil {
			log.Printf("[RedisQueue] Warning: failed to unmarshal claimed message: %v", err)
			continue
		}

		items = append(items, &item)
		messageIDs = append(messageIDs, msg.ID)
	}

	return items, messageIDs, nil
}
