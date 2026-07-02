package migrate

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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

func TestPrintSummaryDryRun(t *testing.T) {
	out := captureStderr(t, func() {
		PrintSummary("KV", MigrationSummary{DryRun: true, Buckets: 2, Migratable: 10, Omitted: 3})
	})
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "10 migratable") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintSummaryVerifyOnly(t *testing.T) {
	out := captureStderr(t, func() {
		PrintSummary("KV", MigrationSummary{
			VerifyOnly:   true,
			Buckets:      1,
			Migratable:   5,
			VerifyOK:     5,
			VerifyFailed: 0,
			DestOnly:     1,
		})
	})
	if !strings.Contains(out, "verification complete") || !strings.Contains(out, "dest-only") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintSummaryMigrationWithVerify(t *testing.T) {
	out := captureStderr(t, func() {
		PrintSummary("object store", MigrationSummary{
			Buckets:    1,
			Migratable: 4,
			Migrated:   4,
			Skipped:    1,
			Failed:     2,
			Omitted:    3,
			VerifyRan:  true,
			VerifyOK:   4,
			DestOnly:   1,
		})
	})
	for _, want := range []string{"4/4 objects copied", "1 skipped", "3 omitted", "2 failed", "verify: 4 ok", "1 dest-only"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestBucketBarFinish(t *testing.T) {
	out := captureStderr(t, func() {
		bar := &BucketBar{baseDesc: "KV schema (1/1)"}
		bar.Finish(ItemStats{Total: 3, Migrated: 2, Skipped: 1})
	})
	if !strings.Contains(out, "2/3 copied") || !strings.Contains(out, "1 skipped") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportStreamScanNoProgress(t *testing.T) {
	out := captureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1) — scanning stream"}
		bar.ReportStreamScan(StreamScanProgress{StreamMessages: 100, Scanned: 1000, UniqueKeys: 5})
	})
	if !strings.Contains(out, "1000/100 stream messages") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportObjectScanNoProgress(t *testing.T) {
	out := captureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "Object store files (1/1) — scanning stream"}
		bar.ReportObjectScan(ObjectScanProgress{StreamMessages: 50, Scanned: 50, UniqueObjects: 2})
	})
	if !strings.Contains(out, "50/50 stream messages") {
		t.Fatalf("output: %s", out)
	}
}

func TestReportVerifyNoProgress(t *testing.T) {
	out := captureStderr(t, func() {
		bar := &BucketBar{enabled: false, baseDesc: "KV schema (1/1) — verifying"}
		bar.ReportVerify(10, 10)
	})
	if !strings.Contains(out, "10/10 keys checked") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintHeaderAndFinishMessage(t *testing.T) {
	out := captureStderr(t, func() {
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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
