package migration

import (
	"errors"
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestPrintSummaryDryRun(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("KV", Summary{DryRun: true, Buckets: 2, Migratable: 10})
	})
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "10 migratable") {
		t.Fatalf("output: %s", out)
	}
	if strings.Contains(out, "omitted") {
		t.Fatalf("KV dry run should not mention omitted: %s", out)
	}
}

func TestPrintSummaryDryRunWithOmitted(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("object store", Summary{DryRun: true, Buckets: 1, Migratable: 5, Omitted: 2})
	})
	if !strings.Contains(out, "5 migratable") || !strings.Contains(out, "2 omitted") {
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

func TestCompleteRunSuccess(t *testing.T) {
	err := CompleteRun("KV", Summary{DryRun: true, Buckets: 1, Migratable: 2}, 0)
	if err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
}

func TestCompleteRunFailure(t *testing.T) {
	err := CompleteRun("KV", Summary{}, 2)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("code = %d, want 2", exitErr.Code)
	}
}

func TestCompleteRunPrintsSummary(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		_ = CompleteRun("KV", Summary{DryRun: true, Buckets: 1, Migratable: 3}, 0)
	})
	if !strings.Contains(out, "dry run") {
		t.Fatalf("output: %s", out)
	}
}

func TestExitErrorError(t *testing.T) {
	t.Parallel()

	err := (&ExitError{Code: 1}).Error()
	if err != "exit status 1" {
		t.Fatalf("Error() = %q", err)
	}
}
