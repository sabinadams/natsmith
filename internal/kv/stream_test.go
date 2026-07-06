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

func TestFilterBackupDirsNoMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "schema")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backup.json"), []byte(`{"config":{"name":"KV_schema"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	all, err := DiscoverBackupDirs(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = FilterBackupDirs(all, func(string) bool { return false })
	if err == nil {
		t.Fatal("expected error when no backups match filter")
	}
}

func TestDataFileSize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := []byte("snapshot-data")
	if err := os.WriteFile(filepath.Join(dir, "stream.tar.s2"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	size, err := DataFileSize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(want)) {
		t.Fatalf("size = %d, want %d", size, len(want))
	}
}

func TestDataFileSizeMissing(t *testing.T) {
	t.Parallel()

	if _, err := DataFileSize(t.TempDir()); err == nil {
		t.Fatal("expected error for missing stream.tar.s2")
	}
}

func TestReadBackupMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "backup.json"), []byte(`{"config":{"name":"KV_schema"},"state":{"messages":3}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	meta, err := ReadBackupMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Config.Name != "KV_schema" || meta.State.Msgs != 3 {
		t.Fatalf("meta: %+v", meta)
	}
}

func TestReadBackupMetadataInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "backup.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadBackupMetadata(dir); err == nil {
		t.Fatal("expected error for missing stream name")
	}
}

func TestBackupDirForBucket(t *testing.T) {
	t.Parallel()

	got := BackupDirForBucket("/backups", "schema")
	want := filepath.Join("/backups", "schema")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
