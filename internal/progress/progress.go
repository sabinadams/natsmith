package progress

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

// ScanProgress reports progress while scanning a JetStream backing stream.
type ScanProgress struct {
	StreamMessages int
	Scanned        int
	Unique         int
	UniqueLabel    string
}

// ReportScan updates progress while scanning a KV or object store backing stream.
func (b *BucketBar) ReportScan(p ScanProgress) {
	var desc string
	if p.StreamMessages > 0 {
		desc = fmt.Sprintf("%s — %d/%d stream messages (%d %s)", b.baseDesc, p.Scanned, p.StreamMessages, p.Unique, p.UniqueLabel)
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
