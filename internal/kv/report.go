package kv

import "fmt"

func ScanOKMessage(bucket string, index, total int, snap BucketSnapshot) string {
	return fmt.Sprintf(
		"  · KV %s (%d/%d) — %d listed, %d migratable, %d omitted (%d stream messages)",
		bucket, index, total, len(snap.Listed), len(snap.Migratable), len(snap.Omitted), snap.MessageCount,
	)
}

func ScanFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ KV %s (%d/%d) — failed to scan bucket: %v", bucket, index, total, err)
}

func DestBucketMissingMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ KV %s (%d/%d) — destination bucket not found: %v", bucket, index, total, err)
}

func CopyFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ KV %s (%d/%d) — migration failed: %v", bucket, index, total, err)
}

func CopyCountMismatchMessage(bucket string, expected, migrated, skipped int) string {
	return fmt.Sprintf(
		"  ✗ KV %s — expected %d migrated, got %d (skipped=%d)",
		bucket, expected, migrated, skipped,
	)
}

func VerifyFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ KV %s (%d/%d) — verify failed: %v", bucket, index, total, err)
}

func FailuresFileErrorMessage(bucket string, err error) string {
	return fmt.Sprintf("  ✗ KV %s — failed to write failures file: %v", bucket, err)
}
