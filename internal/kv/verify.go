package kv

import (
	"fmt"
	"os"
)

// VerifyResult reports how destination keys compare to source migratable keys.
type VerifyResult struct {
	Expected     int
	OK           int
	Missing      int
	Mismatch     int
	DestOnly     int
	MissingKeys  []string
	MismatchKeys []string
	DestOnlyKeys []string
}

func (r VerifyResult) Passed() bool {
	return r.Missing == 0 && r.Mismatch == 0
}

func (r VerifyResult) Issues() int {
	return r.Missing + r.Mismatch
}

const maxSampleKeys = 20

// PrintVerifyReport writes a per-bucket verification summary to stderr.
func PrintVerifyReport(bucket string, verify VerifyResult) {
	if verify.Expected == 0 && verify.DestOnly == 0 {
		fmt.Fprintf(os.Stderr, "  verify %s: nothing to check\n", bucket)
		return
	}

	fmt.Fprintf(os.Stderr,
		"  verify %s: expected %d migratable on dest — %d ok, %d missing, %d mismatch",
		bucket, verify.Expected, verify.OK, verify.Missing, verify.Mismatch,
	)
	if verify.DestOnly > 0 {
		fmt.Fprintf(os.Stderr, ", %d dest-only", verify.DestOnly)
	}
	fmt.Fprintln(os.Stderr)

	printKeySample(os.Stderr, bucket, "missing", verify.MissingKeys)
	printKeySample(os.Stderr, bucket, "mismatch", verify.MismatchKeys)
	printKeySample(os.Stderr, bucket, "dest-only", verify.DestOnlyKeys)

	switch {
	case verify.Passed() && verify.DestOnly == 0:
		fmt.Fprintf(os.Stderr, "  ✓ verify %s: destination matches source migratable keys\n", bucket)
	case verify.Passed() && verify.DestOnly > 0:
		fmt.Fprintf(os.Stderr, "  ! verify %s: migratable keys match but destination has extra keys\n", bucket)
	default:
		fmt.Fprintf(os.Stderr, "  ✗ verify %s: FAILED — %d issue(s)\n", bucket, verify.Issues())
	}
}

func printKeySample(w *os.File, bucket, kind string, keys []string) {
	if len(keys) == 0 {
		return
	}
	limit := len(keys)
	if limit > maxSampleKeys {
		limit = maxSampleKeys
	}
	for _, key := range keys[:limit] {
		fmt.Fprintf(w, "    %s %s: %s\n", bucket, kind, key)
	}
	if len(keys) > maxSampleKeys {
		fmt.Fprintf(w, "    %s %s: ... and %d more\n", bucket, kind, len(keys)-maxSampleKeys)
	}
}

// WriteFailuresFile appends verification failures to path.
func WriteFailuresFile(path, bucket string, verify VerifyResult) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, key := range verify.MissingKeys {
		if _, err := fmt.Fprintf(f, "bucket=%s key=%s issue=missing\n", bucket, key); err != nil {
			return err
		}
	}
	for _, key := range verify.MismatchKeys {
		if _, err := fmt.Fprintf(f, "bucket=%s key=%s issue=mismatch\n", bucket, key); err != nil {
			return err
		}
	}
	for _, key := range verify.DestOnlyKeys {
		if _, err := fmt.Fprintf(f, "bucket=%s key=%s issue=dest-only\n", bucket, key); err != nil {
			return err
		}
	}
	return nil
}
