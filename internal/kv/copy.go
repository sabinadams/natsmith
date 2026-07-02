package kv

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/workpool"
)

// CopyBucket writes migratable keys to dest.
func CopyBucket(ctx context.Context, dest jetstream.KeyValue, keys []string, values map[string][]byte, skipExisting bool, workers int, bar *progress.BucketBar) (progress.ItemStats, error) {
	stats := progress.ItemStats{Total: len(keys)}
	if len(keys) == 0 {
		return stats, nil
	}

	var statsMu sync.Mutex
	err := workpool.RunParallel(ctx, workers, keys, func(ctx context.Context, key string) error {
		bar.SetItem(key)

		value, ok := values[key]
		if !ok {
			return fmt.Errorf("put key %q: no value from stream scan", key)
		}

		if skipExisting {
			if _, err := dest.Get(ctx, key); err == nil {
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				bar.Add(1)
				return nil
			} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
				return fmt.Errorf("check destination key %q: %w", key, err)
			}
		}

		if _, err := dest.Put(ctx, key, value); err != nil {
			return fmt.Errorf("put key %q: %w", key, err)
		}

		statsMu.Lock()
		stats.Migrated++
		statsMu.Unlock()
		bar.Add(1)
		return nil
	})
	bar.ClearItem()
	return stats, err
}
