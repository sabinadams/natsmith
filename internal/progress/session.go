package progress

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// PlanEntry is one label/value row in the run plan block.
type PlanEntry struct {
	Label string
	Value string
}

// FailureRecord tracks a bucket failure for the footer recap.
type FailureRecord struct {
	Kind   string
	Name   string
	Reason string
	Err    error
}

// Footer summarizes a completed command run.
type Footer struct {
	Headline    string
	ExitCode    int
	Completed   int
	Total       int
	Failed      int
	Interrupted bool
}

// Session coordinates stderr output for a natsmith command run.
type Session struct {
	UI          *Progress
	title       string
	start       time.Time
	bucketStart time.Time
	failures    []FailureRecord
	completed   []string
	interrupted bool
}

// SessionConfig configures a new progress session.
type SessionConfig struct {
	Title      string
	NoProgress bool
}

// NewSession prints the command header and returns a progress session.
func NewSession(cfg SessionConfig) *Session {
	s := &Session{
		title: cfg.Title,
		start: time.Now(),
		UI:    NewProgress(!cfg.NoProgress),
	}
	installInterruptHandler(s)

	if IsJSON() {
		emitJSON("start", map[string]any{"title": cfg.Title})
		return s
	}
	if ShowHumanOutput() {
		PrintHeader(cfg.Title)
	}
	return s
}

// Elapsed returns time since the session started.
func (s *Session) Elapsed() time.Duration {
	return time.Since(s.start)
}

// Interrupted reports whether the run received SIGINT/SIGTERM.
func (s *Session) Interrupted() bool {
	return s.interrupted
}

func (s *Session) markInterrupted() {
	s.interrupted = true
}

// BeginBucket marks the start of per-bucket work for timing.
func (s *Session) BeginBucket() {
	s.bucketStart = time.Now()
}

func (s *Session) bucketElapsed() time.Duration {
	if s.bucketStart.IsZero() {
		return 0
	}
	return time.Since(s.bucketStart)
}

// PrintPlan prints the run plan after connections are established.
func (s *Session) PrintPlan(entries []PlanEntry) {
	if IsJSON() {
		rows := make([]map[string]string, len(entries))
		for i, e := range entries {
			rows[i] = map[string]string{"label": e.Label, "value": e.Value}
		}
		emitJSON("plan", map[string]any{"entries": rows})
		return
	}
	if !ShowHumanOutput() {
		return
	}

	width := headerWidth()
	fmt.Fprintln(os.Stderr)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %s  %s\n", Dim(padLabel(e.Label, 10)), e.Value)
	}
	fmt.Fprintf(os.Stderr, "%s\n\n", Dim(strings.Repeat("─", width)))
}

// Status prints a single status line (no progress bar).
func (s *Session) Status(msg string) {
	if IsJSON() {
		emitJSON("status", map[string]any{"message": msg})
		return
	}
	if !ShowHumanOutput() {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", Dim("›"), msg)
}

// BucketInfo prints an informational bucket line (·).
func (s *Session) BucketInfo(kind, name string, index, total int, detail string) {
	s.emitBucketLine("info", infoMark(), kind, name, index, total, detail, BucketStats{})
}

// BucketInfoStats prints an informational line with timing/throughput.
func (s *Session) BucketInfoStats(kind, name string, index, total int, detail string, stats BucketStats) {
	s.emitBucketLine("info", infoMark(), kind, name, index, total, detail, stats)
}

// BucketSuccess prints a completed bucket line (✓).
func (s *Session) BucketSuccess(kind, name string, index, total int, detail string) {
	s.recordCompleted(name)
	s.emitBucketLine("success", successMark(), kind, name, index, total, detail, BucketStats{})
}

// BucketSuccessStats prints success with timing/throughput.
func (s *Session) BucketSuccessStats(kind, name string, index, total int, detail string, stats BucketStats) {
	s.recordCompleted(name)
	s.emitBucketLine("success", successMark(), kind, name, index, total, detail, stats)
}

// BucketCopied prints object/KV copy completion with item stats.
func (s *Session) BucketCopied(kind, name string, index, total int, stats ItemStats) {
	s.recordCompleted(name)
	detail := fmt.Sprintf("%d/%d copied", stats.Migrated, stats.Total)
	if stats.Skipped > 0 {
		detail += fmt.Sprintf(" (%d skipped)", stats.Skipped)
	}
	if stats.Failed > 0 {
		detail += fmt.Sprintf(" (%d failed)", stats.Failed)
	}
	itemStats := BucketStats{Items: int64(stats.Migrated)}
	s.emitBucketLine("success", successMark(), kind, name, index, total, detail, itemStats)
}

// BucketWarning prints a warning bucket line (!).
func (s *Session) BucketWarning(kind, name, detail string) {
	line := fmt.Sprintf("  %s %s %s — %s", warnMark(), labelKV(kind), name, detail)
	s.emitLine("warning", line, map[string]any{"kind": kind, "name": name, "detail": detail})
}

// BucketFail prints a failed bucket line (✗) and records it for the footer.
func (s *Session) BucketFail(kind, name string, index, total int, reason string, err error) {
	s.failures = append(s.failures, FailureRecord{Kind: kind, Name: name, Reason: reason, Err: err})
	line := formatIndexedLine(failMark(), kind, name, index, total, fmt.Sprintf("%s: %v", reason, err))
	s.emitLine("failure", line, map[string]any{
		"kind": kind, "name": name, "index": index, "total": total,
		"reason": reason, "error": err.Error(),
	})
}

// BucketIssue prints a bucket issue without index (used for post-run mismatches).
func (s *Session) BucketIssue(kind, name, detail string) {
	line := fmt.Sprintf("  %s %s %s — %s", failMark(), labelKV(kind), name, detail)
	s.emitLine("issue", line, map[string]any{"kind": kind, "name": name, "detail": detail})
}

func (s *Session) emitBucketLine(eventType, mark, kind, name string, index, total int, detail string, stats BucketStats) {
	detail += formatBucketSuffix(s.bucketElapsed(), stats)
	line := formatIndexedLine(mark, kind, name, index, total, detail)
	s.emitLine(eventType, line, map[string]any{
		"kind": kind, "name": name, "index": index, "total": total, "detail": detail,
	})
}

func (s *Session) emitLine(eventType, line string, fields map[string]any) {
	if IsJSON() {
		emitJSON(eventType, fields)
		return
	}
	if eventType == "failure" || eventType == "issue" || eventType == "warning" {
		fmt.Fprintln(os.Stderr, line)
		return
	}
	if ShowHumanOutput() {
		fmt.Fprintln(os.Stderr, line)
	}
}

func (s *Session) recordCompleted(name string) {
	s.completed = append(s.completed, name)
}

// Completef prints a formatted final summary with elapsed time and failure recap.
func (s *Session) Completef(exitCode int, format string, args ...any) {
	s.Complete(Footer{
		Headline:    fmt.Sprintf(format, args...),
		ExitCode:    exitCode,
		Completed:   len(s.completed),
		Failed:      len(s.failures),
		Interrupted: s.interrupted,
	})
}

// Complete prints the final footer. Exit code in Footer overrides derived failures.
func (s *Session) Complete(footer Footer) {
	exitCode := footer.ExitCode
	if exitCode == 0 && len(s.failures) > 0 {
		exitCode = 1
	}

	headline := footer.Headline
	if footer.Interrupted && !strings.HasPrefix(headline, "Interrupted") {
		headline = fmt.Sprintf("Interrupted — %s", headline)
	}

	elapsed := s.Elapsed()
	if IsJSON() {
		emitJSON("complete", map[string]any{
			"headline": headline, "elapsed": FormatElapsed(elapsed),
			"exit_code": exitCode, "completed": footer.Completed,
			"failed": len(s.failures), "interrupted": footer.Interrupted,
		})
		for _, f := range s.failures {
			emitJSON("failure_recap", map[string]any{
				"kind": f.Kind, "name": f.Name, "reason": f.Reason, "error": f.Err.Error(),
			})
		}
		return
	}

	if ShowHumanOutput() || len(s.failures) > 0 {
		fmt.Fprintln(os.Stderr)

		msg := headline
		switch {
		case footer.Interrupted:
			msg = fmt.Sprintf("%s — %s", Yellow("Interrupted"), headline)
		case exitCode != 0 || len(s.failures) > 0:
			msg = fmt.Sprintf("%s — %s", Yellow("Finished with errors"), headline)
			if len(s.failures) > 0 {
				msg += fmt.Sprintf(" — %d failed", len(s.failures))
			}
		case UseColor():
			msg = fmt.Sprintf("%s — %s", Green("Done"), headline)
		default:
			msg = fmt.Sprintf("Done — %s", headline)
		}
		msg += " — completed in " + FormatElapsed(elapsed)
		fmt.Fprintln(os.Stderr, msg)

		if len(s.failures) > 0 && ShowHumanOutput() {
			fmt.Fprintln(os.Stderr, Dim("  failed buckets:"))
			for _, f := range s.failures {
				fmt.Fprintf(os.Stderr, "    %s %s %s — %s: %v\n",
					failMark(), labelKV(f.Kind), f.Name, f.Reason, f.Err)
			}
		}
	}
}

func padLabel(label string, width int) string {
	if len(label) >= width {
		return label
	}
	return label + strings.Repeat(" ", width-len(label))
}

func headerWidth() int {
	return 52
}
