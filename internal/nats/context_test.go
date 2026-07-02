package nats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContext(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}

	const body = `{
  "url": "nats://example.test:4222",
  "creds": "~/secrets/example.creds"
}`
	if err := os.WriteFile(filepath.Join(ctxDir, "demo.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := LoadContext("demo")
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	if got.URL != "nats://example.test:4222" {
		t.Fatalf("URL = %q", got.URL)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	wantCreds := filepath.Join(home, "secrets", "example.creds")
	if got.Creds != wantCreds {
		t.Fatalf("Creds = %q, want %q", got.Creds, wantCreds)
	}
}

func TestLoadContextRequiresName(t *testing.T) {
	t.Parallel()

	if _, err := LoadContext(""); err == nil {
		t.Fatal("expected error for empty context name")
	}
}

func TestLoadContextUnknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nats", "context"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := LoadContext("missing"); err == nil {
		t.Fatal("expected error for unknown context")
	}
}

func TestLoadContextInvalidName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../etc", `foo/bar`} {
		if _, err := LoadContext(name); err == nil {
			t.Fatalf("expected error for invalid name %q", name)
		}
	}
}

func TestLoadContextInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := LoadContext("broken"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadContextExpandEnvInCreds(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NATS_CREDS_PATH", filepath.Join(dir, "token.creds"))
	body := `{"url":"nats://example.test:4222","creds":"${NATS_CREDS_PATH}"}`
	if err := os.WriteFile(filepath.Join(ctxDir, "demo.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := LoadContext("demo")
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	if got.Creds != filepath.Join(dir, "token.creds") {
		t.Fatalf("Creds = %q", got.Creds)
	}
}
