package kv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestVerifyResultHelpers(t *testing.T) {
	t.Parallel()

	ok := VerifyResult{Missing: 0, Mismatch: 0}
	if !ok.Passed() || ok.Issues() != 0 {
		t.Fatal("expected pass")
	}
	bad := VerifyResult{Missing: 1, Mismatch: 2, DestOnly: 3}
	if bad.Passed() || bad.Issues() != 3 {
		t.Fatal("expected issues")
	}
}

func TestWriteFailuresFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failures.log")

	verify := VerifyResult{
		MissingKeys:  []string{"m1"},
		MismatchKeys: []string{"x1"},
		DestOnlyKeys: []string{"d1"},
	}
	if err := WriteFailuresFile(path, "schema", verify); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"bucket=schema key=m1 issue=missing",
		"bucket=schema key=x1 issue=mismatch",
		"bucket=schema key=d1 issue=dest-only",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}
}

func TestReportVerify(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		session := progress.NewSession(progress.SessionConfig{Title: "verify", NoProgress: true})
		ReportVerify(session, "schema", VerifyResult{Expected: 2, OK: 2})
	})
	if !strings.Contains(out, "destination matches source migratable keys") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintVerifyReport(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintVerifyReport("schema", VerifyResult{
			Expected:     2,
			OK:           1,
			Missing:      1,
			MissingKeys:  []string{"missing-key"},
			DestOnly:     1,
			DestOnlyKeys: []string{"extra-key"},
		})
	})
	for _, want := range []string{"1 ok", "1 missing", "1 dest-only", "missing-key", "extra-key", "FAILED"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestPrintVerifyReportNothingToCheck(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintVerifyReport("empty", VerifyResult{})
	})
	if !strings.Contains(out, "nothing to check") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintVerifyReportAllPassed(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintVerifyReport("schema", VerifyResult{Expected: 2, OK: 2})
	})
	if !strings.Contains(out, "destination matches source migratable keys") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintVerifyReportDestOnlyPassed(t *testing.T) {
	out := testutil.CaptureStderr(t, func() {
		PrintVerifyReport("schema", VerifyResult{
			Expected:     1,
			OK:           1,
			DestOnly:     1,
			DestOnlyKeys: []string{"extra"},
		})
	})
	if !strings.Contains(out, "extra keys") || !strings.Contains(out, "extra") {
		t.Fatalf("output: %s", out)
	}
}

func TestPrintKeySampleTruncation(t *testing.T) {
	keys := make([]string, maxSampleKeys+3)
	for i := range keys {
		keys[i] = "key"
	}
	out := testutil.CaptureStderr(t, func() {
		printKeySample(os.Stderr, "schema", "missing", keys)
	})
	if !strings.Contains(out, "... and 3 more") {
		t.Fatalf("output: %s", out)
	}
}
