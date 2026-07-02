package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migrate"
)

func main() {
	cfg := migrate.ParseObjectFlags("migrate-nats-objects")
	progress := migrate.NewProgress(!cfg.NoProgress)
	copyTimeout := migrate.ObjectCopyTimeout(cfg.RequestTimeout)
	migrate.PrintHeader("Object store migration")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sourceNC, sourceJS, err := migrate.Connect(cfg.SourceURL, cfg.SourceCreds, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("source: %v", err)
	}
	defer sourceNC.Close()

	destNC, destJS, err := migrate.Connect(cfg.DestURL, cfg.DestCreds, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("destination: %v", err)
	}
	defer destNC.Close()

	buckets, err := listObjectBuckets(ctx, sourceJS, cfg.BaseConfig)
	if err != nil {
		log.Fatalf("list object stores: %v", err)
	}

	summary := migrate.MigrationSummary{DryRun: cfg.DryRun}
	exitCode := 0

	for i, status := range buckets {
		bucket := status.Bucket()

		scan := progress.StartIndeterminate("Object store", bucket, i+1, len(buckets), "scanning meta stream")
		snap, err := migrate.SnapshotObjectsFromStream(ctx, sourceJS, bucket, scan.ReportObjectScan)
		if err != nil {
			scan.FinishMessage(fmt.Sprintf("  ✗ Object store %s (%d/%d) — failed to scan bucket: %v", bucket, i+1, len(buckets), err))
			exitCode = 1
			continue
		}

		metaOmitted := len(snap.Omitted)
		summary.Omitted += metaOmitted

		if cfg.DryRun {
			scan.FinishMessage(fmt.Sprintf(
				"  · Object store %s (%d/%d) — %d listed, %d meta-active, %d meta-omitted (%d meta messages)",
				bucket, i+1, len(buckets), len(snap.Listed), len(snap.Migratable), metaOmitted, snap.MessageCount,
			))
			summary.Buckets++
			summary.Migratable += len(snap.Migratable)
			continue
		}

		destOS, err := destJS.ObjectStore(ctx, bucket)
		if err != nil {
			scan.FinishMessage(fmt.Sprintf("  ✗ Object store %s (%d/%d) — destination bucket not found: %v", bucket, i+1, len(buckets), err))
			exitCode = 1
			continue
		}

		sourceOS, err := sourceJS.ObjectStore(ctx, bucket)
		if err != nil {
			scan.FinishMessage(fmt.Sprintf("  ✗ Object store %s (%d/%d) — failed to open source: %v", bucket, i+1, len(buckets), err))
			exitCode = 1
			continue
		}

		migratable, probeOmitted := migrate.FilterRetrievableObjects(ctx, sourceOS, snap.Migratable)
		summary.Omitted += len(probeOmitted)

		scan.FinishMessage(fmt.Sprintf(
			"  · Object store %s (%d/%d) — %d listed, %d migratable, %d omitted (%d meta messages)",
			bucket, i+1, len(buckets), len(snap.Listed), len(migratable), metaOmitted+len(probeOmitted), snap.MessageCount,
		))

		summary.Migratable += len(migratable)

		bar := progress.StartBucket("Object store", bucket, i+1, len(buckets), len(migratable), cfg.Workers)
		stats, err := migrateObjectBucket(ctx, copyTimeout, destJS, sourceOS, destOS, migratable, cfg.SkipExisting, cfg.Workers, bar)
		if err != nil {
			bar.FinishMessage(fmt.Sprintf("  ✗ Object store %s (%d/%d) — migration failed: %v", bucket, i+1, len(buckets), err))
			exitCode = 1
			continue
		}
		bar.Finish(stats)

		summary.Buckets++
		summary.Migrated += stats.Migrated
		summary.Skipped += stats.Skipped
		summary.Failed += stats.Failed

		if stats.Migrated+stats.Skipped+stats.Failed != stats.Total {
			fmt.Fprintf(log.Writer(),
				"  ✗ Object store %s — expected %d processed, got migrated=%d skipped=%d failed=%d\n",
				bucket, stats.Total, stats.Migrated, stats.Skipped, stats.Failed,
			)
			exitCode = 1
		}
		if stats.Failed > 0 {
			exitCode = 1
		}
	}

	migrate.PrintSummary("object store", summary)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func listObjectBuckets(ctx context.Context, js jetstream.JetStream, cfg migrate.BaseConfig) ([]jetstream.ObjectStoreStatus, error) {
	lister := js.ObjectStores(ctx)
	var buckets []jetstream.ObjectStoreStatus
	for status := range lister.Status() {
		if cfg.ShouldMigrateBucket(status.Bucket()) {
			buckets = append(buckets, status)
		}
	}
	return buckets, lister.Error()
}

func migrateObjectBucket(
	ctx context.Context,
	copyTimeout time.Duration,
	destJS jetstream.JetStream,
	sourceOS, destOS jetstream.ObjectStore,
	objects []*jetstream.ObjectInfo,
	skipExisting bool,
	workers int,
	bar *migrate.BucketBar,
) (migrate.ItemStats, error) {
	stats := migrate.ItemStats{Total: len(objects)}

	var dataObjects []*jetstream.ObjectInfo
	var linkObjects []*jetstream.ObjectInfo
	for _, info := range objects {
		if migrate.ObjectIsLink(info) {
			linkObjects = append(linkObjects, info)
		} else {
			dataObjects = append(dataObjects, info)
		}
	}

	if err := migrateObjectsParallel(ctx, workers, dataObjects, bar, &stats, func(ctx context.Context, info *jetstream.ObjectInfo) objectResult {
		bar.SetItem(info.Name)
		return migrateDataObject(ctx, copyTimeout, sourceOS, destOS, info, skipExisting)
	}); err != nil {
		return stats, err
	}

	if err := migrateObjectsParallel(ctx, workers, linkObjects, bar, &stats, func(ctx context.Context, info *jetstream.ObjectInfo) objectResult {
		bar.SetItem(info.Name + " (link)")
		return migrateLinkObject(ctx, destJS, destOS, info, skipExisting)
	}); err != nil {
		return stats, err
	}

	bar.ClearItem()
	return stats, nil
}

type objectResult struct {
	migrated bool
	skipped  bool
	failed   bool
}

func migrateObjectsParallel(
	ctx context.Context,
	workers int,
	objects []*jetstream.ObjectInfo,
	bar *migrate.BucketBar,
	stats *migrate.ItemStats,
	fn func(ctx context.Context, info *jetstream.ObjectInfo) objectResult,
) error {
	if len(objects) == 0 {
		return nil
	}

	var statsMu sync.Mutex
	return migrate.RunParallel(ctx, workers, objects, func(ctx context.Context, info *jetstream.ObjectInfo) error {
		result := fn(ctx, info)

		statsMu.Lock()
		switch {
		case result.failed:
			stats.Failed++
		case result.skipped:
			stats.Skipped++
		case result.migrated:
			stats.Migrated++
		}
		statsMu.Unlock()

		bar.Add(1)
		return nil
	})
}

func migrateDataObject(ctx context.Context, copyTimeout time.Duration, sourceOS, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo, skipExisting bool) objectResult {
	if skipExisting {
		if _, err := destOS.GetInfo(ctx, info.Name); err == nil {
			return objectResult{skipped: true}
		} else if !errors.Is(err, jetstream.ErrObjectNotFound) {
			return objectResult{failed: true}
		}
	}

	objCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), copyTimeout)
	defer cancel()

	if err := copyObject(objCtx, sourceOS, destOS, info); err != nil {
		return objectResult{failed: true}
	}

	return objectResult{migrated: true}
}

func migrateLinkObject(ctx context.Context, destJS jetstream.JetStream, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo, skipExisting bool) objectResult {
	if skipExisting {
		if existing, err := destOS.GetInfo(ctx, info.Name); err == nil && migrate.ObjectIsLink(existing) {
			return objectResult{skipped: true}
		} else if err != nil && !errors.Is(err, jetstream.ErrObjectNotFound) {
			return objectResult{failed: true}
		}
	}

	if err := copyLink(ctx, destJS, destOS, info); err != nil {
		return objectResult{failed: true}
	}

	return objectResult{migrated: true}
}

func copyObject(ctx context.Context, sourceOS, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo) error {
	result, err := sourceOS.Get(ctx, info.Name)
	if err != nil {
		return err
	}
	defer result.Close()

	meta := jetstream.ObjectMeta{
		Name:        info.Name,
		Description: info.Description,
		Headers:     info.Headers,
		Metadata:    info.Metadata,
	}

	if _, err := destOS.Put(ctx, meta, result); err != nil {
		return fmt.Errorf("put object %q: %w", info.Name, err)
	}
	return nil
}

func copyLink(ctx context.Context, destJS jetstream.JetStream, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo) error {
	if info.Opts == nil || info.Opts.Link == nil {
		return fmt.Errorf("object %q marked as link but has no link metadata", info.Name)
	}

	link := info.Opts.Link
	if link.Name == "" {
		targetOS, err := destJS.ObjectStore(ctx, link.Bucket)
		if err != nil {
			return fmt.Errorf("open destination bucket for link %q: %w", info.Name, err)
		}
		if _, err := destOS.AddBucketLink(ctx, info.Name, targetOS); err != nil {
			return fmt.Errorf("add bucket link %q -> %q: %w", info.Name, link.Bucket, err)
		}
		return nil
	}

	target := &jetstream.ObjectInfo{
		Bucket: link.Bucket,
		ObjectMeta: jetstream.ObjectMeta{
			Name: link.Name,
		},
	}
	if _, err := destOS.AddLink(ctx, info.Name, target); err != nil {
		return fmt.Errorf("add link %q -> %s/%s: %w", info.Name, link.Bucket, link.Name, err)
	}
	return nil
}
