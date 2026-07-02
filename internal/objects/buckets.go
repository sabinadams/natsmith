package objects

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
)

// ListBuckets returns object stores on js matching cfg filters.
func ListBuckets(ctx context.Context, js jetstream.JetStream, cfg migration.BaseConfig) ([]jetstream.ObjectStoreStatus, error) {
	lister := js.ObjectStores(ctx)
	var buckets []jetstream.ObjectStoreStatus
	for status := range lister.Status() {
		if cfg.ShouldMigrateBucket(status.Bucket()) {
			buckets = append(buckets, status)
		}
	}
	return buckets, lister.Error()
}
