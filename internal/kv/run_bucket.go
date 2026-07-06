package kv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/workpool"
)

const (
	keyBatchSize            = 256
	destOnlyVerifyMaxKeys   = 1_000_000
	progressReportEveryKeys = 32_000
)

// BucketRunParams configures a KV bucket migration.
type BucketRunParams struct {
	DryRun       bool
	VerifyOnly   bool
	SkipExisting bool
	Verify       bool
	Workers      int
	Dest         jetstream.KeyValue
}

// BucketRunResult summarizes a bucket run.
type BucketRunResult struct {
	Migratable      int
	Copy            progress.ItemStats
	Verify          VerifyResult
	DestOnlySkipped bool
}

// RunBucket lists live keys from the source KV bucket and copies or verifies them
// in parallel using the KV API (ListKeys → Get → Put).
func RunBucket(
	ctx context.Context,
	sourceJS jetstream.JetStream,
	bucket string,
	params BucketRunParams,
	report func(progress.ScanProgress),
	bar *progress.BucketBar,
) (BucketRunResult, error) {
	sourceKV, err := sourceJS.KeyValue(ctx, bucket)
	if err != nil {
		return BucketRunResult{}, fmt.Errorf("open source kv %q: %w", bucket, err)
	}

	lister, err := sourceKV.ListKeys(ctx)
	if err != nil {
		return BucketRunResult{}, fmt.Errorf("list source keys: %w", err)
	}

	var result BucketRunResult
	var statsMu sync.Mutex
	batch := make([]string, 0, keyBatchSize)
	lastReport := 0

	reportKeys := func(processed int) {
		if report == nil {
			return
		}
		report(progress.ScanProgress{
			Scanned:     processed,
			Unique:      processed,
			UniqueLabel: "keys",
		})
	}

	processBatch := func(keys []string) error {
		if len(keys) == 0 {
			return nil
		}

		result.Migratable += len(keys)

		if params.DryRun {
			return nil
		}

		result.Copy.Total += len(keys)

		return workpool.RunParallel(ctx, params.Workers, keys, func(ctx context.Context, key string) error {
			if bar != nil {
				bar.SetItem(key)
			}

			sourceEntry, err := sourceKV.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("get source key %q: %w", key, err)
			}
			value := bytes.Clone(sourceEntry.Value())

			switch {
			case params.VerifyOnly:
				if err := verifyKey(ctx, params.Dest, key, value, &result.Verify, &statsMu); err != nil {
					return err
				}
			case params.Dest != nil:
				if err := copyKey(ctx, params.Dest, key, value, params.SkipExisting, &result.Copy, &statsMu); err != nil {
					return err
				}
				if params.Verify {
					if err := verifyKey(ctx, params.Dest, key, value, &result.Verify, &statsMu); err != nil {
						return err
					}
				}
			}

			if bar != nil {
				bar.Add(1)
			}
			return nil
		})
	}

	for key := range lister.Keys() {
		batch = append(batch, key)
		if len(batch) >= keyBatchSize {
			if err := processBatch(batch); err != nil {
				return result, err
			}
			if result.Migratable-lastReport >= progressReportEveryKeys {
				reportKeys(result.Migratable)
				lastReport = result.Migratable
			}
			batch = batch[:0]
		}
	}

	if err := processBatch(batch); err != nil {
		return result, err
	}
	reportKeys(result.Migratable)

	if params.Verify && !params.DryRun {
		result.Verify.Expected = result.Migratable
	}

	if shouldCheckDestOnly(params, result.Migratable) {
		if err := verifyDestOnlyListKeys(ctx, sourceKV, params.Dest, &result.Verify); err != nil {
			return result, err
		}
	} else if params.Verify && result.Migratable > destOnlyVerifyMaxKeys {
		result.DestOnlySkipped = true
	}

	return result, nil
}

func shouldCheckDestOnly(params BucketRunParams, migratable int) bool {
	return params.Verify && !params.DryRun && !params.VerifyOnly &&
		migratable <= destOnlyVerifyMaxKeys && params.Dest != nil
}

func copyKey(
	ctx context.Context,
	dest jetstream.KeyValue,
	key string,
	value []byte,
	skipExisting bool,
	stats *progress.ItemStats,
	mu *sync.Mutex,
) error {
	if skipExisting {
		if _, err := dest.Get(ctx, key); err == nil {
			mu.Lock()
			stats.Skipped++
			mu.Unlock()
			return nil
		} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
			return fmt.Errorf("check destination key %q: %w", key, err)
		}
	}

	if _, err := dest.Put(ctx, key, value); err != nil {
		return fmt.Errorf("put key %q: %w", key, err)
	}

	mu.Lock()
	stats.Migrated++
	mu.Unlock()
	return nil
}

func verifyKey(
	ctx context.Context,
	dest jetstream.KeyValue,
	key string,
	want []byte,
	verify *VerifyResult,
	mu *sync.Mutex,
) error {
	destEntry, err := dest.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			mu.Lock()
			verify.Missing++
			verify.MissingKeys = append(verify.MissingKeys, key)
			mu.Unlock()
			return nil
		}
		return fmt.Errorf("verify dest key %q: %w", key, err)
	}

	mu.Lock()
	if !bytes.Equal(want, destEntry.Value()) {
		verify.Mismatch++
		verify.MismatchKeys = append(verify.MismatchKeys, key)
	} else {
		verify.OK++
	}
	mu.Unlock()
	return nil
}

func verifyDestOnlyListKeys(ctx context.Context, source, dest jetstream.KeyValue, result *VerifyResult) error {
	lister, err := dest.ListKeys(ctx)
	if err != nil {
		return fmt.Errorf("list destination keys: %w", err)
	}

	for key := range lister.Keys() {
		if _, err := source.Get(ctx, key); err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				result.DestOnly++
				result.DestOnlyKeys = append(result.DestOnlyKeys, key)
				continue
			}
			return fmt.Errorf("check source key %q: %w", key, err)
		}
	}
	return nil
}
