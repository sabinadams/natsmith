package migration

import (
	"testing"
	"time"
)

func TestParseBucketNames(t *testing.T) {
	t.Parallel()

	got := ParseBucketNames(" a , b,, c ")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for _, name := range []string{"a", "b", "c"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing %q in %v", name, got)
		}
	}
}

func TestShouldMigrateBucket(t *testing.T) {
	t.Parallel()

	all := BaseConfig{}
	if !all.ShouldMigrateBucket("any") {
		t.Fatal("empty filter should allow all buckets")
	}

	filtered := BaseConfig{Buckets: map[string]struct{}{"a": {}}}
	if filtered.ShouldMigrateBucket("a") != true || filtered.ShouldMigrateBucket("b") != false {
		t.Fatal("bucket filter mismatch")
	}

	omit := BaseConfig{Omit: map[string]struct{}{"skip": {}}}
	if omit.ShouldMigrateBucket("skip") {
		t.Fatal("omit should exclude bucket")
	}

	both := BaseConfig{
		Buckets: map[string]struct{}{"a": {}, "skip": {}},
		Omit:    map[string]struct{}{"skip": {}},
	}
	if !both.ShouldMigrateBucket("a") || both.ShouldMigrateBucket("skip") {
		t.Fatal("omit should win over bucket include")
	}
}

func TestClampWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want int
	}{
		{1, 1},
		{16, 16},
		{64, 64},
		{100, 64},
	}
	for _, tt := range tests {
		got, err := ClampWorkers(tt.in)
		if err != nil {
			t.Fatalf("clampWorkers(%d): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("clampWorkers(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}

	if _, err := ClampWorkers(0); err == nil {
		t.Fatal("expected error for workers < 1")
	}
}

func TestNewBaseConfig(t *testing.T) {
	t.Parallel()

	cfg, err := NewBaseConfig(BaseConfigInput{
		SourceURL:    " nats://source ",
		DestURL:      "nats://dest",
		SourceCreds:  " src.creds ",
		DestCreds:    "dest.creds",
		BucketFilter: "a,b",
		OmitFilter:   "c",
		DryRun:       true,
		SkipExisting: true,
		NoProgress:   true,
		Workers:      8,
		Timeout:      45 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewBaseConfig: %v", err)
	}
	if cfg.SourceURL != "nats://source" || cfg.DestURL != "nats://dest" {
		t.Fatalf("urls not trimmed: %+v", cfg)
	}
	if cfg.SourceCreds != "src.creds" || cfg.DestCreds != "dest.creds" {
		t.Fatalf("creds not trimmed: %+v", cfg)
	}
	if !cfg.DryRun || !cfg.SkipExisting || !cfg.NoProgress {
		t.Fatalf("bools not set: %+v", cfg)
	}
	if cfg.Workers != 8 || cfg.RequestTimeout != 45*time.Second {
		t.Fatalf("workers/timeout: %+v", cfg)
	}
	if _, ok := cfg.Buckets["a"]; !ok {
		t.Fatalf("missing bucket a: %+v", cfg.Buckets)
	}
	if _, ok := cfg.Buckets["b"]; !ok {
		t.Fatalf("missing bucket b: %+v", cfg.Buckets)
	}
	if _, ok := cfg.Omit["c"]; !ok {
		t.Fatalf("omit: %+v", cfg.Omit)
	}
}

func TestValidateBaseConfig(t *testing.T) {
	t.Parallel()

	if err := ValidateBaseConfig(BaseConfig{SourceURL: "nats://source", DestURL: "nats://dest"}); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	if err := ValidateBaseConfig(BaseConfig{DestURL: "nats://dest"}); err == nil {
		t.Fatal("expected error for missing source URL")
	}
	if err := ValidateBaseConfig(BaseConfig{SourceURL: "nats://source"}); err == nil {
		t.Fatal("expected error for missing dest URL")
	}
}

func TestNewBaseConfigRequiresURLs(t *testing.T) {
	t.Parallel()

	if _, err := NewBaseConfig(BaseConfigInput{}); err == nil {
		t.Fatal("expected error for missing URLs")
	}
}

func TestNewKVConfigVerifyOnly(t *testing.T) {
	t.Parallel()

	base, err := NewBaseConfig(BaseConfigInput{
		SourceURL: "nats://source",
		DestURL:   "nats://dest",
		Workers:   1,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := NewKVConfig(base, false, true, "")
	if !cfg.VerifyOnly || !cfg.Verify {
		t.Fatalf("verify-only should force verify: %+v", cfg)
	}
}

func TestNewObjectConfig(t *testing.T) {
	t.Parallel()

	base, err := NewBaseConfig(BaseConfigInput{
		SourceURL: "nats://source",
		DestURL:   "nats://dest",
		Workers:   4,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := NewObjectConfig(base)
	if cfg.Workers != 4 || cfg.SourceURL != "nats://source" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
