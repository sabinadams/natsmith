package migrate

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// Progress renders per-bucket progress bars to stderr.
type Progress struct {
	enabled bool
}

// ItemStats summarizes work done within one bucket.
type ItemStats struct {
	Total    int
	Migrated int
	Skipped  int
	Omitted  int // not migratable on source (deleted meta / tombstone)
	Failed   int // migratable but could not be copied
}

func NewProgress(enabled bool) *Progress {
	if enabled && !term.IsTerminal(int(os.Stderr.Fd())) {
		enabled = false
	}
	return &Progress{enabled: enabled}
}

// BucketBar tracks progress for a single KV bucket or object store.
type BucketBar struct {
	enabled   bool
	bar       *progressbar.ProgressBar
	baseDesc  string
	mu        sync.Mutex
	showItems bool
}

func (p *Progress) StartBucket(kind, name string, index, total, items, workers int) *BucketBar {
	prefix := fmt.Sprintf("%s %s (%d/%d)", kind, name, index, total)
	b := &BucketBar{enabled: p.enabled, baseDesc: prefix, showItems: workers <= 1}
	if !p.enabled {
		return b
	}

	if items == 0 {
		fmt.Fprintf(os.Stderr, "%s — empty\n", prefix)
		return b
	}

	b.bar = progressbar.NewOptions(
		items,
		progressbar.OptionSetDescription(prefix),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("items"),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[cyan][[reset]",
			BarEnd:        "[cyan]][reset]",
		}),
	)

	return b
}

// StartIndeterminate shows activity when the total item count is not yet known.
func (p *Progress) StartIndeterminate(kind, name string, index, total int, message string) *BucketBar {
	desc := fmt.Sprintf("%s %s (%d/%d) — %s", kind, name, index, total, message)
	b := &BucketBar{enabled: p.enabled, baseDesc: desc}
	if !p.enabled {
		fmt.Fprintln(os.Stderr, " ", desc)
		return b
	}

	b.bar = progressbar.NewOptions(
		-1,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionFullWidth(),
	)

	return b
}

func (b *BucketBar) SetItem(name string) {
	if !b.enabled || b.bar == nil || !b.showItems {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bar.Describe(fmt.Sprintf("%s | %s", b.baseDesc, truncateMiddle(name, 48)))
}

func (b *BucketBar) Add(n int) {
	if !b.enabled || b.bar == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = b.bar.Add(n)
}

func (b *BucketBar) ClearItem() {
	if !b.enabled || b.bar == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bar.Describe(b.baseDesc)
}

// ReportStreamScan updates progress while scanning a KV backing stream.
func (b *BucketBar) ReportStreamScan(p StreamScanProgress) {
	desc := b.baseDesc
	if p.StreamMessages > 0 {
		desc = fmt.Sprintf("%s — %d/%d stream messages (%d keys)", b.baseDesc, p.Scanned, p.StreamMessages, p.UniqueKeys)
	} else {
		desc = fmt.Sprintf("%s — starting stream scan", b.baseDesc)
	}

	if b.enabled && b.bar != nil {
		b.mu.Lock()
		b.bar.Describe(desc)
		b.mu.Unlock()
		return
	}

	if p.Scanned == 0 || p.Scanned%1000 == 0 || (p.StreamMessages > 0 && p.Scanned >= p.StreamMessages) {
		fmt.Fprintf(os.Stderr, "  %s\n", desc)
	}
}

// ReportObjectScan updates progress while scanning an object store meta stream.
func (b *BucketBar) ReportObjectScan(p ObjectScanProgress) {
	desc := b.baseDesc
	if p.StreamMessages > 0 {
		desc = fmt.Sprintf("%s — %d/%d stream messages (%d objects)", b.baseDesc, p.Scanned, p.StreamMessages, p.UniqueObjects)
	} else {
		desc = fmt.Sprintf("%s — starting stream scan", b.baseDesc)
	}

	if b.enabled && b.bar != nil {
		b.mu.Lock()
		b.bar.Describe(desc)
		b.mu.Unlock()
		return
	}

	if p.Scanned == 0 || p.Scanned%1000 == 0 || (p.StreamMessages > 0 && p.Scanned >= p.StreamMessages) {
		fmt.Fprintf(os.Stderr, "  %s\n", desc)
	}
}

// ReportVerify updates progress while checking keys on the destination.
func (b *BucketBar) ReportVerify(checked, total int) {
	desc := fmt.Sprintf("%s — %d/%d keys checked", b.baseDesc, checked, total)
	if b.enabled && b.bar != nil {
		b.mu.Lock()
		b.bar.Describe(desc)
		b.mu.Unlock()
		return
	}
	if checked == 0 || checked%100 == 0 || checked >= total {
		fmt.Fprintf(os.Stderr, "  %s\n", desc)
	}
}

func (b *BucketBar) Finish(stats ItemStats) {
	if b.bar != nil {
		_, _ = io.WriteString(os.Stderr, "\n")
		_ = b.bar.Close()
	}

	if b.baseDesc == "" {
		return
	}

	summary := fmt.Sprintf("  ✓ %s — %d/%d copied", b.baseDesc, stats.Migrated, stats.Total)
	if stats.Skipped > 0 {
		summary += fmt.Sprintf(" (%d skipped)", stats.Skipped)
	}
	if stats.Omitted > 0 {
		summary += fmt.Sprintf(" (%d omitted — not on source)", stats.Omitted)
	}
	if stats.Failed > 0 {
		summary += fmt.Sprintf(" (%d failed)", stats.Failed)
	}
	fmt.Fprintln(os.Stderr, summary)
}

func (b *BucketBar) FinishMessage(message string) {
	if b.bar != nil {
		_, _ = io.WriteString(os.Stderr, "\n")
		_ = b.bar.Close()
	}
	if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}
}

func PrintHeader(title string) {
	fmt.Fprintf(os.Stderr, "\n%s\n\n", title)
}

// MigrationSummary totals across all buckets in one run.
type MigrationSummary struct {
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

func PrintSummary(kind string, summary MigrationSummary) {
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

func truncateMiddle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	keep := (max - 3) / 2
	return s[:keep] + "..." + s[len(s)-keep:]
}
