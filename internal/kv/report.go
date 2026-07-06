package kv

import (
	"fmt"

	"github.com/sabinadams/natsmith/internal/report"
)

func ScanOKRunMessage(bucket string, index, total int, run BucketRunResult) string {
	detail := fmt.Sprintf("%d migratable keys", run.Migratable)
	if run.GhostSkipped > 0 {
		detail = fmt.Sprintf("%s (%d ghost skipped)", detail, run.GhostSkipped)
	}
	return report.BucketInfo(report.KindKV, bucket, index, total, detail)
}

func ScanFailMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindKV, bucket, index, total, "failed", err)
}

func DestBucketMissingMessage(bucket string, index, total int, err error) string {
	return report.BucketError(report.KindKV, bucket, index, total, "destination bucket not found", err)
}

func CopyCountMismatchMessage(bucket string, expected, migrated, skipped int) string {
	return report.BucketIssue(report.KindKV, bucket,
		fmt.Sprintf("expected %d migrated, got %d (skipped=%d)", expected, migrated, skipped))
}

func FailuresFileErrorMessage(bucket string, err error) string {
	return report.BucketIssue(report.KindKV, bucket, fmt.Sprintf("failed to write failures file: %v", err))
}
