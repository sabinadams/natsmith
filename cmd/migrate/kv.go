package migrate

import (
	"fmt"
	"log"
	"os"

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

	clusters, err := migration.ConnectClusters(cfg.BaseConfig, func(msg string) {
		fmt.Fprintln(os.Stderr, msg)
	})
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

		listScan := ui.StartIndeterminate("KV", bucket, index, total, "scanning stream")
		snap, err := kv.SnapshotFromStream(clusters.Ctx, clusters.SourceJS, bucket, listScan.ReportScan)
		if err != nil {
			listScan.FinishMessage(kv.ScanFailMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}
		listScan.FinishMessage(kv.ScanOKMessage(bucket, index, total, snap))

		summary.Migratable += len(snap.Migratable)
		summary.Omitted += len(snap.Omitted)

		if cfg.DryRun {
			summary.Buckets++
			continue
		}

		destKV, err := clusters.DestJS.KeyValue(clusters.Ctx, bucket)
		if err != nil {
			fmt.Fprintln(log.Writer(), kv.DestBucketMissingMessage(bucket, index, total, err))
			exitCode = 1
			continue
		}

		if !cfg.VerifyOnly {
			bar := ui.StartBucket("KV", bucket, index, total, len(snap.Migratable), cfg.Workers)
			stats, err := kv.CopyBucket(clusters.Ctx, destKV, snap.Migratable, snap.Values, cfg.SkipExisting, cfg.Workers, bar)
			if err != nil {
				bar.FinishMessage(kv.CopyFailMessage(bucket, index, total, err))
				exitCode = 1
				continue
			}
			bar.Finish(stats)

			summary.Migrated += stats.Migrated
			summary.Skipped += stats.Skipped

			if !cfg.SkipExisting && stats.Migrated != len(snap.Migratable) {
				fmt.Fprintln(log.Writer(), kv.CopyCountMismatchMessage(bucket, len(snap.Migratable), stats.Migrated, stats.Skipped))
				exitCode = 1
			}
		}

		if cfg.Verify {
			verifyScan := ui.StartIndeterminate("KV", bucket, index, total, fmt.Sprintf("verifying %d keys", len(snap.Migratable)))
			verify, err := kv.VerifyMigratable(clusters.Ctx, clusters.DestJS, bucket, destKV, snap.Migratable, snap.Values, cfg.Workers, verifyScan.ReportVerify)
			if err != nil {
				verifyScan.FinishMessage(kv.VerifyFailMessage(bucket, index, total, err))
				exitCode = 1
				continue
			}
			verifyScan.FinishMessage("")
			kv.PrintVerifyReport(bucket, verify)

			summary.VerifyRan = true
			summary.VerifyOK += verify.OK
			summary.VerifyFailed += verify.Issues()
			summary.DestOnly += verify.DestOnly

			if cfg.FailuresFile != "" && (verify.Issues() > 0 || verify.DestOnly > 0) {
				if err := kv.WriteFailuresFile(cfg.FailuresFile, bucket, verify); err != nil {
					fmt.Fprintln(log.Writer(), kv.FailuresFileErrorMessage(bucket, err))
					exitCode = 1
				}
			}

			if !verify.Passed() {
				exitCode = 1
			}
		}

		summary.Buckets++
	}

	return migration.CompleteRun("KV", summary, exitCode)
}
