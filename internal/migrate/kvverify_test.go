package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

func TestKVVerifyResultHelpers(t *testing.T) {
	t.Parallel()

	ok := KVVerifyResult{Missing: 0, Mismatch: 0}
	if !ok.Passed() || ok.Issues() != 0 {
		t.Fatal("expected pass")
	}
	bad := KVVerifyResult{Missing: 1, Mismatch: 2, DestOnly: 3}
	if bad.Passed() || bad.Issues() != 3 {
		t.Fatal("expected issues")
	}
}

func TestWriteKVFailuresFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failures.log")

	verify := KVVerifyResult{
		MissingKeys:  []string{"m1"},
		MismatchKeys: []string{"x1"},
		DestOnlyKeys: []string{"d1"},
	}
	if err := WriteKVFailuresFile(path, "schema", verify); err != nil {
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

func TestPrintKVVerifyReport(t *testing.T) {
	out := captureStderr(t, func() {
		PrintKVVerifyReport("schema", KVVerifyResult{
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

func TestVerifyKVMigratable(t *testing.T) {
	srv := startNATSServer(t)
	nc := connectNATS(t, srv.ClientURL())
	defer nc.Close()

	js := newJetStream(t, nc)
	ctx := testContext(t)

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

	result, err := VerifyKVMigratable(
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

func TestVerifyKVMigratableDestOnly(t *testing.T) {
	srv := startNATSServer(t)
	nc := connectNATS(t, srv.ClientURL())
	defer nc.Close()

	js := newJetStream(t, nc)
	ctx := testContext(t)

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

	result, err := VerifyKVMigratable(ctx, js, "DEST", dest, []string{"shared"}, map[string][]byte{"shared": []byte("v")}, 1, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed() || result.DestOnly != 1 || result.DestOnlyKeys[0] != "extra" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
