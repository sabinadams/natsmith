package migration

import (
	"fmt"
	"os"

	"github.com/sabinadams/natsmith/internal/progress"
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

// SummaryMessage builds the migration footer headline (without status prefix or elapsed).
func SummaryMessage(kind string, summary Summary) string {
	if summary.VerifyOnly {
		return fmt.Sprintf(
			"All %s verification complete: %d buckets, %d migratable checked, %d ok, %d failed, %d dest-only",
			kind, summary.Buckets, summary.Migratable, summary.VerifyOK, summary.VerifyFailed, summary.DestOnly,
		)
	}

	if summary.DryRun {
		msg := fmt.Sprintf("All %s dry run complete: %d buckets, %d migratable", kind, summary.Buckets, summary.Migratable)
		if summary.Omitted > 0 {
			msg += fmt.Sprintf(", %d omitted", summary.Omitted)
		}
		return msg
	}

	msg := fmt.Sprintf("All %s migration complete: %d buckets, %d/%d objects copied", kind, summary.Buckets, summary.Migrated, summary.Migratable)
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
	return msg
}

// PrintSummary writes a final migration report to stderr (tests and legacy callers).
func PrintSummary(kind string, summary Summary) {
	fmt.Fprintln(os.Stderr, "\n"+SummaryMessage(kind, summary))
}

// CompleteRun prints the final summary and returns ExitError when exitCode is non-zero.
func CompleteRun(kind string, summary Summary, exitCode int, session *progress.Session) error {
	session.Complete(progress.Footer{
		Headline:    SummaryMessage(kind, summary),
		ExitCode:    exitCode,
		Interrupted: session.Interrupted(),
	})
	if exitCode != 0 {
		return &ExitError{Code: exitCode}
	}
	return nil
}
