package kv

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	bucket := "BACKUP"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, "alpha", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, "beta", []byte("two")); err != nil {
		t.Fatal(err)
	}

	nc2, mgr, err := nats.ConnectJSM(srv.ClientURL(), "", nats.DefaultRequestTimeout)
	if err != nil {
		t.Fatalf("connect jsm: %v", err)
	}
	defer nc2.Close()

	backupDir := filepath.Join(t.TempDir(), bucket)
	result, err := BackupBucket(context.Background(), mgr, bucket, backupDir, false, nil)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if result.Messages < 2 {
		t.Fatalf("backup result: %+v", result)
	}

	stream, err := mgr.LoadStream(StreamName(bucket))
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Delete(); err != nil {
		t.Fatal(err)
	}

	restore, err := RestoreBucket(context.Background(), mgr, backupDir, false, 0, false, nil)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restore.Messages < 2 {
		t.Fatalf("restore result: %+v", restore)
	}

	kv, err = js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"alpha": "one", "beta": "two"} {
		entry, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if string(entry.Value()) != want {
			t.Fatalf("key %s = %q, want %q", key, entry.Value(), want)
		}
	}
}

func TestRestoreBucketStreamExists(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	bucket := "EXISTS"
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

	nc2, mgr, err := nats.ConnectJSM(srv.ClientURL(), "", nats.DefaultRequestTimeout)
	if err != nil {
		t.Fatalf("connect jsm: %v", err)
	}
	defer nc2.Close()

	backupDir := filepath.Join(t.TempDir(), bucket)
	if _, err := BackupBucket(context.Background(), mgr, bucket, backupDir, false, nil); err != nil {
		t.Fatalf("backup: %v", err)
	}

	_, err = RestoreBucket(context.Background(), mgr, backupDir, false, 0, false, nil)
	if err == nil {
		t.Fatal("expected error when stream already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestoreBucketForce(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	bucket := "FORCE"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, "alpha", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, "beta", []byte("two")); err != nil {
		t.Fatal(err)
	}

	nc2, mgr, err := nats.ConnectJSM(srv.ClientURL(), "", nats.DefaultRequestTimeout)
	if err != nil {
		t.Fatalf("connect jsm: %v", err)
	}
	defer nc2.Close()

	backupDir := filepath.Join(t.TempDir(), bucket)
	if _, err := BackupBucket(context.Background(), mgr, bucket, backupDir, false, nil); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Corrupt live data; force restore should replace stream from snapshot.
	if _, err := kv.Put(ctx, "alpha", []byte("stale")); err != nil {
		t.Fatal(err)
	}
	if err := kv.Delete(ctx, "beta"); err != nil {
		t.Fatal(err)
	}

	restore, err := RestoreBucket(context.Background(), mgr, backupDir, true, 0, false, nil)
	if err != nil {
		t.Fatalf("force restore: %v", err)
	}
	if restore.Messages < 2 {
		t.Fatalf("restore result: %+v", restore)
	}

	kv, err = js.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"alpha": "one", "beta": "two"} {
		entry, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if string(entry.Value()) != want {
			t.Fatalf("key %s = %q, want %q", key, entry.Value(), want)
		}
	}
}
