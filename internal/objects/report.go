package objects

import (
	"fmt"

	"github.com/sabinadams/natsmith/internal/report"
)

func ScanFailMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindObjectStore, bucket, index, total, "failed to scan bucket", err)
}

func DryRunScanMessage(bucket string, index, total int, snap BucketSnapshot, metaOmitted int) string {
	return report.BucketInfo(report.KindObjectStore, bucket, index, total, fmt.Sprintf(
		"%d listed, %d meta-active, %d meta-omitted (%d meta messages)",
		len(snap.Listed), len(snap.Migratable), metaOmitted, snap.MessageCount,
	))
}

func ScanOKMessage(bucket string, index, total int, listed, migratable, omitted, messages int) string {
	return report.BucketInfo(report.KindObjectStore, bucket, index, total, fmt.Sprintf(
		"%d listed, %d migratable, %d omitted (%d meta messages)",
		listed, migratable, omitted, messages,
	))
}

func DestBucketMissingMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindObjectStore, bucket, index, total, "destination bucket not found", err)
}

func SourceOpenFailMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindObjectStore, bucket, index, total, "failed to open source", err)
}

func CopyFailMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindObjectStore, bucket, index, total, "migration failed", err)
}

func CopyCountMismatchMessage(bucket string, total, migrated, skipped, failed int) string {
	return report.BucketIssue(report.KindObjectStore, bucket, fmt.Sprintf(
		"expected %d processed, got migrated=%d skipped=%d failed=%d",
		total, migrated, skipped, failed,
	))
}
