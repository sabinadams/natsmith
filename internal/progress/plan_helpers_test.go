package progress

import "testing"

func TestFormatBucketCount(t *testing.T) {
	t.Parallel()

	if got := FormatBucketCount(1, ""); got != "1 bucket" {
		t.Fatalf("got %q", got)
	}
	if got := FormatBucketCount(3, "schema,cache"); got != "3 matching filter (schema,cache)" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinFlags(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if got := JoinFlags(); got != "none" {
		t.Fatalf("got %q", got)
	}
	if got := JoinFlags("--force", "", "--dry-run"); got != "--force, --dry-run" {
		t.Fatalf("got %q", got)
	}
}
