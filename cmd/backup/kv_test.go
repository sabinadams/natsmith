package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestRunKVBackupEmbedded(t *testing.T) {
	srv := testutil.StartServer(t)
	url := srv.ClientURL()

	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ctxDir, "test.json"),
		[]byte(`{"url":"`+url+`"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	nc := testutil.Connect(t, url)
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	const bucket = "CLI_BACKUP"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, "k", []byte("v")); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(t.TempDir(), "backups")
	orig := shared
	t.Cleanup(func() { shared = orig })
	shared = sharedFlags{
		context:    "test",
		dir:        backupRoot,
		noProgress: true,
		timeout:    nats.DefaultRequestTimeout,
	}

	cfg, err := endpointConfig()
	if err != nil {
		t.Fatalf("endpointConfig: %v", err)
	}
	if err := runKVBackup(cfg); err != nil {
		t.Fatalf("runKVBackup: %v", err)
	}

	outDir := filepath.Join(backupRoot, bucket)
	for _, name := range []string{"backup.json", "stream.tar.s2"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestRunKVBackupBucketFilter(t *testing.T) {
	srv := testutil.StartServer(t)
	url := srv.ClientURL()

	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "nats", "context")
	if err := os.MkdirAll(ctxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ctxDir, "test.json"),
		[]byte(`{"url":"`+url+`"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	nc := testutil.Connect(t, url)
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	for _, bucket := range []string{"KEEP", "SKIP"} {
		if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
			t.Fatalf("create %s: %v", bucket, err)
		}
	}

	backupRoot := filepath.Join(t.TempDir(), "backups")
	orig := shared
	t.Cleanup(func() { shared = orig })
	shared = sharedFlags{
		context:    "test",
		dir:        backupRoot,
		bucket:     "KEEP",
		noProgress: true,
		timeout:    nats.DefaultRequestTimeout,
	}

	cfg, err := migration.NewEndpointConfig(migration.EndpointInput{
		Context:      shared.context,
		BucketFilter: shared.bucket,
		NoProgress:   shared.noProgress,
		Timeout:      shared.timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runKVBackup(cfg); err != nil {
		t.Fatalf("runKVBackup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupRoot, "KEEP", "backup.json")); err != nil {
		t.Fatalf("KEEP backup missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupRoot, "SKIP", "backup.json")); !os.IsNotExist(err) {
		t.Fatalf("SKIP backup should not exist: %v", err)
	}
}
