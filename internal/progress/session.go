package progress

import (
	"fmt"
	"os"

	"github.com/sabinadams/natsmith/internal/report"
)

// Session coordinates stderr output for a natsmith command run.
//
// Standard flow:
//  1. NewSession + title header
//  2. Status (e.g. "Connecting...")
//  3. Per bucket: one BucketBar or TransferTracker at a time
//  4. BucketSuccess / BucketFail per bucket
//  5. Complete footer
type Session struct {
	UI *Progress
}

// NewSession prints the command title and returns a progress session.
func NewSession(enabled bool, title string) *Session {
	PrintHeader(title)
	return &Session{UI: NewProgress(enabled)}
}

// Status prints a single status line (no progress bar).
func (s *Session) Status(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// BucketSuccess prints a completed bucket line.
func (s *Session) BucketSuccess(kind, name string, index, total int, detail string) {
	fmt.Fprintf(os.Stderr, "  ✓ %s %s (%d/%d) — %s\n", kind, name, index, total, detail)
}

// BucketFail prints a failed bucket line.
func (s *Session) BucketFail(kind, name string, index, total int, reason string, err error) {
	fmt.Fprintln(os.Stderr, report.BucketError(kind, name, index, total, reason, err))
}

// Completef prints a formatted final command summary.
func (s *Session) Completef(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n"+format+"\n", args...)
}
