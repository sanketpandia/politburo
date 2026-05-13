// Package messaging provides watermill-based pub/sub infrastructure for
// async event processing. All Redis Stream configuration lives here.
package messaging

import "time"

// DefaultBlockTime is the maximum duration a subscriber blocks waiting for
// new messages from a Redis Stream before returning an empty result.
const DefaultBlockTime = 5 * time.Second

// DefaultMaxIdle is the duration after which a pending message owned by a
// consumer is considered stale and eligible for re-claiming by another consumer.
const DefaultMaxIdle = 5 * time.Minute
