package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migrate"
)

func TestMigrateKVBucket(t *testing.T) {
	t.Parallel()

	dest := &memKeyValue{data: map[string][]byte{"existing": []byte("old")}}
	bar := migrate.NewProgress(false).StartBucket("KV", "test", 1, 1, 3, 1)

	stats, err := migrateKVBucket(
		context.Background(),
		dest,
		[]string{"new", "existing"},
		map[string][]byte{
			"new":      []byte("v1"),
			"existing": []byte("v2"),
		},
		true,
		1,
		bar,
	)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if stats.Migrated != 1 || stats.Skipped != 1 || stats.Total != 2 {
		t.Fatalf("stats: %+v", stats)
	}
	if string(dest.data["new"]) != "v1" {
		t.Fatalf("dest new key: %q", dest.data["new"])
	}
	if string(dest.data["existing"]) != "old" {
		t.Fatalf("skipped key should be unchanged: %q", dest.data["existing"])
	}
}

func TestMigrateKVBucketMissingValue(t *testing.T) {
	t.Parallel()

	dest := &memKeyValue{data: map[string][]byte{}}
	bar := migrate.NewProgress(false).StartBucket("KV", "test", 1, 1, 1, 1)

	_, err := migrateKVBucket(
		context.Background(),
		dest,
		[]string{"k1"},
		map[string][]byte{},
		false,
		1,
		bar,
	)
	if err == nil {
		t.Fatal("expected error for missing scanned value")
	}
}

type memKeyValue struct {
	data map[string][]byte
}

func (m *memKeyValue) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	value, ok := m.data[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return &memKeyValueEntry{key: key, value: bytes.Clone(value)}, nil
}

func (m *memKeyValue) Put(_ context.Context, key string, value []byte) (uint64, error) {
	m.data[key] = bytes.Clone(value)
	return 1, nil
}

func (m *memKeyValue) GetRevision(context.Context, string, uint64) (jetstream.KeyValueEntry, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) PutString(context.Context, string, string) (uint64, error) {
	return 0, errors.New("unimplemented")
}
func (m *memKeyValue) Create(context.Context, string, []byte, ...jetstream.KVCreateOpt) (uint64, error) {
	return 0, errors.New("unimplemented")
}
func (m *memKeyValue) Update(context.Context, string, []byte, uint64) (uint64, error) {
	return 0, errors.New("unimplemented")
}
func (m *memKeyValue) Delete(context.Context, string, ...jetstream.KVDeleteOpt) error {
	return errors.New("unimplemented")
}
func (m *memKeyValue) Purge(context.Context, string, ...jetstream.KVDeleteOpt) error {
	return errors.New("unimplemented")
}
func (m *memKeyValue) Watch(context.Context, string, ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) WatchAll(context.Context, ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) WatchFiltered(context.Context, []string, ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) Keys(context.Context, ...jetstream.WatchOpt) ([]string, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) ListKeys(context.Context, ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) ListKeysFiltered(context.Context, ...string) (jetstream.KeyLister, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) History(context.Context, string, ...jetstream.WatchOpt) ([]jetstream.KeyValueEntry, error) {
	return nil, errors.New("unimplemented")
}
func (m *memKeyValue) Bucket() string { return "mem" }
func (m *memKeyValue) PurgeDeletes(context.Context, ...jetstream.KVPurgeOpt) error {
	return errors.New("unimplemented")
}
func (m *memKeyValue) Status(context.Context) (jetstream.KeyValueStatus, error) {
	return nil, errors.New("unimplemented")
}

type memKeyValueEntry struct {
	key   string
	value []byte
}

func (e *memKeyValueEntry) Bucket() string                    { return "mem" }
func (e *memKeyValueEntry) Key() string                       { return e.key }
func (e *memKeyValueEntry) Value() []byte                     { return e.value }
func (e *memKeyValueEntry) Revision() uint64                  { return 1 }
func (e *memKeyValueEntry) Created() time.Time                { return time.Now() }
func (e *memKeyValueEntry) Delta() uint64                     { return 0 }
func (e *memKeyValueEntry) Operation() jetstream.KeyValueOp   { return jetstream.KeyValuePut }

func TestListKVBuckets(t *testing.T) {
	srv := startNATSServer(t)
	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx := testContext(t)

	bucket := "listkv-selected"
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "listkv-other"}); err != nil {
		t.Fatal(err)
	}

	buckets, err := listKVBuckets(ctx, js, migrate.BaseConfig{
		Buckets: map[string]struct{}{bucket: {}},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Bucket() != bucket {
		t.Fatalf("buckets: %+v", buckets)
	}
}

func startNATSServer(t *testing.T) *server.Server {
	t.Helper()
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := test.RunServer(&opts)
	t.Cleanup(func() { srv.Shutdown() })
	return srv
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}
