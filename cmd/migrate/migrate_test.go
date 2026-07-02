package migrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/nats"
)

func TestSharedBaseConfigFromContexts(t *testing.T) {
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

	orig := shared
	t.Cleanup(func() { shared = orig })

	shared = sharedFlags{
		sourceContext: "source",
		destContext:   "dest",
		workers:       2,
		timeout:       45 * time.Second,
		dryRun:        true,
	}

	cfg, err := sharedBaseConfig()
	if err != nil {
		t.Fatalf("sharedBaseConfig: %v", err)
	}
	if cfg.SourceURL != "nats://source:4222" || cfg.DestURL != "nats://dest:4222" {
		t.Fatalf("urls: %+v", cfg)
	}
	if cfg.Workers != 2 || !cfg.DryRun || cfg.RequestTimeout != 45*time.Second {
		t.Fatalf("flags: %+v", cfg)
	}
}

func TestSharedBaseConfigRequiresContexts(t *testing.T) {
	orig := shared
	t.Cleanup(func() { shared = orig })

	shared = sharedFlags{workers: 1, timeout: nats.DefaultRequestTimeout}

	if _, err := sharedBaseConfig(); err == nil {
		t.Fatal("expected error for missing contexts")
	}
}
