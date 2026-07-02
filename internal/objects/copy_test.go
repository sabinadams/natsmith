package objects

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestCopyObject(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	source, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "SRC"})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "DEST"})
	if err != nil {
		t.Fatal(err)
	}

	info, err := source.PutBytes(ctx, "file.txt", []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	info.Description = "desc"
	info.Headers = nats.Header{"X-Test": []string{"1"}}
	info.Metadata = map[string]string{"k": "v"}

	if err := copyObject(ctx, source, dest, info); err != nil {
		t.Fatalf("copy: %v", err)
	}

	got, err := dest.Get(ctx, "file.txt")
	if err != nil {
		t.Fatalf("get dest: %v", err)
	}
	defer got.Close()
	data, err := io.ReadAll(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("payload")) {
		t.Fatalf("data: %q", data)
	}
}

func TestCopyLinkObject(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	targetBucket, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "TARGET"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := targetBucket.PutBytes(ctx, "target.txt", []byte("target")); err != nil {
		t.Fatal(err)
	}

	source, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "SRC-LINK"})
	if err != nil {
		t.Fatal(err)
	}
	_ = source
	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "DEST-LINK"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "TARGET-DEST"}); err != nil {
		t.Fatal(err)
	}

	objectLink := &jetstream.ObjectInfo{
		ObjectMeta: jetstream.ObjectMeta{
			Name: "obj-link",
			Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Bucket: "TARGET", Name: "target.txt"}},
		},
	}
	if err := copyLink(ctx, js, dest, objectLink); err != nil {
		t.Fatalf("object link: %v", err)
	}

	bucketLink := &jetstream.ObjectInfo{
		ObjectMeta: jetstream.ObjectMeta{
			Name: "bucket-link",
			Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Bucket: "TARGET-DEST"}},
		},
	}
	if err := copyLink(ctx, js, dest, bucketLink); err != nil {
		t.Fatalf("bucket link: %v", err)
	}

	if info, err := dest.GetInfo(ctx, "obj-link"); err != nil || !IsLink(info) {
		t.Fatalf("object link dest info: %v err=%v", info, err)
	}
	if info, err := dest.GetInfo(ctx, "bucket-link"); err != nil || !IsLink(info) {
		t.Fatalf("bucket link dest info: %v err=%v", info, err)
	}
}

func TestMigrateDataObjectSkipExisting(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	source, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "SRC"})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "DEST"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.PutBytes(ctx, "file.txt", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.PutBytes(ctx, "file.txt", []byte("existing")); err != nil {
		t.Fatal(err)
	}

	info, err := source.GetInfo(ctx, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	result := copyDataObject(ctx, time.Minute, source, dest, info, true)
	if !result.skipped || result.migrated || result.failed {
		t.Fatalf("result: %+v", result)
	}
}

func TestMigrateObjectsParallelRecordsFailure(t *testing.T) {
	bar := progress.NewProgress(false).StartBucket("Object store", "test", 1, 1, 1, 1)
	stats := progress.ItemStats{Total: 1}

	err := copyParallel(context.Background(), 1, []*jetstream.ObjectInfo{
		{ObjectMeta: jetstream.ObjectMeta{Name: "missing"}},
	}, bar, &stats, func(ctx context.Context, info *jetstream.ObjectInfo) copyResult {
		return copyResult{failed: true}
	})
	if err != nil {
		t.Fatalf("parallel: %v", err)
	}
	if stats.Failed != 1 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestListObjectBuckets(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	for _, bucket := range []string{"keep", "skip"} {
		if _, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: bucket}); err != nil {
			t.Fatal(err)
		}
	}

	buckets, err := ListBuckets(ctx, js, migration.BaseConfig{
		Buckets: map[string]struct{}{"keep": {}, "skip": {}},
		Omit:    map[string]struct{}{"skip": {}},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Bucket() != "keep" {
		t.Fatalf("buckets: %+v", buckets)
	}
}

func TestMigrateObjectBucket(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	source, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "MIG-SRC"})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "MIG-DEST"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.PutBytes(ctx, "file.txt", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	info, err := source.GetInfo(ctx, "file.txt")
	if err != nil {
		t.Fatal(err)
	}

	bar := progress.NewProgress(false).StartBucket("Object store", "test", 1, 1, 1, 1)
	stats, err := CopyBucket(ctx, time.Minute, js, source, dest, []*jetstream.ObjectInfo{info}, false, 1, bar)
	if err != nil {
		t.Fatalf("migrate bucket: %v", err)
	}
	if stats.Migrated != 1 || stats.Total != 1 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestMigrateLinkObject(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "LINK-DEST"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "LINK-TARGET"}); err != nil {
		t.Fatal(err)
	}

	info := &jetstream.ObjectInfo{
		ObjectMeta: jetstream.ObjectMeta{
			Name: "bucket-link",
			Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Bucket: "LINK-TARGET"}},
		},
	}
	result := copyLinkObject(ctx, js, dest, info, false)
	if !result.migrated || result.failed || result.skipped {
		t.Fatalf("result: %+v", result)
	}
}

func TestCopyLinkObjectSkipExisting(t *testing.T) {
	srv := testutil.StartServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)

	dest, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "LINK-SKIP-DEST"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: "LINK-SKIP-TARGET"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dest.AddBucketLink(ctx, "bucket-link", target); err != nil {
		t.Fatal(err)
	}

	info := &jetstream.ObjectInfo{
		ObjectMeta: jetstream.ObjectMeta{
			Name: "bucket-link",
			Opts: &jetstream.ObjectMetaOptions{Link: &jetstream.ObjectLink{Bucket: "LINK-SKIP-TARGET"}},
		},
	}
	result := copyLinkObject(ctx, js, dest, info, true)
	if !result.skipped || result.migrated || result.failed {
		t.Fatalf("result: %+v", result)
	}
}
