package backup

import (
	"fmt"
	"os"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
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
	fmt.Fprintln(os.Stderr, "\nKV backup")
	logStatus("Connecting...")

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

		logStatus(fmt.Sprintf("  KV %s (%d/%d) — backing up stream %s", bucket, index, total, kv.StreamName(bucket)))

		var report kv.ProgressWriter
		if !cfg.NoProgress {
			report = func(format string, args ...any) {
				logStatus(fmt.Sprintf(format, args...))
			}
		}

		result, err := kv.BackupBucket(ctx, mgr, bucket, outDir, !cfg.NoProgress, report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ KV %s (%d/%d) — failed: %v\n", bucket, index, total, err)
			exitCode = 1
			continue
		}

		fmt.Fprintf(os.Stderr,
			"  ✓ KV %s (%d/%d) — %d messages, %d bytes → %s\n",
			bucket, index, total, result.Messages, result.Bytes, result.Dir,
		)
		completed++
	}

	fmt.Fprintf(os.Stderr, "\nKV backup complete: %d/%d buckets under %s\n", completed, len(buckets), shared.dir)
	if exitCode != 0 {
		return &migration.ExitError{Code: exitCode}
	}
	return nil
}
