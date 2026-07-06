package restore

import (
	"fmt"
	"os"

	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/progress"
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
	ui := progress.NewProgress(!cfg.NoProgress)
	progress.PrintHeader("KV restore")
	fmt.Fprintln(os.Stderr, "Connecting...")

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
	exitCode := 0
	completed := 0

	for i, dir := range dirs {
		index, total := i+1, len(dirs)

		dataSize, err := kv.DataFileSize(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ KV restore (%d/%d) from %s — failed: %v\n", index, total, dir, err)
			exitCode = 1
			continue
		}

		meta, err := kv.ReadBackupMetadata(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ KV restore (%d/%d) from %s — failed: %v\n", index, total, dir, err)
			exitCode = 1
			continue
		}
		bucket, _ := kv.BucketFromStreamName(meta.Config.Name)

		transfer := ui.StartTransferTracked("KV", bucket, index, total, "restoring", dataSize)
		var report kv.TransferReporter
		if !cfg.NoProgress {
			report = func(p kv.TransferProgress) {
				totalBytes := p.Total
				if totalBytes <= 0 {
					totalBytes = dataSize
				}
				transfer.Report(p.Sent, totalBytes)
			}
		}

		result, err := kv.RestoreBucket(ctx, mgr, dir, shared.force, shared.replicas, !cfg.NoProgress, report)
		transfer.Finish()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ KV restore (%d/%d) from %s — failed: %v\n", index, total, dir, err)
			exitCode = 1
			continue
		}

		fmt.Fprintf(os.Stderr,
			"  ✓ KV %s (%d/%d) — %d messages, %d bytes restored from %s\n",
			result.Bucket, index, total, result.Messages, result.Bytes, dir,
		)
		completed++
	}

	fmt.Fprintf(os.Stderr, "\nKV restore complete: %d/%d buckets from %s\n", completed, len(dirs), shared.dir)
	if exitCode != 0 {
		return &migration.ExitError{Code: exitCode}
	}
	return nil
}
