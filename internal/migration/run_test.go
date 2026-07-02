package migration

import (
	"errors"
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/testutil"
)

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
		_ = CompleteRun("KV", Summary{DryRun: true, Buckets: 1, Migratable: 3, Omitted: 1}, 0)
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
