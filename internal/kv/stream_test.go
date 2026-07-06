package kv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStreamName(t *testing.T) {
	t.Parallel()

	if got := StreamName("schema"); got != "KV_schema" {
		t.Fatalf("StreamName = %q", got)
	}
	bucket, ok := BucketFromStreamName("KV_schema")
	if !ok || bucket != "schema" {
		t.Fatalf("BucketFromStreamName = %q, %v", bucket, ok)
	}
	if _, ok := BucketFromStreamName("OBJ_files"); ok {
		t.Fatal("expected non-KV stream to be rejected")
	}
}

func TestDiscoverBackupDirsSingle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "backup.json"), []byte(`{"config":{"name":"KV_a"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	dirs, err := DiscoverBackupDirs(root)
	if err != nil || len(dirs) != 1 || dirs[0] != root {
		t.Fatalf("dirs=%v err=%v", dirs, err)
	}
}

func TestDiscoverBackupDirsNested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, bucket := range []string{"schema", "shorturl"} {
		dir := filepath.Join(root, bucket)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "backup.json"), []byte(`{"config":{"name":"KV_`+bucket+`"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := DiscoverBackupDirs(root)
	if err != nil || len(dirs) != 2 {
		t.Fatalf("dirs=%v err=%v", dirs, err)
	}
}

func TestFilterBackupDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, bucket := range []string{"keep", "skip"} {
		dir := filepath.Join(root, bucket)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "backup.json"), []byte(`{"config":{"name":"KV_`+bucket+`"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	all, err := DiscoverBackupDirs(root)
	if err != nil {
		t.Fatal(err)
	}

	filtered, err := FilterBackupDirs(all, func(bucket string) bool {
		return bucket == "keep"
	})
	if err != nil || len(filtered) != 1 {
		t.Fatalf("filtered=%v err=%v", filtered, err)
	}
}
