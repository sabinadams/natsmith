package restore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/nats"
)

func TestEndpointConfigFromContext(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ctxDir, "local.json"),
		[]byte(`{"url":"nats://127.0.0.1:4222","creds":"local.creds"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	orig := shared
	t.Cleanup(func() { shared = orig })

	shared = sharedFlags{
		context: "local",
		dir:     "/tmp/backups",
		timeout: 45 * time.Second,
		bucket:  "schema",
		force:   true,
	}

	cfg, err := endpointConfig()
	if err != nil {
		t.Fatalf("endpointConfig: %v", err)
	}
	if cfg.URL != "nats://127.0.0.1:4222" || cfg.Creds != "local.creds" {
		t.Fatalf("context: %+v", cfg)
	}
	if !cfg.ShouldIncludeBucket("schema") || cfg.ShouldIncludeBucket("other") {
		t.Fatal("expected bucket filter")
	}
}

func TestEndpointConfigRequiresContext(t *testing.T) {
	orig := shared
	t.Cleanup(func() { shared = orig })

	shared = sharedFlags{dir: "/tmp/backups", timeout: nats.DefaultRequestTimeout}

	if _, err := endpointConfig(); err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestCommandHelp(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"kv", "--help"},
	} {
		cmd := Command()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("help %v: %v", args, err)
		}
	}
}
