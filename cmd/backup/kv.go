package backup

import (
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/report"
	"github.com/spf13/cobra"
)

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Backup KV buckets as JetStream stream snapshots",
	RunE: func(*cobra.Command, []string) error {
		cfg, err := endpointConfig()
		if err != nil {
			return err
		}
		return runKVBackup(cfg)
	},
}

func init() {
	backupCmd.AddCommand(kvCmd)
}

func runKVBackup(cfg migration.EndpointConfig) error {
	session := progress.NewSession(!cfg.NoProgress, "KV backup")
	session.Status("Connecting...")

	nc, mgr, err := nats.ConnectJSM(cfg.URL, cfg.Creds, cfg.RequestTimeout)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(cfg.RequestTimeout))
	if err != nil {
		return fmt.Errorf("create jetstream context: %w", err)
	}

	ctx := nats.RunContext()

	buckets, err := kv.ListBuckets(ctx, js, migration.BaseConfig{
		Buckets: cfg.Buckets,
		Omit:    cfg.Omit,
	})
	if err != nil {
		return fmt.Errorf("list KV buckets: %w", err)
	}

	exitCode := 0
	completed := 0
	for i, status := range buckets {
		bucket := status.Bucket()
		index, total := i+1, len(buckets)
		outDir := kv.BackupDirForBucket(shared.dir, bucket)

		transfer := session.UI.StartTransferTracked(report.KindKV, bucket, index, total, "backing up", 0)
		var reportFn kv.TransferReporter
		if !cfg.NoProgress {
			reportFn = func(p kv.TransferProgress) {
				transfer.Report(p.Sent, p.Total)
			}
		}

		result, err := kv.BackupBucket(ctx, mgr, bucket, outDir, !cfg.NoProgress, reportFn)
		transfer.Finish()
		if err != nil {
			session.BucketFail(report.KindKV, bucket, index, total, "backup failed", err)
			exitCode = 1
			continue
		}

		session.BucketSuccess(report.KindKV, bucket, index, total,
			fmt.Sprintf("%d messages, %d bytes → %s", result.Messages, result.Bytes, result.Dir),
		)
		completed++
	}

	session.Completef("KV backup complete: %d/%d buckets under %s", completed, len(buckets), shared.dir)
	if exitCode != 0 {
		return &migration.ExitError{Code: exitCode}
	}
	return nil
}
