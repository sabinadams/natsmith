package kv

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestMigrateKVBucket(t *testing.T) {
	t.Parallel()

	dest := &memKeyValue{data: map[string][]byte{"existing": []byte("old")}}
	bar := progress.NewProgress(false).StartBucket("KV", "test", 1, 1, 3, 1)

	stats, err := CopyBucket(
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
	bar := progress.NewProgress(false).StartBucket("KV", "test", 1, 1, 1, 1)

	_, err := CopyBucket(
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

func TestCopyBucketSkipExistingGetError(t *testing.T) {
	t.Parallel()

	dest := &errOnGetKeyValue{
		memKeyValue: memKeyValue{data: map[string][]byte{}},
		getErr:      errors.New("get failed"),
	}
	bar := progress.NewProgress(false).StartBucket("KV", "test", 1, 1, 1, 1)

	_, err := CopyBucket(
		context.Background(),
		dest,
		[]string{"k1"},
		map[string][]byte{"k1": []byte("v")},
		true,
		1,
		bar,
	)
	if err == nil || !strings.Contains(err.Error(), "check destination key") {
		t.Fatalf("expected check error, got %v", err)
	}
}

type errOnGetKeyValue struct {
	memKeyValue
	getErr error
}

func (m *errOnGetKeyValue) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	return nil, m.getErr
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

func (e *memKeyValueEntry) Bucket() string                  { return "mem" }
func (e *memKeyValueEntry) Key() string                     { return e.key }
func (e *memKeyValueEntry) Value() []byte                   { return e.value }
func (e *memKeyValueEntry) Revision() uint64                { return 1 }
func (e *memKeyValueEntry) Created() time.Time              { return time.Now() }
func (e *memKeyValueEntry) Delta() uint64                   { return 0 }
func (e *memKeyValueEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }

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
