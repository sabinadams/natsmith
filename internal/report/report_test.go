package report

import (
	"errors"
	"strings"
	"testing"
)

func TestBucketError(t *testing.T) {
	t.Parallel()

	err := errors.New("boom")
	got := BucketError(KindKV, "schema", 2, 5, "failed", err)
	for _, want := range []string{"✗ KV schema (2/5)", "failed", "boom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message %q missing %q", got, want)
		}
	}
}

func TestBucketInfo(t *testing.T) {
	t.Parallel()

	got := BucketInfo(KindObjectStore, "files", 1, 3, "10 migratable")
	for _, want := range []string{"· Object store files (1/3)", "10 migratable"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message %q missing %q", got, want)
		}
	}
}

func TestBucketIssue(t *testing.T) {
	t.Parallel()

	got := BucketIssue(KindKV, "schema", "expected 10 migrated, got 8 (skipped=2)")
	for _, want := range []string{"✗ KV schema", "expected 10 migrated", "skipped=2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message %q missing %q", got, want)
		}
	}
}
