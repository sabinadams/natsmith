package migration

import (
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestPrintSummaryDryRun(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("KV", Summary{DryRun: true, Buckets: 2, Migratable: 10, Omitted: 3})
	})
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "10 migratable") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintSummaryVerifyOnly(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("KV", Summary{
			VerifyOnly:   true,
			Buckets:      1,
			Migratable:   5,
			VerifyOK:     5,
			VerifyFailed: 0,
			DestOnly:     1,
		})
	})
	if !strings.Contains(out, "verification complete") || !strings.Contains(out, "dest-only") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintSummaryMigration(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("KV", Summary{
			Buckets:    2,
			Migratable: 10,
			Migrated:   10,
		})
	})
	if !strings.Contains(out, "2 buckets") || !strings.Contains(out, "10/10 objects copied") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintSummaryMigrationWithVerify(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("object store", Summary{
			Buckets:    1,
			Migratable: 4,
			Migrated:   4,
			Skipped:    1,
			Failed:     2,
			Omitted:    3,
			VerifyRan:  true,
			VerifyOK:   4,
			DestOnly:   1,
		})
	})
	for _, want := range []string{"4/4 objects copied", "1 skipped", "3 omitted", "2 failed", "verify: 4 ok", "1 dest-only"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}
