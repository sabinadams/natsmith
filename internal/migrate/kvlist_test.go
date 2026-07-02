package migrate

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestKeyFromSubject(t *testing.T) {
	t.Parallel()

	prefix := "$KV.schema."
	if key, ok := keyFromSubject("$KV.schema.my-key", prefix); !ok || key != "my-key" {
		t.Fatalf("got %q ok=%v", key, ok)
	}
	if _, ok := keyFromSubject("$KV.other.key", prefix); ok {
		t.Fatal("wrong prefix should not match")
	}
	if _, ok := keyFromSubject(prefix, prefix); ok {
		t.Fatal("empty key should be rejected")
	}
}

func TestKvOpFromHeaders(t *testing.T) {
	t.Parallel()

	if op := kvOpFromHeaders(nil); op != jetstream.KeyValuePut {
		t.Fatalf("nil headers: %v", op)
	}
	if op := kvOpFromHeaders(nats.Header{"KV-Operation": []string{"DEL"}}); op != jetstream.KeyValueDelete {
		t.Fatalf("DEL: %v", op)
	}
	if op := kvOpFromHeaders(nats.Header{"KV-Operation": []string{"PURGE"}}); op != jetstream.KeyValuePurge {
		t.Fatalf("PURGE: %v", op)
	}
	if op := kvOpFromHeaders(nats.Header{jetstream.MarkerReasonHeader: []string{"MaxAge"}}); op != jetstream.KeyValuePurge {
		t.Fatalf("marker MaxAge: %v", op)
	}
	if op := kvOpFromHeaders(nats.Header{jetstream.MarkerReasonHeader: []string{"Remove"}}); op != jetstream.KeyValueDelete {
		t.Fatalf("marker Remove: %v", op)
	}
	if op := kvOpFromHeaders(nats.Header{"KV-Operation": []string{"PUT"}}); op != jetstream.KeyValuePut {
		t.Fatalf("default put: %v", op)
	}
}

func TestSnapshotKVFromStreamIntegration(t *testing.T) {
	srv := startNATSServer(t)

	nc := connectNATS(t, srv.ClientURL())
	defer nc.Close()

	js := newJetStream(t, nc)
	ctx := testContext(t)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "TEST"})
	if err != nil {
		t.Fatalf("create kv: %v", err)
	}
	if _, err := kv.Put(ctx, "active", []byte("value")); err != nil {
		t.Fatalf("put active: %v", err)
	}
	if _, err := kv.Put(ctx, "deleted", []byte("gone")); err != nil {
		t.Fatalf("put deleted: %v", err)
	}
	if err := kv.Delete(ctx, "deleted"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	snap, err := SnapshotKVFromStream(ctx, js, "TEST", nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Migratable) != 1 || snap.Migratable[0] != "active" {
		t.Fatalf("migratable: %v", snap.Migratable)
	}
	if len(snap.Omitted) != 1 || snap.Omitted[0] != "deleted" {
		t.Fatalf("omitted: %v", snap.Omitted)
	}
	if string(snap.Values["active"]) != "value" {
		t.Fatalf("values: %v", snap.Values)
	}
}

func TestSnapshotKVFromStreamEmpty(t *testing.T) {
	srv := startNATSServer(t)

	nc := connectNATS(t, srv.ClientURL())
	defer nc.Close()

	js := newJetStream(t, nc)
	ctx := testContext(t)

	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "EMPTY"}); err != nil {
		t.Fatalf("create kv: %v", err)
	}

	snap, err := SnapshotKVFromStream(ctx, js, "EMPTY", nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Listed) != 0 || snap.MessageCount != 0 {
		t.Fatalf("expected empty snapshot: %+v", snap)
	}
}

func TestSnapshotKVFromStreamMissingBucket(t *testing.T) {
	srv := startNATSServer(t)

	nc := connectNATS(t, srv.ClientURL())
	defer nc.Close()

	js := newJetStream(t, nc)
	if _, err := SnapshotKVFromStream(testContext(t), js, "NOPE", nil); err == nil {
		t.Fatal("expected error for missing bucket")
	}
}
