package objects

import "fmt"

func ScanFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ Object store %s (%d/%d) — failed to scan bucket: %v", bucket, index, total, err)
}

func DryRunScanMessage(bucket string, index, total int, snap BucketSnapshot, metaOmitted int) string {
	return fmt.Sprintf(
		"  · Object store %s (%d/%d) — %d listed, %d meta-active, %d meta-omitted (%d meta messages)",
		bucket, index, total, len(snap.Listed), len(snap.Migratable), metaOmitted, snap.MessageCount,
	)
}

func ScanOKMessage(bucket string, index, total int, listed, migratable, omitted, messages int) string {
	return fmt.Sprintf(
		"  · Object store %s (%d/%d) — %d listed, %d migratable, %d omitted (%d meta messages)",
		bucket, index, total, listed, migratable, omitted, messages,
	)
}

func DestBucketMissingMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ Object store %s (%d/%d) — destination bucket not found: %v", bucket, index, total, err)
}

func SourceOpenFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ Object store %s (%d/%d) — failed to open source: %v", bucket, index, total, err)
}

func CopyFailMessage(bucket string, index, total int, err error) string {
	return fmt.Sprintf("  ✗ Object store %s (%d/%d) — migration failed: %v", bucket, index, total, err)
}

func CopyCountMismatchMessage(bucket string, total, migrated, skipped, failed int) string {
	return fmt.Sprintf(
		"  ✗ Object store %s — expected %d processed, got migrated=%d skipped=%d failed=%d",
		bucket, total, migrated, skipped, failed,
	)
}
