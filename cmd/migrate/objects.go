package migrate

import (
	"fmt"
	"log"

	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/objects"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/spf13/cobra"
)

var objectsCmd = &cobra.Command{
	Use:   "objects",
	Short: "Copy object store buckets between clusters",
	RunE: func(*cobra.Command, []string) error {
		base, err := sharedBaseConfig()
		if err != nil {
			return err
		}
		return runObjects(migration.NewObjectConfig(base))
	},
}

func init() {
	migrateCmd.AddCommand(objectsCmd)
}

func runObjects(cfg migration.ObjectConfig) error {
	session := progress.NewSession(!cfg.NoProgress, "Object store migration")
	session.Status("Connecting...")
	copyTimeout := objects.CopyTimeout(cfg.RequestTimeout)

	clusters, err := connectClusters(cfg.BaseConfig)
	if err != nil {
		return err
	}
	defer clusters.Close()

	buckets, err := objects.ListBuckets(clusters.Ctx, clusters.SourceJS, cfg.BaseConfig)
	if err != nil {
		return fmt.Errorf("list object stores: %w", err)
	}

	summary := migration.Summary{DryRun: cfg.DryRun}
	exitCode := 0

	for i, status := range buckets {
		bucket := status.Bucket()
		index, total := i+1, len(buckets)

		scan := session.UI.StartIndeterminate("Object store", bucket, index, total, "scanning meta stream")
		snap, err := objects.SnapshotFromStream(clusters.Ctx, clusters.SourceJS, bucket, scan.ReportScan)
		if err != nil {
			scan.FinishMessage(objects.ScanFailMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}

		metaOmitted := len(snap.Omitted)
		summary.Omitted += metaOmitted

		if cfg.DryRun {
			scan.FinishMessage(objects.DryRunScanMessage(bucket, index, total, snap, metaOmitted))
			summary.Buckets++
			summary.Migratable += len(snap.Migratable)
			continue
		}

		destOS, err := clusters.DestJS.ObjectStore(clusters.Ctx, bucket)
		if err != nil {
			scan.FinishMessage(objects.DestBucketMissingMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}

		sourceOS, err := clusters.SourceJS.ObjectStore(clusters.Ctx, bucket)
		if err != nil {
			scan.FinishMessage(objects.SourceOpenFailMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}

		migratable, probeOmitted := objects.FilterRetrievableObjects(clusters.Ctx, sourceOS, snap.Migratable)
		summary.Omitted += len(probeOmitted)

		scan.FinishMessage(objects.ScanOKMessage(
			bucket, index, total,
			len(snap.Listed), len(migratable), metaOmitted+len(probeOmitted), snap.MessageCount,
		))

		summary.Migratable += len(migratable)

		bar := session.UI.StartBucket("Object store", bucket, index, total, len(migratable), cfg.Workers)
		stats, err := objects.CopyBucket(clusters.Ctx, copyTimeout, clusters.DestJS, sourceOS, destOS, migratable, cfg.SkipExisting, cfg.Workers, bar)
		if err != nil {
			bar.FinishMessage(objects.CopyFailMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}
		bar.Finish(stats)

		summary.Buckets++
		summary.Migrated += stats.Migrated
		summary.Skipped += stats.Skipped
		summary.Failed += stats.Failed

		if stats.Migrated+stats.Skipped+stats.Failed != stats.Total {
			fmt.Fprintln(log.Writer(), objects.CopyCountMismatchMessage(bucket, stats.Total, stats.Migrated, stats.Skipped, stats.Failed))
			exitCode = 1
		}
		if stats.Failed > 0 {
			exitCode = 1
		}
	}

	return migration.CompleteRun("object store", summary, exitCode)
}
