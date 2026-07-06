package kv

import (
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestListKVBuckets(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	bucket := "listkv-selected"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "listkv-other"}); err != nil {
		t.Fatal(err)
	}

	buckets, err := ListBuckets(ctx, js, migration.BaseConfig{
		Buckets: map[string]struct{}{bucket: {}},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Bucket() != bucket {
		t.Fatalf("buckets: %+v", buckets)
	}
}
