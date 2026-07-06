package migration

import (
	"fmt"
	"strings"
	"time"

	"github.com/sabinadams/natsmith/internal/nats"
)

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

// KVConfig adds verification options for KV migration.
type KVConfig struct {
	BaseConfig
	Verify       bool
	VerifyOnly   bool
	FailuresFile string
}

// ObjectConfig is the flag set for object store migration.
type ObjectConfig struct {
	BaseConfig
}

// BaseConfigInput holds raw flag values for building a BaseConfig.
type BaseConfigInput struct {
	SourceContext string
	DestContext   string
	BucketFilter  string
	OmitFilter    string
	DryRun        bool
	SkipExisting  bool
	NoProgress    bool
	Workers       int
	Timeout       time.Duration
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

// ParseBucketNames splits a comma-separated bucket list.
func ParseBucketNames(raw string) map[string]struct{} {
	names := make(map[string]struct{})
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

// NewBaseConfig builds and validates a BaseConfig from flag values.
func NewBaseConfig(in BaseConfigInput) (BaseConfig, error) {
	workers, err := ClampWorkers(in.Workers)
	if err != nil {
		return BaseConfig{}, err
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = nats.DefaultRequestTimeout
	}

	sourceURL, sourceCreds, err := resolveContext("source", in.SourceContext)
	if err != nil {
		return BaseConfig{}, err
	}

	destURL, destCreds, err := resolveContext("dest", in.DestContext)
	if err != nil {
		return BaseConfig{}, err
	}

	cfg := BaseConfig{
		SourceURL:      sourceURL,
		DestURL:        destURL,
		SourceCreds:    sourceCreds,
		DestCreds:      destCreds,
		DryRun:         in.DryRun,
		SkipExisting:   in.SkipExisting,
		NoProgress:     in.NoProgress,
		Workers:        workers,
		RequestTimeout: timeout,
	}

	if in.BucketFilter != "" {
		cfg.Buckets = ParseBucketNames(in.BucketFilter)
	}
	if in.OmitFilter != "" {
		cfg.Omit = ParseBucketNames(in.OmitFilter)
	}

	if err := ValidateBaseConfig(cfg); err != nil {
		return BaseConfig{}, err
	}
	return cfg, nil
}

// NewKVConfig builds a KVConfig from a base config and KV-specific flag values.
func NewKVConfig(base BaseConfig, verify, verifyOnly bool, failuresFile string) KVConfig {
	cfg := KVConfig{
		BaseConfig:   base,
		Verify:       verify,
		VerifyOnly:   verifyOnly,
		FailuresFile: strings.TrimSpace(failuresFile),
	}
	if cfg.VerifyOnly {
		cfg.Verify = true
	}
	return cfg
}

// NewObjectConfig wraps a validated base config for object store migration.
func NewObjectConfig(base BaseConfig) ObjectConfig {
	return ObjectConfig{BaseConfig: base}
}

// ValidateBaseConfig checks required migration settings.
func ValidateBaseConfig(cfg BaseConfig) error {
	if cfg.SourceURL == "" || cfg.DestURL == "" {
		return fmt.Errorf("source-context and dest-context are required")
	}
	return nil
}

// ClampWorkers bounds worker count to 1–64.
func ClampWorkers(workers int) (int, error) {
	if workers < 1 {
		return 0, fmt.Errorf("workers must be at least 1")
	}
	if workers > 64 {
		return 64, nil
	}
	return workers, nil
}

func resolveContext(label, name string) (url, creds string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("%s-context is required", label)
	}

	ctx, err := nats.LoadContext(name)
	if err != nil {
		return "", "", fmt.Errorf("%s-context: %w", label, err)
	}
	if ctx.URL == "" {
		return "", "", fmt.Errorf("%s-context %q has no url", label, name)
	}

	return ctx.URL, ctx.Creds, nil
}

// EndpointConfig holds settings for single-cluster stream operations.
type EndpointConfig struct {
	URL            string
	Creds          string
	Buckets        map[string]struct{}
	Omit           map[string]struct{}
	NoProgress     bool
	RequestTimeout time.Duration
}

// EndpointInput holds raw flag values for building an EndpointConfig.
type EndpointInput struct {
	Context      string
	BucketFilter string
	OmitFilter   string
	NoProgress   bool
	Timeout      time.Duration
}

// NewEndpointConfig builds and validates an EndpointConfig from flag values.
func NewEndpointConfig(in EndpointInput) (EndpointConfig, error) {
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = nats.DefaultRequestTimeout
	}

	url, creds, err := resolveContext("context", in.Context)
	if err != nil {
		return EndpointConfig{}, err
	}

	cfg := EndpointConfig{
		URL:            url,
		Creds:          creds,
		NoProgress:     in.NoProgress,
		RequestTimeout: timeout,
	}

	if in.BucketFilter != "" {
		cfg.Buckets = ParseBucketNames(in.BucketFilter)
	}
	if in.OmitFilter != "" {
		cfg.Omit = ParseBucketNames(in.OmitFilter)
	}

	return cfg, nil
}

func (c *EndpointConfig) ShouldIncludeBucket(name string) bool {
	if _, excluded := c.Omit[name]; excluded {
		return false
	}
	if len(c.Buckets) == 0 {
		return true
	}
	_, ok := c.Buckets[name]
	return ok
}
