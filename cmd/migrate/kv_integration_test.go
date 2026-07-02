//go:build integration

package migrate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/integration"
	"github.com/sabinadams/natsmith/internal/migration"
)

func TestKVMigrationAcrossContainers(t *testing.T) {
	const bucket = "MIGRATE_KV"

	pair := integration.StartNATSPair(t)
	base := integration.BaseConfig(t, pair.SourceURL, pair.DestURL)

	sourceKV := integration.CreateKV(t, pair.SourceURL, bucket)
	integration.PutKV(t, sourceKV, map[string][]byte{
		"active":   []byte("value-1"),
		"other":    []byte("value-2"),
		"deleted":  []byte("gone"),
	})
	if err := sourceKV.Delete(context.Background(), "deleted"); err != nil {
		t.Fatalf("delete key: %v", err)
	}

	integration.CreateKV(t, pair.DestURL, bucket)

	cfg := migration.NewKVConfig(base, true, false, "")
	if err := runKV(cfg); err != nil {
		t.Fatalf("runKV: %v", err)
	}

	destJS := integration.JetStream(t, pair.DestURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	destKV, err := destJS.KeyValue(ctx, bucket)
	if err != nil {
		t.Fatalf("open dest kv: %v", err)
	}

	for key, want := range map[string]string{
		"active": "value-1",
		"other":  "value-2",
	} {
		entry, err := destKV.Get(ctx, key)
		if err != nil {
			t.Fatalf("get dest key %q: %v", key, err)
		}
		if string(entry.Value()) != want {
			t.Fatalf("dest key %q = %q, want %q", key, entry.Value(), want)
		}
	}

	if _, err := destKV.Get(ctx, "deleted"); !errors.Is(err, jetstream.ErrKeyNotFound) {
		t.Fatalf("deleted key should be absent on dest, got err=%v", err)
	}
}

func TestKVSkipExistingAcrossContainers(t *testing.T) {
	const bucket = "MIGRATE_KV_SKIP"

	pair := integration.StartNATSPair(t)
	base := integration.BaseConfig(t, pair.SourceURL, pair.DestURL)
	base.SkipExisting = true

	sourceKV := integration.CreateKV(t, pair.SourceURL, bucket)
	integration.PutKV(t, sourceKV, map[string][]byte{
		"shared": []byte("source-value"),
		"new":    []byte("only-source"),
	})

	destKV := integration.CreateKV(t, pair.DestURL, bucket)
	integration.PutKV(t, destKV, map[string][]byte{
		"shared": []byte("dest-value"),
	})

	cfg := migration.NewKVConfig(base, false, false, "")
	if err := runKV(cfg); err != nil {
		t.Fatalf("runKV: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shared, err := destKV.Get(ctx, "shared")
	if err != nil {
		t.Fatalf("get shared: %v", err)
	}
	if string(shared.Value()) != "dest-value" {
		t.Fatalf("skip-existing should preserve dest value, got %q", shared.Value())
	}

	newKey, err := destKV.Get(ctx, "new")
	if err != nil {
		t.Fatalf("get new: %v", err)
	}
	if string(newKey.Value()) != "only-source" {
		t.Fatalf("new key = %q, want %q", newKey.Value(), "only-source")
	}
}
