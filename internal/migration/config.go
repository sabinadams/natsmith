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
	SourceURL    string
	DestURL      string
	SourceCreds  string
	DestCreds    string
	BucketFilter string
	OmitFilter   string
	DryRun       bool
	SkipExisting bool
	NoProgress   bool
	Workers      int
	Timeout      time.Duration
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

	cfg := BaseConfig{
		SourceURL:      strings.TrimSpace(in.SourceURL),
		DestURL:        strings.TrimSpace(in.DestURL),
		SourceCreds:    strings.TrimSpace(in.SourceCreds),
		DestCreds:      strings.TrimSpace(in.DestCreds),
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
		return fmt.Errorf("source-url and dest-url are required")
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
