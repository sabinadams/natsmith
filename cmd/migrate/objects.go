package migrate

import (
	"fmt"

	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/objects"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/report"
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
	session := progress.NewSession(progress.SessionConfig{
		Title:      "Object store migration",
		NoProgress: cfg.NoProgress,
	})
	copyTimeout := objects.CopyTimeout(cfg.RequestTimeout)

	clusters, err := migration.ConnectClusters(cfg.BaseConfig, session.Status)
	if err != nil {
		return err
	}
	defer clusters.Close()

	buckets, err := objects.ListBuckets(clusters.Ctx, clusters.SourceJS, cfg.BaseConfig)
	if err != nil {
		return fmt.Errorf("list object stores: %w", err)
	}

	session.PrintPlan([]progress.PlanEntry{
		{Label: "Source", Value: shared.sourceContext},
		{Label: "Dest", Value: shared.destContext},
		{Label: "Buckets", Value: progress.FormatBucketCount(len(buckets), shared.bucket)},
		{Label: "Flags", Value: progress.JoinFlags(migratePlanFlags(cfg.BaseConfig, false, false, "")...)},
	})

	summary := migration.Summary{DryRun: cfg.DryRun}
	exitCode := 0

	for i, status := range buckets {
		if session.Interrupted() {
			exitCode = 1
			break
		}

		bucket := status.Bucket()
		index, total := i+1, len(buckets)
		session.BeginBucket()

		scan := session.UI.StartIndeterminate("Object store", bucket, index, total, "scanning meta stream")
		snap, err := objects.SnapshotFromStream(clusters.Ctx, clusters.SourceJS, bucket, scan.ReportScan)
		if err != nil {
			scan.Close()
			session.BucketFail(report.KindObjectStore, bucket, index, total, "failed to scan bucket", err)
			exitCode = 1
			continue
		}

		metaOmitted := len(snap.Omitted)
		summary.Omitted += metaOmitted

		if cfg.DryRun {
			scan.Close()
			session.BucketInfo(report.KindObjectStore, bucket, index, total, fmt.Sprintf(
				"%d listed, %d meta-active, %d meta-omitted (%d meta messages)",
				len(snap.Listed), len(snap.Migratable), metaOmitted, snap.MessageCount,
			))
			summary.Buckets++
			summary.Migratable += len(snap.Migratable)
			continue
		}

		destOS, err := clusters.DestJS.ObjectStore(clusters.Ctx, bucket)
		if err != nil {
			scan.Close()
			session.BucketFail(report.KindObjectStore, bucket, index, total, "destination bucket not found", err)
			exitCode = 1
			continue
		}

		sourceOS, err := clusters.SourceJS.ObjectStore(clusters.Ctx, bucket)
		if err != nil {
			scan.Close()
			session.BucketFail(report.KindObjectStore, bucket, index, total, "failed to open source", err)
			exitCode = 1
			continue
		}

		migratable, probeOmitted := objects.FilterRetrievableObjects(clusters.Ctx, sourceOS, snap.Migratable)
		summary.Omitted += len(probeOmitted)

		scan.Close()
		session.BucketInfo(report.KindObjectStore, bucket, index, total, fmt.Sprintf(
			"%d listed, %d migratable, %d omitted (%d meta messages)",
			len(snap.Listed), len(migratable), metaOmitted+len(probeOmitted), snap.MessageCount,
		))

		summary.Migratable += len(migratable)

		bar := session.UI.StartBucket("Object store", bucket, index, total, len(migratable), cfg.Workers)
		stats, err := objects.CopyBucket(clusters.Ctx, copyTimeout, clusters.DestJS, sourceOS, destOS, migratable, cfg.SkipExisting, cfg.Workers, bar)
		bar.Close()
		if err != nil {
			session.BucketFail(report.KindObjectStore, bucket, index, total, "migration failed", err)
			exitCode = 1
			continue
		}

		session.BucketCopied(report.KindObjectStore, bucket, index, total, stats)

		summary.Buckets++
		summary.Migrated += stats.Migrated
		summary.Skipped += stats.Skipped
		summary.Failed += stats.Failed

		if stats.Migrated+stats.Skipped+stats.Failed != stats.Total {
			session.BucketIssue(report.KindObjectStore, bucket, fmt.Sprintf(
				"expected %d processed, got migrated=%d skipped=%d failed=%d",
				stats.Total, stats.Migrated, stats.Skipped, stats.Failed,
			))
			exitCode = 1
		}
		if stats.Failed > 0 {
			exitCode = 1
		}
	}

	if session.Interrupted() {
		exitCode = 1
	}
	return migration.CompleteRun("object store", summary, exitCode, session)
}
