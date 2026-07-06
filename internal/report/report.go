package report

import "fmt"

const (
	KindKV          = "KV"
	KindObjectStore = "Object store"
)

// BucketError formats an indexed bucket failure line.
func BucketError(kind, bucket string, index, total int, reason string, err error) string {
	return fmt.Sprintf("  ✗ %s %s (%d/%d) — %s: %v", kind, bucket, index, total, reason, err)
}

// BucketInfo formats an indexed bucket info line.
func BucketInfo(kind, bucket string, index, total int, detail string) string {
	return fmt.Sprintf("  · %s %s (%d/%d) — %s", kind, bucket, index, total, detail)
}

// BucketIssue formats a bucket-level issue without index/total.
func BucketIssue(kind, bucket, detail string) string {
	return fmt.Sprintf("  ✗ %s %s — %s", kind, bucket, detail)
}
