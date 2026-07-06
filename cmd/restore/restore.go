package restore

import (
	"time"

	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore JetStream data from local snapshot files",
}

type sharedFlags struct {
	context    string
	bucket     string
	omit       string
	dir        string
	force      bool
	replicas   int
	noProgress bool
	timeout    time.Duration
}

var shared sharedFlags

// Command returns the restore command group.
func Command() *cobra.Command {
	return restoreCmd
}

func init() {
	flags := restoreCmd.PersistentFlags()
	flags.StringVar(&shared.context, "context", "", "NATS CLI context name")
	flags.StringVar(&shared.bucket, "bucket", "", "comma-separated KV bucket names to restore (default: all found)")
	flags.StringVar(&shared.omit, "omit", "", "comma-separated KV bucket names to skip")
	flags.StringVar(&shared.dir, "dir", "", "backup directory (single bucket or parent of bucket subdirs)")
	flags.BoolVar(&shared.force, "force", false, "delete existing streams before restore")
	flags.IntVar(&shared.replicas, "replicas", 0, "override replica count (0 = use backup config)")
	flags.BoolVar(&shared.noProgress, "no-progress", false, "disable progress output")
	flags.DurationVar(&shared.timeout, "timeout", nats.DefaultRequestTimeout, "per-request NATS timeout")

	_ = restoreCmd.MarkPersistentFlagRequired("context")
	_ = restoreCmd.MarkPersistentFlagRequired("dir")
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
