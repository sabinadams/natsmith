package migrate

import (
	"time"

	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate JetStream data between NATS clusters",
}

type sharedFlags struct {
	sourceURL    string
	destURL      string
	sourceCreds  string
	destCreds    string
	bucket       string
	omit         string
	dryRun       bool
	skipExisting bool
	noProgress   bool
	workers      int
	timeout      time.Duration
}

var shared sharedFlags

// Command returns the migrate command group.
func Command() *cobra.Command {
	return migrateCmd
}

func init() {
	flags := migrateCmd.PersistentFlags()
	flags.StringVar(&shared.sourceURL, "source-url", "", "source NATS server URL (required)")
	flags.StringVar(&shared.destURL, "dest-url", "", "destination NATS server URL (required)")
	flags.StringVar(&shared.sourceCreds, "source-creds", "", "source credentials file (.creds)")
	flags.StringVar(&shared.destCreds, "dest-creds", "", "destination credentials file (.creds)")
	flags.StringVar(&shared.bucket, "bucket", "", "comma-separated bucket names to migrate (default: all)")
	flags.StringVar(&shared.omit, "omit", "", "comma-separated bucket names to skip")
	flags.BoolVar(&shared.dryRun, "dry-run", false, "list buckets and records without writing to destination")
	flags.BoolVar(&shared.skipExisting, "skip-existing", false, "skip records that already exist on the destination")
	flags.BoolVar(&shared.noProgress, "no-progress", false, "disable progress bars (useful for logs/CI)")
	flags.IntVar(&shared.workers, "workers", 1, "number of concurrent workers for copying records (1-64)")
	flags.DurationVar(&shared.timeout, "timeout", nats.DefaultRequestTimeout, "per-request timeout for NATS JetStream API calls")

	_ = migrateCmd.MarkPersistentFlagRequired("source-url")
	_ = migrateCmd.MarkPersistentFlagRequired("dest-url")
}

func sharedBaseConfig() (migration.BaseConfig, error) {
	return migration.NewBaseConfig(migration.BaseConfigInput{
		SourceURL:    shared.sourceURL,
		DestURL:      shared.destURL,
		SourceCreds:  shared.sourceCreds,
		DestCreds:    shared.destCreds,
		BucketFilter: shared.bucket,
		OmitFilter:   shared.omit,
		DryRun:       shared.dryRun,
		SkipExisting: shared.skipExisting,
		NoProgress:   shared.noProgress,
		Workers:      shared.workers,
		Timeout:      shared.timeout,
	})
}
