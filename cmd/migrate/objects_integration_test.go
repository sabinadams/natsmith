//go:build integration

package migrate

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/integration"
	"github.com/sabinadams/natsmith/internal/migration"
)

func TestObjectsMigrationAcrossContainers(t *testing.T) {
	const bucket = "MIGRATE_OBJ"

	pair := integration.StartNATSPair(t)
	base := integration.BaseConfig(t, pair.SourceURL, pair.DestURL)

	sourceOS := integration.CreateObjectStore(t, pair.SourceURL, bucket)
	integration.PutObjectBytes(t, sourceOS, "notes.txt", []byte("hello from source"))
	integration.PutObjectBytes(t, sourceOS, "data.bin", []byte{0x01, 0x02, 0x03})

	integration.CreateObjectStore(t, pair.DestURL, bucket)

	cfg := migration.NewObjectConfig(base)
	if err := runObjects(cfg); err != nil {
		t.Fatalf("runObjects: %v", err)
	}

	destJS := integration.JetStream(t, pair.DestURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	destOS, err := destJS.ObjectStore(ctx, bucket)
	if err != nil {
		t.Fatalf("open dest object store: %v", err)
	}

	for name, want := range map[string][]byte{
		"notes.txt": []byte("hello from source"),
		"data.bin":  {0x01, 0x02, 0x03},
	} {
		obj, err := destOS.Get(ctx, name)
		if err != nil {
			t.Fatalf("get dest object %q: %v", name, err)
		}
		data, err := io.ReadAll(obj)
		obj.Close()
		if err != nil {
			t.Fatalf("read dest object %q: %v", name, err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("dest object %q = %q, want %q", name, data, want)
		}
	}
}

func TestObjectsSkipExistingAcrossContainers(t *testing.T) {
	const bucket = "MIGRATE_OBJ_SKIP"

	pair := integration.StartNATSPair(t)
	base := integration.BaseConfig(t, pair.SourceURL, pair.DestURL)
	base.SkipExisting = true

	sourceOS := integration.CreateObjectStore(t, pair.SourceURL, bucket)
	integration.PutObjectBytes(t, sourceOS, "shared.txt", []byte("source"))
	integration.PutObjectBytes(t, sourceOS, "new.txt", []byte("new"))

	destOS := integration.CreateObjectStore(t, pair.DestURL, bucket)
	integration.PutObjectBytes(t, destOS, "shared.txt", []byte("dest"))

	cfg := migration.NewObjectConfig(base)
	if err := runObjects(cfg); err != nil {
		t.Fatalf("runObjects: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shared, err := destOS.Get(ctx, "shared.txt")
	if err != nil {
		t.Fatalf("get shared: %v", err)
	}
	data, err := io.ReadAll(shared)
	shared.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "dest" {
		t.Fatalf("skip-existing should preserve dest object, got %q", data)
	}

	newObj, err := destOS.Get(ctx, "new.txt")
	if err != nil {
		t.Fatalf("get new: %v", err)
	}
	data, err = io.ReadAll(newObj)
	newObj.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("new object = %q, want %q", data, "new")
	}
}
