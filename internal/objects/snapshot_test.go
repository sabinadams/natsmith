package objects

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestObjectNameFromSubject(t *testing.T) {
	t.Parallel()

	prefix := "$O.bucket.M."
	encoded := base64.URLEncoding.EncodeToString([]byte("my/file.pdf"))
	name, ok := objectNameFromSubject(prefix+encoded, prefix)
	if !ok || name != "my/file.pdf" {
		t.Fatalf("got %q ok=%v", name, ok)
	}
	if _, ok := objectNameFromSubject("$O.other.M.x", prefix); ok {
		t.Fatal("wrong bucket prefix should not match")
	}
	if _, ok := objectNameFromSubject(prefix, prefix); ok {
		t.Fatal("empty encoded name should fail")
	}
	if _, ok := objectNameFromSubject(prefix+"%%%", prefix); ok {
		t.Fatal("invalid base64 should fail")
	}
}

func TestObjectMetaMigratable(t *testing.T) {
	t.Parallel()

	if objectMetaMigratable(&jetstream.ObjectInfo{Deleted: true, NUID: "n1"}) {
		t.Fatal("deleted meta should not be migratable")
	}
	if !objectMetaMigratable(&jetstream.ObjectInfo{NUID: "n1"}) {
		t.Fatal("active object with NUID should be migratable")
	}
	if !objectMetaMigratable(&jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Bucket: "other"}}}}) {
		t.Fatal("link should be migratable")
	}
	if objectMetaMigratable(&jetstream.ObjectInfo{}) {
		t.Fatal("empty meta should be omitted")
	}
}

func TestIsLink(t *testing.T) {
	t.Parallel()

	if !IsLink(&jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Name: "x"}}}}) {
		t.Fatal("expected link")
	}
	if IsLink(&jetstream.ObjectInfo{}) {
		t.Fatal("expected not link")
	}
}

func TestCopyTimeout(t *testing.T) {
	t.Parallel()

	if got := CopyTimeout(30 * time.Second); got != 5*time.Minute {
		t.Fatalf("got %v", got)
	}
	if want := 10 * time.Minute; CopyTimeout(want) != want {
		t.Fatalf("large timeout should pass through")
	}
}

func TestFilterRetrievableObjectsIntegration(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	os, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "FILTER"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.PutBytes(ctx, "exists", []byte("data")); err != nil {
		t.Fatal(err)
	}

	candidates := []*jetstream.ObjectInfo{
		{ObjectMeta: jetstream.ObjectMeta{Name: "exists"}},
		{ObjectMeta: jetstream.ObjectMeta{Name: "ghost"}},
	}
	migratable, omitted := FilterRetrievableObjects(ctx, os, candidates)
	if len(migratable) != 1 || migratable[0].Name != "exists" {
		t.Fatalf("migratable: %+v", migratable)
	}
	if len(omitted) != 1 || omitted[0] != "ghost" {
		t.Fatalf("omitted: %v", omitted)
	}
}

func TestSnapshotFromStreamEmpty(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	if _, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "EMPTY"}); err != nil {
		t.Fatalf("create object store: %v", err)
	}

	snap, err := SnapshotFromStream(ctx, js, "EMPTY", nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Listed) != 0 {
		t.Fatalf("expected empty snapshot: %+v", snap)
	}
}

func TestSnapshotFromStreamWithObject(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	os, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "DATA"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.PutBytes(ctx, "file.txt", []byte("hello")); err != nil {
		t.Fatal(err)
	}

	snap, err := SnapshotFromStream(ctx, js, "DATA", nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Migratable) != 1 || snap.Migratable[0].Name != "file.txt" {
		t.Fatalf("migratable: %+v", snap.Migratable)
	}
}
