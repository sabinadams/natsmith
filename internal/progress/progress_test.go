package progress

import (
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestTruncateMiddle(t *testing.T) {
	t.Parallel()

	if got := truncateMiddle("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	if got := truncateMiddle("0123456789abcdef", 10); got != "012...def" {
		t.Fatalf("got %q", got)
	}
	if got := truncateMiddle("abc", 3); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestBucketBarFinish(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{baseDesc: "KV schema (1/1)"}
		bar.Finish(ItemStats{Total: 3, Migrated: 2, Skipped: 1})
	})
	if !strings.Contains(out, "2/3 copied") || !strings.Contains(out, "1 skipped") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportScanKVNoProgress(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1) — scanning stream"}
		bar.ReportScan(ScanProgress{StreamMessages: 100, Scanned: 1000, Unique: 5, UniqueLabel: "keys"})
	})
	if !strings.Contains(out, "1000/100 stream messages") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportScanObjectsNoProgress(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "Object store files (1/1) — scanning stream"}
		bar.ReportScan(ScanProgress{StreamMessages: 50, Scanned: 50, Unique: 2, UniqueLabel: "objects"})
	})
	if !strings.Contains(out, "50/50 stream messages") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportVerifyNoProgress(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1) — verifying"}
		bar.ReportVerify(10, 10)
	})
	if !strings.Contains(out, "10/10 keys checked") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintHeaderAndFinishMessage(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintHeader("KV migration")
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1)"}
		bar.FinishMessage("  · done")
	})
	if !strings.Contains(out, "KV migration") || !strings.Contains(out, "done") {
		t.Fatalf("output: %s", out)
	}
}

func TestNewProgressDisabled(t *testing.T) {
	p := NewProgress(false)
	bar := p.StartBucket("KV", "schema", 1, 1, 0, 1)
	bar.SetItem("key")
	bar.Add(1)
	bar.ClearItem()
}
