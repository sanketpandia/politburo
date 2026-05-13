package messaging

// TopicPirepSync is the Redis Stream topic that receives one message per
// Airtable PIREP record fetched during the PIREP sync job. Consumers in the
// "pirep-handlers" consumer group upsert each record into pirep_at_synced.
const TopicPirepSync = "wm:pirep:sync"

// TopicPirepPoison is the Redis Stream topic that receives messages which
// could not be processed after all retries are exhausted. These messages are
// written by the PoisonQueue middleware and must not be re-delivered automatically.
const TopicPirepPoison = "wm:pirep:poison"
