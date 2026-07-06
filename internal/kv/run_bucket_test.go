package kv

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestRunBucketCopyAndVerify(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	source, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SRC"})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SRC_DEST"})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}

	for key, value := range map[string]string{"a": "1", "b": "2"} {
		if _, err := source.Put(ctx, key, []byte(value)); err != nil {
			t.Fatalf("put %s: %v", key, err)
		}
	}
	if _, err := source.Put(ctx, "deleted", []byte("gone")); err != nil {
		t.Fatalf("put deleted: %v", err)
	}
	if err := source.Delete(ctx, "deleted"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	run, err := RunBucket(ctx, js, "SRC", BucketRunParams{
		Workers: 2,
		Dest:    dest,
		Verify:  true,
	}, nil, nil)
	if err != nil {
		t.Fatalf("run bucket: %v", err)
	}
	if run.Migratable != 2 {
		t.Fatalf("migratable = %d, want 2", run.Migratable)
	}
	if run.Copy.Migrated != 2 {
		t.Fatalf("copy stats: %+v", run.Copy)
	}
	if !run.Verify.Passed() || run.Verify.OK != 2 {
		t.Fatalf("verify: %+v", run.Verify)
	}
}

func TestRunBucketDryRun(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	kvStore, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "DRY"})
	if err != nil {
		t.Fatalf("create kv: %v", err)
	}
	if _, err := kvStore.Put(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("put: %v", err)
	}

	run, err := RunBucket(ctx, js, "DRY", BucketRunParams{DryRun: true}, nil, nil)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if run.Migratable != 1 || run.Copy.Migrated != 0 {
		t.Fatalf("unexpected dry-run result: %+v", run)
	}
}

func TestRunBucketVerifyOnly(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	source, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "VERIFY_SRC"})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "VERIFY_DEST"})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}

	if _, err := source.Put(ctx, "match", []byte("ok")); err != nil {
		t.Fatalf("put source: %v", err)
	}
	if _, err := dest.Put(ctx, "match", []byte("ok")); err != nil {
		t.Fatalf("put dest: %v", err)
	}
	if _, err := source.Put(ctx, "missing", []byte("x")); err != nil {
		t.Fatalf("put missing: %v", err)
	}

	run, err := RunBucket(ctx, js, "VERIFY_SRC", BucketRunParams{
		VerifyOnly: true,
		Verify:     true,
		Workers:    2,
		Dest:       dest,
	}, nil, nil)
	if err != nil {
		t.Fatalf("verify only: %v", err)
	}
	if run.Verify.OK != 1 || run.Verify.Missing != 1 {
		t.Fatalf("verify result: %+v", run.Verify)
	}
}

func TestRunBucketSkipExisting(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	source, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SKIP_SRC"})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SKIP_DEST"})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}

	if _, err := source.Put(ctx, "shared", []byte("source")); err != nil {
		t.Fatalf("put shared source: %v", err)
	}
	if _, err := source.Put(ctx, "new", []byte("only-source")); err != nil {
		t.Fatalf("put new source: %v", err)
	}
	if _, err := dest.Put(ctx, "shared", []byte("dest")); err != nil {
		t.Fatalf("put shared dest: %v", err)
	}

	run, err := RunBucket(ctx, js, "SKIP_SRC", BucketRunParams{
		SkipExisting: true,
		Workers:      1,
		Dest:         dest,
	}, nil, nil)
	if err != nil {
		t.Fatalf("skip existing: %v", err)
	}
	if run.Copy.Migrated != 1 || run.Copy.Skipped != 1 {
		t.Fatalf("copy stats: %+v", run.Copy)
	}

	shared, err := dest.Get(ctx, "shared")
	if err != nil {
		t.Fatalf("get shared: %v", err)
	}
	if string(shared.Value()) != "dest" {
		t.Fatalf("shared = %q, want dest value preserved", shared.Value())
	}
}

type ghostKeyLister struct {
	real    jetstream.KeyLister
	phantom string
}

func (g *ghostKeyLister) Keys() <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		ch <- g.phantom
		for key := range g.real.Keys() {
			ch <- key
		}
	}()
	return ch
}

func (g *ghostKeyLister) Stop() error {
	return g.real.Stop()
}

type ghostSourceKV struct {
	jetstream.KeyValue
	phantom string
}

func (g *ghostSourceKV) ListKeys(ctx context.Context, opts ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	real, err := g.KeyValue.ListKeys(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &ghostKeyLister{real: real, phantom: g.phantom}, nil
}

func TestRunBucketSkipsGhostKeys(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	source, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "GHOST_SRC"})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "GHOST_DEST"})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}

	if _, err := source.Put(ctx, "real", []byte("ok")); err != nil {
		t.Fatalf("put: %v", err)
	}

	sourceWithGhost := &ghostSourceKV{KeyValue: source, phantom: "phantom-key"}

	run, err := runBucketFromSource(ctx, sourceWithGhost, BucketRunParams{
		Workers: 1,
		Dest:    dest,
	}, nil, nil)
	if err != nil {
		t.Fatalf("run bucket: %v", err)
	}
	if run.Migratable != 2 {
		t.Fatalf("migratable = %d, want 2", run.Migratable)
	}
	if run.GhostSkipped != 1 {
		t.Fatalf("ghost skipped = %d, want 1", run.GhostSkipped)
	}
	if run.Copy.Migrated != 1 {
		t.Fatalf("copy stats: %+v", run.Copy)
	}

	entry, err := dest.Get(ctx, "real")
	if err != nil {
		t.Fatalf("get real on dest: %v", err)
	}
	if string(entry.Value()) != "ok" {
		t.Fatalf("real = %q, want ok", entry.Value())
	}
}

func TestRunBucketDestOnly(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	source, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "DO_SRC"})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "DO_DEST"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := source.Put(ctx, "shared", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Put(ctx, "shared", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Put(ctx, "extra", []byte("x")); err != nil {
		t.Fatal(err)
	}

	run, err := RunBucket(ctx, js, "DO_SRC", BucketRunParams{
		Workers: 1,
		Dest:    dest,
		Verify:  true,
	}, nil, nil)
	if err != nil {
		t.Fatalf("run bucket: %v", err)
	}
	if !run.Verify.Passed() || run.Verify.DestOnly != 1 || run.Verify.DestOnlyKeys[0] != "extra" {
		t.Fatalf("verify: %+v", run.Verify)
	}
}
