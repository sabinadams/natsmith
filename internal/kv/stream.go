package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

const kvStreamPrefix = "KV_"

// StreamName returns the JetStream stream backing a KV bucket.
func StreamName(bucket string) string {
	return kvStreamPrefix + bucket
}

// BucketFromStreamName returns the KV bucket name from a stream name.
func BucketFromStreamName(stream string) (string, bool) {
	bucket, ok := strings.CutPrefix(stream, kvStreamPrefix)
	return bucket, ok && bucket != ""
}

type backupMetadata struct {
	Config api.StreamConfig `json:"config"`
	State  api.StreamState  `json:"state"`
}

// BackupResult summarizes one bucket backup.
type BackupResult struct {
	Bucket   string
	Stream   string
	Dir      string
	Messages uint64
	Bytes    uint64
}

// RestoreResult summarizes one bucket restore.
type RestoreResult struct {
	Bucket   string
	Stream   string
	Dir      string
	Messages uint64
	Bytes    uint64
}

// TransferProgress reports byte transfer for backup or restore.
type TransferProgress struct {
	Sent  int64
	Total int64
}

// TransferReporter receives throttled transfer updates.
type TransferReporter func(TransferProgress)

// DataFileSize returns the size of stream.tar.s2 in a backup directory.
func DataFileSize(dir string) (int64, error) {
	fi, err := os.Stat(filepath.Join(dir, "stream.tar.s2"))
	if err != nil {
		return 0, fmt.Errorf("stat stream.tar.s2: %w", err)
	}
	return fi.Size(), nil
}

// BackupBucket snapshots the backing stream into dir (creates backup.json + stream.tar.s2).
func BackupBucket(
	ctx context.Context,
	mgr *jsm.Manager,
	bucket, dir string,
	showProgress bool,
	report TransferReporter,
) (BackupResult, error) {
	streamName := StreamName(bucket)

	stream, err := mgr.LoadStream(streamName)
	if err != nil {
		return BackupResult{}, fmt.Errorf("open stream %q: %w", streamName, err)
	}

	var expectedBytes int64
	if info, err := stream.Information(); err == nil {
		expectedBytes = int64(info.State.Bytes)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return BackupResult{}, fmt.Errorf("create backup dir: %w", err)
	}

	var opts []jsm.SnapshotOption
	if showProgress {
		opts = append(opts, jsm.SnapshotNotify(func(p jsm.SnapshotProgress) {
			if report == nil {
				return
			}
			if p.Finished() {
				return
			}
			total := int64(p.BytesExpected())
			if total <= 0 {
				total = expectedBytes
			}
			report(TransferProgress{
				Sent:  int64(p.BytesReceived()),
				Total: total,
			})
		}))
	}

	if _, err := stream.SnapshotToDirectory(ctx, dir, opts...); err != nil {
		return BackupResult{}, fmt.Errorf("snapshot stream %q: %w", streamName, err)
	}

	meta, err := ReadBackupMetadata(dir)
	if err != nil {
		return BackupResult{}, err
	}

	return BackupResult{
		Bucket:   bucket,
		Stream:   streamName,
		Dir:      dir,
		Messages: meta.State.Msgs,
		Bytes:    meta.State.Bytes,
	}, nil
}

// RestoreBucket restores a stream snapshot from dir into the cluster.
func RestoreBucket(
	ctx context.Context,
	mgr *jsm.Manager,
	dir string,
	force bool,
	replicas int,
	showProgress bool,
	report TransferReporter,
) (RestoreResult, error) {
	meta, err := ReadBackupMetadata(dir)
	if err != nil {
		return RestoreResult{}, err
	}

	streamName := meta.Config.Name
	bucket, ok := BucketFromStreamName(streamName)
	if !ok {
		return RestoreResult{}, fmt.Errorf("backup %q is not a KV stream (expected name KV_<bucket>)", streamName)
	}

	known, err := mgr.IsKnownStream(streamName)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("check stream %q: %w", streamName, err)
	}
	if known {
		if !force {
			return RestoreResult{}, fmt.Errorf("stream %q already exists (use --force to replace)", streamName)
		}
		stream, err := mgr.LoadStream(streamName)
		if err != nil {
			return RestoreResult{}, fmt.Errorf("load stream %q: %w", streamName, err)
		}
		if err := stream.Delete(); err != nil {
			return RestoreResult{}, fmt.Errorf("delete stream %q: %w", streamName, err)
		}
	}

	cfg := meta.Config
	if replicas > 0 {
		cfg.Replicas = replicas
	}

	var opts []jsm.SnapshotOption
	opts = append(opts, jsm.RestoreConfiguration(cfg))
	if showProgress {
		opts = append(opts, jsm.RestoreNotify(func(p jsm.RestoreProgress) {
			if report == nil {
				return
			}
			total := int64(p.ChunksToSend() * p.ChunkSize())
			report(TransferProgress{
				Sent:  int64(p.BytesSent()),
				Total: total,
			})
		}))
	}

	// JSM starts a trackBps goroutine tied to ctx; cancel when this restore
	// finishes so completed buckets stop emitting progress callbacks.
	restoreCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, state, err := mgr.RestoreSnapshotFromDirectory(restoreCtx, streamName, dir, opts...)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore stream %q: %w", streamName, err)
	}

	result := RestoreResult{
		Bucket: bucket,
		Stream: streamName,
		Dir:    dir,
	}
	if state != nil {
		result.Messages = state.Msgs
		result.Bytes = state.Bytes
	}
	return result, nil
}

// ReadBackupMetadata reads backup.json from a snapshot directory.
func ReadBackupMetadata(dir string) (backupMetadata, error) {
	path := filepath.Join(dir, "backup.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return backupMetadata{}, fmt.Errorf("read %s: %w", path, err)
	}

	var meta backupMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return backupMetadata{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if meta.Config.Name == "" {
		return backupMetadata{}, fmt.Errorf("%s: missing stream name", path)
	}
	return meta, nil
}

// DiscoverBackupDirs returns snapshot directories under root.
// If root itself contains backup.json, it is returned alone.
func DiscoverBackupDirs(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, "backup.json")); err == nil {
		return []string{root}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read backup dir %q: %w", root, err)
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "backup.json")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no backups found under %q (expected backup.json in dir or subdirs)", root)
	}
	return dirs, nil
}

// BackupDirForBucket returns the output directory for a bucket under root.
func BackupDirForBucket(root, bucket string) string {
	return filepath.Join(root, bucket)
}

// FilterBackupDirs keeps dirs whose bucket name passes the filter.
func FilterBackupDirs(dirs []string, include func(bucket string) bool) ([]string, error) {
	var filtered []string
	for _, dir := range dirs {
		meta, err := ReadBackupMetadata(dir)
		if err != nil {
			return nil, err
		}
		bucket, ok := BucketFromStreamName(meta.Config.Name)
		if !ok {
			continue
		}
		if include(bucket) {
			filtered = append(filtered, dir)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no matching KV backups found")
	}
	return filtered, nil
}
