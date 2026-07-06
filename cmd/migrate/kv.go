package migrate

import (
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/report"
	"github.com/spf13/cobra"
)

type kvFlags struct {
	verify       bool
	verifyOnly   bool
	failuresFile string
}

var kvOpts kvFlags

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Copy KV buckets between clusters and verify",
	RunE: func(*cobra.Command, []string) error {
		base, err := sharedBaseConfig()
		if err != nil {
			return err
		}
		cfg := migration.NewKVConfig(base, kvOpts.verify, kvOpts.verifyOnly, kvOpts.failuresFile)
		return runKV(cfg)
	},
}

func init() {
	migrateCmd.AddCommand(kvCmd)

	flags := kvCmd.Flags()
	flags.BoolVar(&kvOpts.verify, "verify", true, "verify destination keys match source after migration")
	flags.BoolVar(&kvOpts.verifyOnly, "verify-only", false, "verify only — compare source and destination without writing")
	flags.StringVar(&kvOpts.failuresFile, "failures-file", "", "append verification failures to this file (bucket, key, issue per line)")
}

func runKV(cfg migration.KVConfig) error {
	title := "KV migration"
	if cfg.VerifyOnly {
		title = "KV verification"
	}
	session := progress.NewSession(progress.SessionConfig{
		Title:      title,
		NoProgress: cfg.NoProgress,
	})

	clusters, err := migration.ConnectClusters(cfg.BaseConfig, session.Status)
	if err != nil {
		return err
	}
	defer clusters.Close()

	buckets, err := kv.ListBuckets(clusters.Ctx, clusters.SourceJS, cfg.BaseConfig)
	if err != nil {
		return fmt.Errorf("list KV buckets: %w", err)
	}

	flags := migratePlanFlags(cfg.BaseConfig, cfg.Verify, cfg.VerifyOnly, kvOpts.failuresFile)
	session.PrintPlan([]progress.PlanEntry{
		{Label: "Source", Value: shared.sourceContext},
		{Label: "Dest", Value: shared.destContext},
		{Label: "Buckets", Value: progress.FormatBucketCount(len(buckets), shared.bucket)},
		{Label: "Flags", Value: progress.JoinFlags(flags...)},
	})

	summary := migration.Summary{
		DryRun:     cfg.DryRun,
		VerifyOnly: cfg.VerifyOnly,
	}
	exitCode := 0

	for i, status := range buckets {
		if session.Interrupted() {
			exitCode = 1
			break
		}

		bucket := status.Bucket()
		index, total := i+1, len(buckets)
		session.BeginBucket()

		scanBar := session.UI.StartIndeterminate("KV", bucket, index, total, "listing keys")

		var destKV jetstream.KeyValue
		if !cfg.DryRun {
			var err error
			destKV, err = clusters.DestJS.KeyValue(clusters.Ctx, bucket)
			if err != nil {
				scanBar.Close()
				session.BucketFail(report.KindKV, bucket, index, total, "destination bucket not found", err)
				exitCode = 1
				continue
			}
		}

		var actionBar *progress.BucketBar
		if !cfg.DryRun {
			action := "migrating"
			if cfg.VerifyOnly {
				action = "verifying"
			}
			actionBar = session.UI.StartIndeterminate("KV", bucket, index, total, action)
		}

		run, err := kv.RunBucket(
			clusters.Ctx,
			clusters.SourceJS,
			bucket,
			kv.BucketRunParams{
				DryRun:       cfg.DryRun,
				VerifyOnly:   cfg.VerifyOnly,
				SkipExisting: cfg.SkipExisting,
				Verify:       cfg.Verify,
				Workers:      cfg.Workers,
				Dest:         destKV,
			},
			scanBar.ReportScan,
			actionBar,
		)
		if err != nil {
			scanBar.Close()
			if actionBar != nil {
				actionBar.Close()
			}
			session.BucketFail(report.KindKV, bucket, index, total, "failed", err)
			exitCode = 1
			continue
		}
		scanBar.Close()

		detail := fmt.Sprintf("%d migratable keys", run.Migratable)
		if run.GhostSkipped > 0 {
			detail = fmt.Sprintf("%s (%d ghost skipped)", detail, run.GhostSkipped)
		}
		session.BucketInfoStats(report.KindKV, bucket, index, total, detail, progress.BucketStats{Items: int64(run.Migratable)})

		summary.Migratable += run.Migratable

		if cfg.DryRun {
			summary.Buckets++
			if actionBar != nil {
				actionBar.Close()
			}
			continue
		}

		if actionBar != nil {
			actionBar.Close()
		}

		if !cfg.VerifyOnly {
			session.BucketCopied(report.KindKV, bucket, index, total, run.Copy)
			summary.Migrated += run.Copy.Migrated
			summary.Skipped += run.Copy.Skipped

			if !cfg.SkipExisting && run.Copy.Migrated != run.Migratable {
				session.BucketIssue(report.KindKV, bucket,
					fmt.Sprintf("expected %d migrated, got %d (skipped=%d)", run.Migratable, run.Copy.Migrated, run.Copy.Skipped))
				exitCode = 1
			}
		}

		if cfg.Verify {
			kv.ReportVerify(session, bucket, run.Verify)
			if run.DestOnlySkipped {
				session.BucketWarning(report.KindKV, bucket,
					fmt.Sprintf("skipped dest-only scan (%d migratable keys exceeds limit)", run.Migratable))
			}

			summary.VerifyRan = true
			summary.VerifyOK += run.Verify.OK
			summary.VerifyFailed += run.Verify.Issues()
			summary.DestOnly += run.Verify.DestOnly

			if cfg.FailuresFile != "" && (run.Verify.Issues() > 0 || run.Verify.DestOnly > 0) {
				if err := kv.WriteFailuresFile(cfg.FailuresFile, bucket, run.Verify); err != nil {
					session.BucketIssue(report.KindKV, bucket, fmt.Sprintf("failed to write failures file: %v", err))
					exitCode = 1
				}
			}

			if !run.Verify.Passed() {
				exitCode = 1
			}
		}

		summary.Buckets++
	}

	if session.Interrupted() {
		exitCode = 1
	}
	return migration.CompleteRun("KV", summary, exitCode, session)
}

func migratePlanFlags(base migration.BaseConfig, verify bool, verifyOnly bool, failuresFile string) []string {
	var flags []string
	if base.DryRun {
		flags = append(flags, "--dry-run")
	}
	if base.SkipExisting {
		flags = append(flags, "--skip-existing")
	}
	if base.NoProgress {
		flags = append(flags, "--no-progress")
	}
	if base.Workers > 1 {
		flags = append(flags, fmt.Sprintf("--workers=%d", base.Workers))
	}
	if verifyOnly {
		flags = append(flags, "--verify-only")
	} else if verify {
		flags = append(flags, "--verify")
	}
	if failuresFile != "" {
		flags = append(flags, "--failures-file")
	}
	return flags
}
