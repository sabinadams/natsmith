package migration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/nats"
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

func writeTestContexts(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "source.json"), []byte(`{"url":"nats://source:4222","creds":"source.creds"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "dest.json"), []byte(`{"url":"nats://dest:4222","creds":"dest.creds"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestResolveContext(t *testing.T) {
	writeTestContexts(t)

	url, creds, err := resolveContext("source", "source")
	if err != nil {
		t.Fatalf("resolveContext: %v", err)
	}
	if url != "nats://source:4222" || creds != "source.creds" {
		t.Fatalf("got url=%q creds=%q", url, creds)
	}
}

func TestResolveContextRequiresName(t *testing.T) {
	t.Parallel()

	if _, _, err := resolveContext("dest", ""); err == nil {
		t.Fatal("expected error for missing context name")
	}
}

func TestResolveContextUnknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nats", "context"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, _, err := resolveContext("source", "missing"); err == nil {
		t.Fatal("expected error for unknown context")
	}
}

func TestResolveContextEmptyURL(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "empty.json"), []byte(`{"url":"","creds":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, _, err := resolveContext("dest", "empty"); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestNewBaseConfigDefaultTimeout(t *testing.T) {
	writeTestContexts(t)

	cfg, err := NewBaseConfig(BaseConfigInput{
		SourceContext: "source",
		DestContext:   "dest",
		Workers:       1,
	})
	if err != nil {
		t.Fatalf("NewBaseConfig: %v", err)
	}
	if cfg.RequestTimeout != nats.DefaultRequestTimeout {
		t.Fatalf("timeout = %v, want default", cfg.RequestTimeout)
	}
}

func TestNewBaseConfig(t *testing.T) {
	writeTestContexts(t)

	cfg, err := NewBaseConfig(BaseConfigInput{
		SourceContext: "source",
		DestContext:   "dest",
		BucketFilter:  "a,b",
		OmitFilter:    "c",
		DryRun:        true,
		SkipExisting:  true,
		NoProgress:    true,
		Workers:       8,
		Timeout:       45 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewBaseConfig: %v", err)
	}
	if cfg.SourceURL != "nats://source:4222" || cfg.DestURL != "nats://dest:4222" {
		t.Fatalf("urls from context: %+v", cfg)
	}
	if cfg.SourceCreds != "source.creds" || cfg.DestCreds != "dest.creds" {
		t.Fatalf("creds from context: %+v", cfg)
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

func TestNewBaseConfigRequiresContexts(t *testing.T) {
	t.Parallel()

	if _, err := NewBaseConfig(BaseConfigInput{Workers: 1}); err == nil {
		t.Fatal("expected error for missing contexts")
	}
}

func TestNewKVConfigVerifyOnly(t *testing.T) {
	t.Parallel()

	base := BaseConfig{
		SourceURL: "nats://source",
		DestURL:   "nats://dest",
		Workers:   1,
	}

	cfg := NewKVConfig(base, false, true, "")
	if !cfg.VerifyOnly || !cfg.Verify {
		t.Fatalf("verify-only should force verify: %+v", cfg)
	}
}

func TestNewObjectConfig(t *testing.T) {
	t.Parallel()

	base := BaseConfig{
		SourceURL: "nats://source",
		DestURL:   "nats://dest",
		Workers:   4,
	}

	cfg := NewObjectConfig(base)
	if cfg.Workers != 4 || cfg.SourceURL != "nats://source" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestNewEndpointConfigRequiresContext(t *testing.T) {
	t.Parallel()

	if _, err := NewEndpointConfig(EndpointInput{}); err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestNewEndpointConfigFromContext(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ctxDir, "cluster.json"),
		[]byte(`{"url":"nats://cluster:4222","creds":"cluster.creds"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := NewEndpointConfig(EndpointInput{
		Context:      "cluster",
		BucketFilter: "a,b",
		OmitFilter:   "skip",
		NoProgress:   true,
		Timeout:      45 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewEndpointConfig: %v", err)
	}
	if cfg.URL != "nats://cluster:4222" || cfg.Creds != "cluster.creds" {
		t.Fatalf("resolved context: %+v", cfg)
	}
	if cfg.RequestTimeout != 45*time.Second || !cfg.NoProgress {
		t.Fatalf("flags: %+v", cfg)
	}
	if !cfg.ShouldIncludeBucket("a") || !cfg.ShouldIncludeBucket("b") {
		t.Fatal("expected bucket filter")
	}
	if cfg.ShouldIncludeBucket("other") || cfg.ShouldIncludeBucket("skip") {
		t.Fatal("expected other/skip filtered")
	}
}

func TestEndpointShouldIncludeBucket(t *testing.T) {
	t.Parallel()

	cfg := EndpointConfig{
		Buckets: map[string]struct{}{"keep": {}},
		Omit:    map[string]struct{}{"skip": {}},
	}
	if !cfg.ShouldIncludeBucket("keep") {
		t.Fatal("expected keep")
	}
	if cfg.ShouldIncludeBucket("skip") {
		t.Fatal("expected skip omitted")
	}
	if cfg.ShouldIncludeBucket("other") {
		t.Fatal("expected other filtered out")
	}
}
