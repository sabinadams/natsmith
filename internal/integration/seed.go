//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/sabinadams/natsmith/internal/nats"
)

// BaseConfig builds a migration BaseConfig for integration tests.
func BaseConfig(t *testing.T, sourceURL, destURL string) migration.BaseConfig {
	t.Helper()

	cfg := migration.BaseConfig{
		SourceURL:      sourceURL,
		DestURL:        destURL,
		Workers:        1,
		NoProgress:     true,
		RequestTimeout: 30 * time.Second,
	}
	if err := migration.ValidateBaseConfig(cfg); err != nil {
		t.Fatalf("base config: %v", err)
	}
	return cfg
}

// JetStream connects to url and returns a JetStream context.
func JetStream(t *testing.T, url string) jetstream.JetStream {
	t.Helper()

	nc, js, err := nats.Connect(url, "", 30*time.Second)
	if err != nil {
		t.Fatalf("connect %s: %v", url, err)
	}
	t.Cleanup(func() { nc.Close() })
	return js
}

// CreateKV creates a KV bucket on the server at url.
func CreateKV(t *testing.T, url, bucket string) jetstream.KeyValue {
	t.Helper()

	js := JetStream(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: bucket})
	if err != nil {
		t.Fatalf("create kv %q on %s: %v", bucket, url, err)
	}
	return kv
}

// PutKV writes keys on an existing KV bucket handle.
func PutKV(t *testing.T, kv jetstream.KeyValue, values map[string][]byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	for key, value := range values {
		if _, err := kv.Put(ctx, key, value); err != nil {
			t.Fatalf("put kv key %q: %v", key, err)
		}
	}
}

// CreateObjectStore creates an object store bucket on the server at url.
func CreateObjectStore(t *testing.T, url, bucket string) jetstream.ObjectStore {
	t.Helper()

	js := JetStream(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	os, err := js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: bucket})
	if err != nil {
		t.Fatalf("create object store %q on %s: %v", bucket, url, err)
	}
	return os
}

// PutObjectBytes stores an object on the given bucket handle.
func PutObjectBytes(t *testing.T, store jetstream.ObjectStore, name string, data []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	if _, err := store.PutBytes(ctx, name, data); err != nil {
		t.Fatalf("put object %q: %v", name, err)
	}
}
