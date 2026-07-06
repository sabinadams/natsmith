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
	if IsQuiet() || IsJSON() {
		enabled = false
	}
	if enabled && !term.IsTerminal(int(os.Stderr.Fd())) {
		enabled = false
	}
	return &Progress{enabled: enabled}
}

// BucketBar tracks progress for a single KV bucket or object store.
type BucketBar struct {
	enabled            bool
	closed             bool
	bar                *progressbar.ProgressBar
	bar64              *progressbar.ProgressBar
	baseDesc           string
	mu                 sync.Mutex
	showItems          bool
	transferTotal      int64
	lastTransferSent   int64
	lastTransferReport time.Time
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
		progressbar.OptionEnableColorCodes(UseColor()),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("items"),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetTheme(barTheme()),
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
		progressbar.OptionEnableColorCodes(UseColor()),
		progressbar.OptionFullWidth(),
	)

	return b
}

// StartTransfer shows byte transfer progress for backup/restore operations.
func (p *Progress) StartTransfer(kind, name string, index, total int, verb string, totalBytes int64) *BucketBar {
	desc := fmt.Sprintf("%s %s (%d/%d) — %s", kind, name, index, total, verb)
	b := &BucketBar{enabled: p.enabled, baseDesc: desc, transferTotal: totalBytes}
	if !p.enabled {
		return b
	}

	if totalBytes <= 0 {
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

	b.bar64 = progressbar.NewOptions64(
		totalBytes,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(UseColor()),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetTheme(barTheme()),
	)

	return b
}

// ReportTransfer updates byte transfer progress. Safe to call frequently; updates are throttled.
func (b *BucketBar) ReportTransfer(sent int64) {
	if sent < 0 {
		return
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	if b.enabled && b.bar64 != nil {
		b.mu.Lock()
		_ = b.bar64.Set64(sent)
		b.mu.Unlock()
		return
	}

	if b.enabled && b.bar != nil {
		return
	}

	if !shouldReportTransfer(b, sent) {
		return
	}

	total := b.transferTotal
	if total > 0 {
		fmt.Fprintf(os.Stderr, "  %s — %s / %s\n", b.baseDesc, humanizeBytes(sent), humanizeBytes(total))
	} else {
		fmt.Fprintf(os.Stderr, "  %s — %s\n", b.baseDesc, humanizeBytes(sent))
	}
}

func shouldReportTransfer(b *BucketBar, sent int64) bool {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.lastTransferReport.IsZero() {
		b.lastTransferSent = sent
		b.lastTransferReport = now
		return true
	}

	elapsed := now.Sub(b.lastTransferReport)
	pctDelta := false
	if b.transferTotal > 0 {
		lastPct := float64(b.lastTransferSent) / float64(b.transferTotal)
		curPct := float64(sent) / float64(b.transferTotal)
		pctDelta = curPct-lastPct >= 0.05
	}

	if elapsed >= 2*time.Second || pctDelta || (b.transferTotal > 0 && sent >= b.transferTotal) {
		b.lastTransferSent = sent
		b.lastTransferReport = now
		return true
	}
	return false
}

// FinishTransfer closes an active transfer progress bar.
func (b *BucketBar) FinishTransfer() {
	b.Close()
}

// TransferTracker owns a transfer bar and upgrades to a sized bar when total bytes are known.
type TransferTracker struct {
	ui        *Progress
	bar       *BucketBar
	kind      string
	name      string
	index     int
	buckets   int
	verb      string
	totalSize int64
}

// StartTransferTracked begins transfer progress. Pass totalBytes when known (e.g. restore); use 0 for backup until the first callback.
func (p *Progress) StartTransferTracked(kind, name string, index, buckets int, verb string, totalBytes int64) *TransferTracker {
	return &TransferTracker{
		ui:        p,
		bar:       p.StartTransfer(kind, name, index, buckets, verb, totalBytes),
		kind:      kind,
		name:      name,
		index:     index,
		buckets:   buckets,
		verb:      verb,
		totalSize: totalBytes,
	}
}

// Report updates transfer progress.
func (t *TransferTracker) Report(sent, total int64) {
	if total > 0 && t.totalSize != total {
		t.bar.FinishTransfer()
		t.totalSize = total
		t.bar = t.ui.StartTransfer(t.kind, t.name, t.index, t.buckets, t.verb, total)
	}
	t.bar.ReportTransfer(sent)
}

// Finish closes the transfer bar.
func (t *TransferTracker) Finish() {
	t.bar.FinishTransfer()
}

func humanizeBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
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
	switch {
	case p.StreamMessages > 0:
		desc = fmt.Sprintf("%s — %d/%d stream messages (%d %s)", b.baseDesc, p.Scanned, p.StreamMessages, p.Unique, p.UniqueLabel)
	case p.Scanned > 0:
		desc = fmt.Sprintf("%s — %d %s", b.baseDesc, p.Scanned, p.UniqueLabel)
	default:
		desc = b.baseDesc
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

func (b *BucketBar) Finish(_ ItemStats) {
	b.Close()
}

// Close shuts down any active progress bar without printing a summary line.
func (b *BucketBar) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	if b.bar64 != nil {
		_, _ = io.WriteString(os.Stderr, "\n")
		_ = b.bar64.Close()
		b.bar64 = nil
	}
	if b.bar != nil {
		_, _ = io.WriteString(os.Stderr, "\n")
		_ = b.bar.Close()
		b.bar = nil
	}
}

func barTheme() progressbar.Theme {
	return progressbar.Theme{
		Saucer:        "[green]=[reset]",
		SaucerHead:    "[green]>[reset]",
		SaucerPadding: " ",
		BarStart:      "[cyan][[reset]",
		BarEnd:        "[cyan]][reset]",
	}
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
