package restore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/kv"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestRunKVRestoreEmbedded(t *testing.T) {
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

	const bucket = "CLI_RESTORE"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	kvStore, err := js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kvStore.Put(ctx, "alpha", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := kvStore.Put(ctx, "beta", []byte("two")); err != nil {
		t.Fatal(err)
	}

	nc2, mgr, err := nats.ConnectJSM(url, "", nats.DefaultRequestTimeout)
	if err != nil {
		t.Fatalf("connect jsm: %v", err)
	}
	defer nc2.Close()

	backupRoot := filepath.Join(t.TempDir(), "backups")
	backupDir := filepath.Join(backupRoot, bucket)
	if _, err := kv.BackupBucket(context.Background(), mgr, bucket, backupDir, false, nil); err != nil {
		t.Fatalf("backup: %v", err)
	}

	stream, err := mgr.LoadStream(kv.StreamName(bucket))
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Delete(); err != nil {
		t.Fatal(err)
	}

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
	if err := runKVRestore(cfg); err != nil {
		t.Fatalf("runKVRestore: %v", err)
	}

	kvStore, err = js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"alpha": "one", "beta": "two"} {
		entry, err := kvStore.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if string(entry.Value()) != want {
			t.Fatalf("key %s = %q, want %q", key, entry.Value(), want)
		}
	}
}

func TestRunKVRestoreForce(t *testing.T) {
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

	const bucket = "CLI_FORCE"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	kvStore, err := js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kvStore.Put(ctx, "alpha", []byte("one")); err != nil {
		t.Fatal(err)
	}

	nc2, mgr, err := nats.ConnectJSM(url, "", nats.DefaultRequestTimeout)
	if err != nil {
		t.Fatalf("connect jsm: %v", err)
	}
	defer nc2.Close()

	backupRoot := filepath.Join(t.TempDir(), "backups")
	backupDir := filepath.Join(backupRoot, bucket)
	if _, err := kv.BackupBucket(context.Background(), mgr, bucket, backupDir, false, nil); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if _, err := kvStore.Put(ctx, "alpha", []byte("stale")); err != nil {
		t.Fatal(err)
	}

	orig := shared
	t.Cleanup(func() { shared = orig })
	shared = sharedFlags{
		context:    "test",
		dir:        backupRoot,
		force:      true,
		noProgress: true,
		timeout:    nats.DefaultRequestTimeout,
	}

	cfg, err := endpointConfig()
	if err != nil {
		t.Fatalf("endpointConfig: %v", err)
	}
	if err := runKVRestore(cfg); err != nil {
		t.Fatalf("runKVRestore: %v", err)
	}

	entry, err := kvStore.Get(ctx, "alpha")
	if err != nil {
		t.Fatalf("get alpha: %v", err)
	}
	if string(entry.Value()) != "one" {
		t.Fatalf("alpha = %q, want one", entry.Value())
	}
}
