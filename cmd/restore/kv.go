package restore

import (
	"fmt"
	"path/filepath"

	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/report"
	"github.com/spf13/cobra"
)

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Restore KV buckets from JetStream stream snapshots",
	RunE: func(*cobra.Command, []string) error {
		cfg, err := endpointConfig()
		if err != nil {
			return err
		}
		return runKVRestore(cfg)
	},
}

func init() {
	restoreCmd.AddCommand(kvCmd)
}

func runKVRestore(cfg migration.EndpointConfig) error {
	session := progress.NewSession(progress.SessionConfig{
		Title:      "KV restore",
		NoProgress: cfg.NoProgress,
	})
	session.Status("Connecting...")

	nc, mgr, err := nats.ConnectJSM(cfg.URL, cfg.Creds, cfg.RequestTimeout)
	if err != nil {
		return err
	}
	defer nc.Close()

	dirs, err := kv.DiscoverBackupDirs(shared.dir)
	if err != nil {
		return err
	}

	dirs, err = kv.FilterBackupDirs(dirs, cfg.ShouldIncludeBucket)
	if err != nil {
		return err
	}

	ctx := nats.RunContext()

	flags := []string{}
	if shared.force {
		flags = append(flags, "--force")
	}
	if shared.replicas > 0 {
		flags = append(flags, fmt.Sprintf("--replicas=%d", shared.replicas))
	}
	if cfg.NoProgress {
		flags = append(flags, "--no-progress")
	}

	session.PrintPlan([]progress.PlanEntry{
		{Label: "Context", Value: shared.context},
		{Label: "Source", Value: shared.dir},
		{Label: "Buckets", Value: progress.FormatBucketCount(len(dirs), shared.bucket)},
		{Label: "Flags", Value: progress.JoinFlags(flags...)},
	})

	exitCode := 0
	completed := 0

	for i, dir := range dirs {
		index, total := i+1, len(dirs)

		dataSize, err := kv.DataFileSize(dir)
		if err != nil {
			session.BucketFail(report.KindKV, dirLabel(dir), index, total, "invalid backup", err)
			exitCode = 1
			continue
		}

		meta, err := kv.ReadBackupMetadata(dir)
		if err != nil {
			session.BucketFail(report.KindKV, dirLabel(dir), index, total, "invalid backup", err)
			exitCode = 1
			continue
		}
		bucket, _ := kv.BucketFromStreamName(meta.Config.Name)

		session.BeginBucket()
		transfer := session.UI.StartTransferTracked(report.KindKV, bucket, index, total, "restoring", dataSize)
		var reportFn kv.TransferReporter
		if !cfg.NoProgress {
			reportFn = func(p kv.TransferProgress) {
				totalBytes := p.Total
				if totalBytes <= 0 {
					totalBytes = dataSize
				}
				transfer.Report(p.Sent, totalBytes)
			}
		}

		result, err := kv.RestoreBucket(ctx, mgr, dir, shared.force, shared.replicas, !cfg.NoProgress, reportFn)
		transfer.Finish()
		if err != nil {
			session.BucketFail(report.KindKV, bucket, index, total, "restore failed", err)
			exitCode = 1
			continue
		}

		session.BucketSuccessStats(report.KindKV, result.Bucket, index, total,
			fmt.Sprintf("%d messages, %d bytes restored from %s", result.Messages, result.Bytes, dir),
			progress.BucketStats{Items: int64(result.Messages), Bytes: int64(result.Bytes)},
		)
		completed++
	}

	session.Completef(exitCode, "KV restore complete: %d/%d buckets from %s", completed, len(dirs), shared.dir)
	if exitCode != 0 {
		return &migration.ExitError{Code: exitCode}
	}
	return nil
}

func dirLabel(dir string) string {
	if base := filepath.Base(dir); base != "" && base != "." {
		return base
	}
	return dir
}
