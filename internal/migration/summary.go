package migration

import (
	"fmt"
	"os"
)

// ExitError signals a non-zero process exit code from a command handler to cmd.Execute.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Summary totals across all buckets in one migration run.
type Summary struct {
	Buckets      int
	Migratable   int
	Migrated     int
	Skipped      int
	Omitted      int
	Failed       int
	VerifyOK     int
	VerifyFailed int
	DestOnly     int
	VerifyRan    bool
	DryRun       bool
	VerifyOnly   bool
}

// PrintSummary writes a final migration report to stderr.
func PrintSummary(kind string, summary Summary) {
	if summary.VerifyOnly {
		fmt.Fprintf(os.Stderr,
			"\nAll %s verification complete: %d buckets, %d migratable checked, %d ok, %d failed, %d dest-only\n",
			kind, summary.Buckets, summary.Migratable, summary.VerifyOK, summary.VerifyFailed, summary.DestOnly,
		)
		return
	}

	if summary.DryRun {
		fmt.Fprintf(os.Stderr,
			"\nAll %s dry run complete: %d buckets, %d migratable, %d omitted\n",
			kind, summary.Buckets, summary.Migratable, summary.Omitted,
		)
		return
	}

	msg := fmt.Sprintf("\nAll %s migration complete: %d buckets, %d/%d objects copied", kind, summary.Buckets, summary.Migrated, summary.Migratable)
	if summary.Skipped > 0 {
		msg += fmt.Sprintf(", %d skipped", summary.Skipped)
	}
	if summary.Omitted > 0 {
		msg += fmt.Sprintf(", %d omitted", summary.Omitted)
	}
	if summary.Failed > 0 {
		msg += fmt.Sprintf(", %d failed", summary.Failed)
	}
	if summary.VerifyRan {
		msg += fmt.Sprintf(" — verify: %d ok", summary.VerifyOK)
		if summary.VerifyFailed > 0 {
			msg += fmt.Sprintf(", %d failed", summary.VerifyFailed)
		}
		if summary.DestOnly > 0 {
			msg += fmt.Sprintf(", %d dest-only", summary.DestOnly)
		}
	}
	fmt.Fprintln(os.Stderr, msg)
}

// CompleteRun prints the final summary and returns ExitError when exitCode is non-zero.
func CompleteRun(kind string, summary Summary, exitCode int) error {
	PrintSummary(kind, summary)
	if exitCode != 0 {
		return &ExitError{Code: exitCode}
	}
	return nil
}
