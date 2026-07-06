package migration

import (
	"errors"
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestSummaryMessageDryRun(t *testing.T) {
	msg := SummaryMessage("KV", Summary{DryRun: true, Buckets: 2, Migratable: 10})
	if !strings.Contains(msg, "dry run") || !strings.Contains(msg, "10 migratable") {
		t.Fatalf("message: %s", msg)
	}
	if strings.Contains(msg, "omitted") {
		t.Fatal("KV dry run should not mention omitted")
	}
}

func TestSummaryMessageDryRunWithOmitted(t *testing.T) {
	msg := SummaryMessage("object store", Summary{DryRun: true, Buckets: 1, Migratable: 5, Omitted: 2})
	if !strings.Contains(msg, "5 migratable") || !strings.Contains(msg, "2 omitted") {
		t.Fatalf("message: %s", msg)
	}
}

func TestSummaryMessageVerifyOnly(t *testing.T) {
	msg := SummaryMessage("KV", Summary{
		VerifyOnly:   true,
		Buckets:      1,
		Migratable:   5,
		VerifyOK:     5,
		VerifyFailed: 0,
		DestOnly:     1,
	})
	if !strings.Contains(msg, "verification complete") || !strings.Contains(msg, "dest-only") {
		t.Fatalf("message: %s", msg)
	}
}

func TestSummaryMessageMigration(t *testing.T) {
	msg := SummaryMessage("KV", Summary{
		Buckets:    2,
		Migratable: 10,
		Migrated:   10,
	})
	if !strings.Contains(msg, "2 buckets") || !strings.Contains(msg, "10/10 objects copied") {
		t.Fatalf("message: %s", msg)
	}
}

func TestSummaryMessageMigrationWithVerify(t *testing.T) {
	msg := SummaryMessage("object store", Summary{
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
	for _, want := range []string{"4/4 objects copied", "1 skipped", "3 omitted", "2 failed", "verify: 4 ok", "1 dest-only"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %s", want, msg)
		}
	}
}

func TestPrintSummaryDryRun(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintSummary("KV", Summary{DryRun: true, Buckets: 2, Migratable: 10})
	})
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "10 migratable") {
		t.Fatalf("output: %s", out)
	}
}

func TestCompleteRunSuccess(t *testing.T) {
	session := progress.NewSession(progress.SessionConfig{Title: "KV migration", NoProgress: true})
	err := CompleteRun("KV", Summary{DryRun: true, Buckets: 1, Migratable: 2}, 0, session)
	if err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
}

func TestCompleteRunFailure(t *testing.T) {
	session := progress.NewSession(progress.SessionConfig{Title: "KV migration", NoProgress: true})
	err := CompleteRun("KV", Summary{}, 2, session)
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
		session := progress.NewSession(progress.SessionConfig{Title: "KV migration", NoProgress: true})
		_ = CompleteRun("KV", Summary{DryRun: true, Buckets: 1, Migratable: 3}, 0, session)
	})
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "completed in") {
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
