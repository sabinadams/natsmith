package migrate

import (
	"fmt"
	"log"
	"os"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/progress"
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
	ui := progress.NewProgress(!cfg.NoProgress)

	title := "KV migration"
	if cfg.VerifyOnly {
		title = "KV verification"
	}
	progress.PrintHeader(title)

	clusters, err := connectClusters(cfg.BaseConfig)
	if err != nil {
		return err
	}
	defer clusters.Close()

	buckets, err := kv.ListBuckets(clusters.Ctx, clusters.SourceJS, cfg.BaseConfig)
	if err != nil {
		return fmt.Errorf("list KV buckets: %w", err)
	}

	summary := migration.Summary{
		DryRun:     cfg.DryRun,
		VerifyOnly: cfg.VerifyOnly,
	}
	exitCode := 0

	for i, status := range buckets {
		bucket := status.Bucket()
		index, total := i+1, len(buckets)

		scanBar := ui.StartIndeterminate("KV", bucket, index, total, "listing keys")

		var destKV jetstream.KeyValue
		if !cfg.DryRun {
			var err error
			destKV, err = clusters.DestJS.KeyValue(clusters.Ctx, bucket)
			if err != nil {
				scanBar.FinishMessage(kv.DestBucketMissingMessage(bucket, index, total, err))
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
			actionBar = ui.StartIndeterminate("KV", bucket, index, total, action)
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
			scanBar.FinishMessage(kv.ScanFailMessage(bucket, index, total, err))
			if actionBar != nil {
				actionBar.FinishMessage("")
			}
			exitCode = 1
			continue
		}
		scanBar.FinishMessage(kv.ScanOKRunMessage(bucket, index, total, run))

		summary.Migratable += run.Migratable

		if cfg.DryRun {
			summary.Buckets++
			continue
		}

		if actionBar != nil {
			if cfg.VerifyOnly {
				actionBar.FinishMessage("")
			} else {
				actionBar.Finish(run.Copy)
			}
		}

		if !cfg.VerifyOnly {
			summary.Migrated += run.Copy.Migrated
			summary.Skipped += run.Copy.Skipped

			if !cfg.SkipExisting && run.Copy.Migrated != run.Migratable {
				fmt.Fprintln(log.Writer(), kv.CopyCountMismatchMessage(bucket, run.Migratable, run.Copy.Migrated, run.Copy.Skipped))
				exitCode = 1
			}
		}

		if cfg.Verify {
			kv.PrintVerifyReport(bucket, run.Verify)
			if run.DestOnlySkipped {
				fmt.Fprintf(os.Stderr, "  ! verify %s: skipped dest-only scan (%d migratable keys exceeds limit)\n", bucket, run.Migratable)
			}

			summary.VerifyRan = true
			summary.VerifyOK += run.Verify.OK
			summary.VerifyFailed += run.Verify.Issues()
			summary.DestOnly += run.Verify.DestOnly

			if cfg.FailuresFile != "" && (run.Verify.Issues() > 0 || run.Verify.DestOnly > 0) {
				if err := kv.WriteFailuresFile(cfg.FailuresFile, bucket, run.Verify); err != nil {
					fmt.Fprintln(log.Writer(), kv.FailuresFileErrorMessage(bucket, err))
					exitCode = 1
				}
			}

			if !run.Verify.Passed() {
				exitCode = 1
			}
		}

		summary.Buckets++
	}

	return migration.CompleteRun("KV", summary, exitCode)
}
