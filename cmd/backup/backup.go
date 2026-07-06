package backup

import (
	"fmt"
	"os"
	"time"

	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup JetStream data to local snapshot files",
}

type sharedFlags struct {
	context    string
	bucket     string
	omit       string
	dir        string
	noProgress bool
	timeout    time.Duration
}

var shared sharedFlags

// Command returns the backup command group.
func Command() *cobra.Command {
	return backupCmd
}

func init() {
	flags := backupCmd.PersistentFlags()
	flags.StringVar(&shared.context, "context", "", "NATS CLI context name")
	flags.StringVar(&shared.bucket, "bucket", "", "comma-separated KV bucket names (default: all)")
	flags.StringVar(&shared.omit, "omit", "", "comma-separated KV bucket names to skip")
	flags.StringVar(&shared.dir, "dir", "", "output directory for snapshot files")
	flags.BoolVar(&shared.noProgress, "no-progress", false, "disable progress output")
	flags.DurationVar(&shared.timeout, "timeout", nats.DefaultRequestTimeout, "per-request NATS timeout")

	_ = backupCmd.MarkPersistentFlagRequired("context")
	_ = backupCmd.MarkPersistentFlagRequired("dir")
}

func endpointConfig() (migration.EndpointConfig, error) {
	return migration.NewEndpointConfig(migration.EndpointInput{
		Context:      shared.context,
		BucketFilter: shared.bucket,
		OmitFilter:   shared.omit,
		NoProgress:   shared.noProgress,
		Timeout:      shared.timeout,
	})
}

func logStatus(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
