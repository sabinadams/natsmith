package kv

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
)

// ListBuckets returns KV buckets on js matching cfg filters.
func ListBuckets(ctx context.Context, js jetstream.JetStream, cfg migration.BaseConfig) ([]jetstream.KeyValueStatus, error) {
	lister := js.KeyValueStores(ctx)
	var buckets []jetstream.KeyValueStatus
	for status := range lister.Status() {
		if cfg.ShouldMigrateBucket(status.Bucket()) {
			buckets = append(buckets, status)
		}
	}
	return buckets, lister.Error()
}
