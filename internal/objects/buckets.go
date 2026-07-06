package objects

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
)

// ListBuckets returns object stores on js matching cfg filters.
func ListBuckets(ctx context.Context, js jetstream.JetStream, cfg migration.BaseConfig) ([]jetstream.ObjectStoreStatus, error) {
	lister := js.ObjectStores(ctx)
	buckets := migration.FilterBucketStatuses(cfg, lister.Status())
	return buckets, lister.Error()
}
