package migrate

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const defaultRequestTimeout = 30 * time.Second

// BaseConfig holds flags shared by KV and object store migration.
type BaseConfig struct {
	SourceURL      string
	DestURL        string
	SourceCreds    string
	DestCreds      string
	Buckets        map[string]struct{}
	Omit           map[string]struct{}
	DryRun         bool
	SkipExisting   bool
	NoProgress     bool
	Workers        int
	RequestTimeout time.Duration
}

// KVConfig adds verification options for migrate-nats-kv.
type KVConfig struct {
	BaseConfig
	Verify       bool
	VerifyOnly   bool
	FailuresFile string
}

// ObjectConfig is the flag set for migrate-nats-objects.
type ObjectConfig struct {
	BaseConfig
}

func (c *BaseConfig) ShouldMigrateBucket(name string) bool {
	if _, excluded := c.Omit[name]; excluded {
		return false
	}
	if len(c.Buckets) == 0 {
		return true
	}
	_, ok := c.Buckets[name]
	return ok
}

type baseFlagRefs struct {
	sourceURL    *string
	destURL      *string
	sourceCreds  *string
	destCreds    *string
	bucketFilter *string
	omitFilter   *string
	dryRun       *bool
	skipExisting *bool
	noProgress   *bool
	workers      *int
	timeout      *time.Duration
}

func registerBaseFlags(fs *flag.FlagSet) baseFlagRefs {
	return baseFlagRefs{
		sourceURL:    fs.String("source-url", "", "source NATS server URL (required)"),
		destURL:      fs.String("dest-url", "", "destination NATS server URL (required)"),
		sourceCreds:  fs.String("source-creds", "", "source credentials file (.creds)"),
		destCreds:    fs.String("dest-creds", "", "destination credentials file (.creds)"),
		bucketFilter: fs.String("bucket", "", "comma-separated bucket names to migrate (default: all)"),
		omitFilter:   fs.String("omit", "", "comma-separated bucket names to skip"),
		dryRun:       fs.Bool("dry-run", false, "list buckets and records without writing to destination"),
		skipExisting: fs.Bool("skip-existing", false, "skip records that already exist on the destination"),
		noProgress:   fs.Bool("no-progress", false, "disable progress bars (useful for logs/CI)"),
		workers:      fs.Int("workers", 1, "number of concurrent workers for copying records (1-64)"),
		timeout:      fs.Duration("timeout", defaultRequestTimeout, "per-request timeout for NATS JetStream API calls"),
	}
}

func baseConfigFromRefs(refs baseFlagRefs) BaseConfig {
	cfg := BaseConfig{
		SourceURL:      strings.TrimSpace(*refs.sourceURL),
		DestURL:        strings.TrimSpace(*refs.destURL),
		SourceCreds:    strings.TrimSpace(*refs.sourceCreds),
		DestCreds:      strings.TrimSpace(*refs.destCreds),
		DryRun:         *refs.dryRun,
		SkipExisting:   *refs.skipExisting,
		NoProgress:     *refs.noProgress,
		Workers:        clampWorkers(*refs.workers),
		RequestTimeout: *refs.timeout,
	}

	if *refs.bucketFilter != "" {
		cfg.Buckets = parseBucketNames(*refs.bucketFilter)
	}
	if *refs.omitFilter != "" {
		cfg.Omit = parseBucketNames(*refs.omitFilter)
	}

	return cfg
}

func parseBucketNames(raw string) map[string]struct{} {
	names := make(map[string]struct{})
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func ParseKVFlags(usage string) KVConfig {
	fs := flag.NewFlagSet(usage, flag.ExitOnError)
	refs := registerBaseFlags(fs)
	verify := fs.Bool("verify", true, "verify destination keys match source after migration")
	verifyOnly := fs.Bool("verify-only", false, "verify only — compare source and destination without writing")
	failuresFile := fs.String("failures-file", "", "append verification failures to this file (bucket, key, issue per line)")

	fs.Parse(os.Args[1:])

	cfg := KVConfig{
		BaseConfig:   baseConfigFromRefs(refs),
		Verify:       *verify,
		VerifyOnly:   *verifyOnly,
		FailuresFile: strings.TrimSpace(*failuresFile),
	}
	if cfg.VerifyOnly {
		cfg.Verify = true
	}
	validateBaseConfig(fs, usage, cfg.BaseConfig)
	return cfg
}

func ParseObjectFlags(usage string) ObjectConfig {
	fs := flag.NewFlagSet(usage, flag.ExitOnError)
	refs := registerBaseFlags(fs)

	fs.Parse(os.Args[1:])

	cfg := ObjectConfig{BaseConfig: baseConfigFromRefs(refs)}
	validateBaseConfig(fs, usage, cfg.BaseConfig)
	return cfg
}

func validateBaseConfig(fs *flag.FlagSet, usage string, cfg BaseConfig) {
	if cfg.SourceURL == "" || cfg.DestURL == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", usage)
		fs.PrintDefaults()
		os.Exit(2)
	}
}

func clampWorkers(workers int) int {
	if workers < 1 {
		fmt.Fprintf(os.Stderr, "workers must be at least 1\n")
		os.Exit(2)
	}
	if workers > 64 {
		return 64
	}
	return workers
}

func Connect(url, creds string, requestTimeout time.Duration) (*nats.Conn, jetstream.JetStream, error) {
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}

	opts := []nats.Option{
		nats.Name("nats-keys-migrate"),
		nats.MaxReconnects(-1),
		nats.Timeout(requestTimeout),
	}
	if creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", url, err)
	}

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(requestTimeout))
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return nc, js, nil
}
