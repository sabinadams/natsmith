package progress

import (
	"fmt"
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

func TestReportScanListKeysNoProgress(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1) — listing keys"}
		bar.ReportScan(ScanProgress{Scanned: 32000, Unique: 32000, UniqueLabel: "keys"})
	})
	if !strings.Contains(out, "32000 keys") {
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

func TestReportTransferThrottled(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		bar := &BucketBar{
			enabled:       false,
			baseDesc:      "KV telnyx (1/1) — restoring",
			transferTotal: 1000,
		}
		bar.ReportTransfer(10)
		bar.ReportTransfer(20)
		bar.ReportTransfer(30)
	})
	if strings.Count(out, "restoring") != 1 {
		t.Fatalf("expected one throttled line, got: %q", out)
	}
}

func TestTransferTrackerUpgradesBar(t *testing.T) {
	_ = testutil.CaptureStderr(t, func() {
		p := NewProgress(false)
		tracker := p.StartTransferTracked("KV", "schema", 1, 1, "backing up", 0)
		tracker.Report(500, 2000)
		tracker.Report(1000, 2000)
		tracker.Finish()
	})
}

func TestSessionOutput(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		s := NewSession(false, "KV restore")
		s.Status("Connecting...")
		s.BucketSuccess("KV", "schema", 1, 2, "done")
		s.BucketFail("KV", "bad", 2, 2, "restore failed", fmt.Errorf("boom"))
		s.Completef("KV restore complete: %d/%d buckets", 1, 2)
	})
	for _, want := range []string{
		"KV restore",
		"Connecting...",
		"✓ KV schema (1/2)",
		"✗ KV bad (2/2)",
		"KV restore complete: 1/2 buckets",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}
