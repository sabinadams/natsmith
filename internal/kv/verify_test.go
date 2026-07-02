package kv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
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

func TestVerifyMigratable(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	destKV, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "VERIFY"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := destKV.Put(ctx, "ok", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if _, err := destKV.Put(ctx, "bad", []byte("dest")); err != nil {
		t.Fatal(err)
	}

	result, err := VerifyMigratable(
		ctx,
		js,
		"VERIFY",
		destKV,
		[]string{"ok", "bad", "missing"},
		map[string][]byte{
			"ok":      []byte("v1"),
			"bad":     []byte("source"),
			"missing": []byte("x"),
		},
		2,
		nil,
	)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.OK != 1 || result.Mismatch != 1 || result.Missing != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Passed() {
		t.Fatal("expected failure due to missing/mismatch")
	}
}

func TestVerifyMigratableDestOnly(t *testing.T) {
	srv := testutil.StartServer(t)
	nc := testutil.Connect(t, srv.ClientURL())
	js := testutil.JetStream(t, nc)
	ctx := testutil.Context(t)

	src, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SRC"})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "DEST"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Put(ctx, "shared", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Put(ctx, "shared", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Put(ctx, "extra", []byte("x")); err != nil {
		t.Fatal(err)
	}

	result, err := VerifyMigratable(ctx, js, "DEST", dest, []string{"shared"}, map[string][]byte{"shared": []byte("v")}, 1, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed() || result.DestOnly != 1 || result.DestOnlyKeys[0] != "extra" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
