package progress

import (
	"fmt"
	"strings"
)

// FormatBucketCount describes how many buckets will be processed.
func FormatBucketCount(count int, filter string) string {
	if filter != "" {
		return fmt.Sprintf("%d matching filter (%s)", count, filter)
	}
	if count == 1 {
		return "1 bucket"
	}
	return fmt.Sprintf("%d buckets", count)
}

// JoinFlags returns a comma-separated flag list, or "none".
func JoinFlags(flags ...string) string {
	var active []string
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f != "" {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return Dim("none")
	}
	return strings.Join(active, ", ")
}
