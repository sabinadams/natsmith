package migrate

import (
	"flag"
	"os"
	"testing"
	"time"
)

func TestParseBucketNames(t *testing.T) {
	t.Parallel()

	got := parseBucketNames(" a , b,, c ")
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
		if got := clampWorkers(tt.in); got != tt.want {
			t.Errorf("clampWorkers(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBaseConfigFromRefs(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	refs := registerBaseFlags(fs)
	timeout := 45 * time.Second
	workers := 8
	dryRun := true
	skipExisting := true
	noProgress := true
	*refs.sourceURL = " nats://source "
	*refs.destURL = "nats://dest"
	*refs.sourceCreds = " src.creds "
	*refs.destCreds = "dest.creds"
	*refs.bucketFilter = "a,b"
	*refs.omitFilter = "c"
	*refs.dryRun = dryRun
	*refs.skipExisting = skipExisting
	*refs.noProgress = noProgress
	*refs.workers = workers
	*refs.timeout = timeout

	cfg := baseConfigFromRefs(refs)
	if cfg.SourceURL != "nats://source" || cfg.DestURL != "nats://dest" {
		t.Fatalf("urls not trimmed: %+v", cfg)
	}
	if cfg.SourceCreds != "src.creds" || cfg.DestCreds != "dest.creds" {
		t.Fatalf("creds not trimmed: %+v", cfg)
	}
	if !cfg.DryRun || !cfg.SkipExisting || !cfg.NoProgress {
		t.Fatalf("bools not set: %+v", cfg)
	}
	if cfg.Workers != 8 || cfg.RequestTimeout != timeout {
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

func TestParseKVFlagsVerifyOnly(t *testing.T) {
	oldArgs := osArgs()
	defer restoreArgs(oldArgs)

	setArgs([]string{
		"migrate-nats-kv",
		"-source-url", "nats://source",
		"-dest-url", "nats://dest",
		"-verify-only",
		"-verify=false",
	})
	cfg := ParseKVFlags("migrate-nats-kv")
	if !cfg.VerifyOnly || !cfg.Verify {
		t.Fatalf("verify-only should force verify: %+v", cfg)
	}
}

func TestParseObjectFlags(t *testing.T) {
	oldArgs := osArgs()
	defer restoreArgs(oldArgs)

	setArgs([]string{
		"migrate-nats-objects",
		"-source-url", "nats://source",
		"-dest-url", "nats://dest",
		"-workers", "4",
	})
	cfg := ParseObjectFlags("migrate-nats-objects")
	if cfg.Workers != 4 || cfg.SourceURL != "nats://source" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func osArgs() []string {
	return append([]string(nil), os.Args...)
}

func restoreArgs(args []string) {
	os.Args = args
}

func setArgs(args []string) {
	os.Args = args
}
