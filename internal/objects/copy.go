package objects

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/workpool"
)

// CopyBucket copies objects from sourceOS to destOS.
func CopyBucket(
	ctx context.Context,
	copyTimeout time.Duration,
	destJS jetstream.JetStream,
	sourceOS, destOS jetstream.ObjectStore,
	objects []*jetstream.ObjectInfo,
	skipExisting bool,
	workers int,
	bar *progress.BucketBar,
) (progress.ItemStats, error) {
	stats := progress.ItemStats{Total: len(objects)}

	var dataObjects []*jetstream.ObjectInfo
	var linkObjects []*jetstream.ObjectInfo
	for _, info := range objects {
		if IsLink(info) {
			linkObjects = append(linkObjects, info)
		} else {
			dataObjects = append(dataObjects, info)
		}
	}

	if err := copyParallel(ctx, workers, dataObjects, bar, &stats, func(ctx context.Context, info *jetstream.ObjectInfo) copyResult {
		bar.SetItem(info.Name)
		return copyDataObject(ctx, copyTimeout, sourceOS, destOS, info, skipExisting)
	}); err != nil {
		return stats, err
	}

	if err := copyParallel(ctx, workers, linkObjects, bar, &stats, func(ctx context.Context, info *jetstream.ObjectInfo) copyResult {
		bar.SetItem(info.Name + " (link)")
		return copyLinkObject(ctx, destJS, destOS, info, skipExisting)
	}); err != nil {
		return stats, err
	}

	bar.ClearItem()
	return stats, nil
}

type copyResult struct {
	migrated bool
	skipped  bool
	failed   bool
}

func copyParallel(
	ctx context.Context,
	workers int,
	objects []*jetstream.ObjectInfo,
	bar *progress.BucketBar,
	stats *progress.ItemStats,
	fn func(ctx context.Context, info *jetstream.ObjectInfo) copyResult,
) error {
	if len(objects) == 0 {
		return nil
	}

	var statsMu sync.Mutex
	return workpool.RunParallel(ctx, workers, objects, func(ctx context.Context, info *jetstream.ObjectInfo) error {
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

func copyDataObject(ctx context.Context, copyTimeout time.Duration, sourceOS, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo, skipExisting bool) copyResult {
	if skipExisting {
		if _, err := destOS.GetInfo(ctx, info.Name); err == nil {
			return copyResult{skipped: true}
		} else if !errors.Is(err, jetstream.ErrObjectNotFound) {
			return copyResult{failed: true}
		}
	}

	objCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), copyTimeout)
	defer cancel()

	if err := copyObject(objCtx, sourceOS, destOS, info); err != nil {
		return copyResult{failed: true}
	}

	return copyResult{migrated: true}
}

func copyLinkObject(ctx context.Context, destJS jetstream.JetStream, destOS jetstream.ObjectStore, info *jetstream.ObjectInfo, skipExisting bool) copyResult {
	if skipExisting {
		if existing, err := destOS.GetInfo(ctx, info.Name); err == nil && IsLink(existing) {
			return copyResult{skipped: true}
		} else if err != nil && !errors.Is(err, jetstream.ErrObjectNotFound) {
			return copyResult{failed: true}
		}
	}

	if err := copyLink(ctx, destJS, destOS, info); err != nil {
		return copyResult{failed: true}
	}

	return copyResult{migrated: true}
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
