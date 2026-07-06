package kv

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
)

// ListBuckets returns KV buckets on js matching cfg filters.
func ListBuckets(ctx context.Context, js jetstream.JetStream, cfg migration.BaseConfig) ([]jetstream.KeyValueStatus, error) {
	lister := js.KeyValueStores(ctx)
	buckets := migration.FilterBucketStatuses(cfg, lister.Status())
	return buckets, lister.Error()
}
