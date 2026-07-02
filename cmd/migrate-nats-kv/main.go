package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migrate"
)

func main() {
	cfg := migrate.ParseKVFlags("migrate-nats-kv")
	progress := migrate.NewProgress(!cfg.NoProgress)

	title := "KV migration"
	if cfg.VerifyOnly {
		title = "KV verification"
	}
	migrate.PrintHeader(title)

	fmt.Fprintln(os.Stderr, "Connecting to source...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sourceNC, sourceJS, err := migrate.Connect(cfg.SourceURL, cfg.SourceCreds, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("source: %v", err)
	}
	defer sourceNC.Close()

	fmt.Fprintln(os.Stderr, "Connecting to destination...")
	destNC, destJS, err := migrate.Connect(cfg.DestURL, cfg.DestCreds, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("destination: %v", err)
	}
	defer destNC.Close()

	buckets, err := listKVBuckets(ctx, sourceJS, cfg.BaseConfig)
	if err != nil {
		log.Fatalf("list KV buckets: %v", err)
	}

	summary := migrate.MigrationSummary{
		DryRun:     cfg.DryRun,
		VerifyOnly: cfg.VerifyOnly,
	}
	exitCode := 0

	for i, status := range buckets {
		bucket := status.Bucket()

		listScan := progress.StartIndeterminate("KV", bucket, i+1, len(buckets), "scanning stream")
		snap, err := migrate.SnapshotKVFromStream(ctx, sourceJS, bucket, listScan.ReportStreamScan)
		if err != nil {
			listScan.FinishMessage(fmt.Sprintf("  ✗ KV %s (%d/%d) — failed to scan bucket: %v", bucket, i+1, len(buckets), err))
			exitCode = 1
			continue
		}
		listScan.FinishMessage(fmt.Sprintf(
			"  · KV %s (%d/%d) — %d listed, %d migratable, %d omitted (%d stream messages)",
			bucket, i+1, len(buckets), len(snap.Listed), len(snap.Migratable), len(snap.Omitted), snap.MessageCount,
		))

		summary.Migratable += len(snap.Migratable)
		summary.Omitted += len(snap.Omitted)

		if cfg.DryRun {
			summary.Buckets++
			continue
		}

		destKV, err := destJS.KeyValue(ctx, bucket)
		if err != nil {
			fmt.Fprintf(log.Writer(), "  ✗ KV %s (%d/%d) — destination bucket not found: %v\n", bucket, i+1, len(buckets), err)
			exitCode = 1
			continue
		}

		if !cfg.VerifyOnly {
			bar := progress.StartBucket("KV", bucket, i+1, len(buckets), len(snap.Migratable), cfg.Workers)
			stats, err := migrateKVBucket(ctx, destKV, snap.Migratable, snap.Values, cfg.SkipExisting, cfg.Workers, bar)
			if err != nil {
				bar.FinishMessage(fmt.Sprintf("  ✗ KV %s (%d/%d) — migration failed: %v", bucket, i+1, len(buckets), err))
				exitCode = 1
				continue
			}
			bar.Finish(stats)

			summary.Migrated += stats.Migrated
			summary.Skipped += stats.Skipped

			if !cfg.SkipExisting && stats.Migrated != len(snap.Migratable) {
				fmt.Fprintf(log.Writer(),
					"  ✗ KV %s — expected %d migrated, got %d (skipped=%d)\n",
					bucket, len(snap.Migratable), stats.Migrated, stats.Skipped,
				)
				exitCode = 1
			}
		}

		if cfg.Verify {
			verifyScan := progress.StartIndeterminate("KV", bucket, i+1, len(buckets), fmt.Sprintf("verifying %d keys", len(snap.Migratable)))
			verify, err := migrate.VerifyKVMigratable(ctx, destJS, bucket, destKV, snap.Migratable, snap.Values, cfg.Workers, verifyScan.ReportVerify)
			if err != nil {
				verifyScan.FinishMessage(fmt.Sprintf("  ✗ KV %s (%d/%d) — verify failed: %v", bucket, i+1, len(buckets), err))
				exitCode = 1
				continue
			}
			verifyScan.FinishMessage("")
			migrate.PrintKVVerifyReport(bucket, verify)

			summary.VerifyRan = true
			summary.VerifyOK += verify.OK
			summary.VerifyFailed += verify.Issues()
			summary.DestOnly += verify.DestOnly

			if cfg.FailuresFile != "" && (verify.Issues() > 0 || verify.DestOnly > 0) {
				if err := migrate.WriteKVFailuresFile(cfg.FailuresFile, bucket, verify); err != nil {
					fmt.Fprintf(log.Writer(), "  ✗ KV %s — failed to write failures file: %v\n", bucket, err)
					exitCode = 1
				}
			}

			if !verify.Passed() {
				exitCode = 1
			}
		}

		summary.Buckets++
	}

	migrate.PrintSummary("KV", summary)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func listKVBuckets(ctx context.Context, js jetstream.JetStream, cfg migrate.BaseConfig) ([]jetstream.KeyValueStatus, error) {
	lister := js.KeyValueStores(ctx)
	var buckets []jetstream.KeyValueStatus
	for status := range lister.Status() {
		if cfg.ShouldMigrateBucket(status.Bucket()) {
			buckets = append(buckets, status)
		}
	}
	return buckets, lister.Error()
}

func migrateKVBucket(ctx context.Context, dest jetstream.KeyValue, keys []string, values map[string][]byte, skipExisting bool, workers int, bar *migrate.BucketBar) (migrate.ItemStats, error) {
	stats := migrate.ItemStats{Total: len(keys)}
	if len(keys) == 0 {
		return stats, nil
	}

	var statsMu sync.Mutex
	err := migrate.RunParallel(ctx, workers, keys, func(ctx context.Context, key string) error {
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
