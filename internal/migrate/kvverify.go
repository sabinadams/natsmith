package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
)

// KVVerifyResult reports how destination keys compare to source migratable keys.
type KVVerifyResult struct {
	Expected     int
	OK           int
	Missing      int
	Mismatch     int
	DestOnly     int
	MissingKeys  []string
	MismatchKeys []string
	DestOnlyKeys []string
}

func (r KVVerifyResult) Passed() bool {
	return r.Missing == 0 && r.Mismatch == 0
}

func (r KVVerifyResult) Issues() int {
	return r.Missing + r.Mismatch
}

// VerifyKVMigratable checks that every source migratable key exists on dest with the same value,
// then reports destination keys not in the migratable set.
func VerifyKVMigratable(
	ctx context.Context,
	destJS jetstream.JetStream,
	bucket string,
	dest jetstream.KeyValue,
	migratable []string,
	sourceValues map[string][]byte,
	workers int,
	report func(checked, total int),
) (KVVerifyResult, error) {
	result := KVVerifyResult{Expected: len(migratable)}

	migratableSet := make(map[string]struct{}, len(migratable))
	for _, key := range migratable {
		migratableSet[key] = struct{}{}
	}

	if len(migratable) == 0 {
		return result, verifyDestOnlyKeys(ctx, destJS, bucket, migratableSet, &result)
	}

	if report != nil {
		report(0, len(migratable))
	}

	var checked int
	var mu sync.Mutex
	markChecked := func() {
		checked++
		if report != nil && (checked%100 == 0 || checked == len(migratable)) {
			report(checked, len(migratable))
		}
	}

	err := RunParallel(ctx, workers, migratable, func(ctx context.Context, key string) error {
		sourceValue, ok := sourceValues[key]
		if !ok {
			return fmt.Errorf("verify key %q: no value from stream scan", key)
		}

		destEntry, err := dest.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				mu.Lock()
				result.Missing++
				result.MissingKeys = append(result.MissingKeys, key)
				markChecked()
				mu.Unlock()
				return nil
			}
			return fmt.Errorf("verify dest key %q: %w", key, err)
		}

		if !bytes.Equal(sourceValue, destEntry.Value()) {
			mu.Lock()
			result.Mismatch++
			result.MismatchKeys = append(result.MismatchKeys, key)
			markChecked()
			mu.Unlock()
			return nil
		}

		mu.Lock()
		result.OK++
		markChecked()
		mu.Unlock()
		return nil
	})
	if err != nil {
		return result, err
	}

	return result, verifyDestOnlyKeys(ctx, destJS, bucket, migratableSet, &result)
}

func verifyDestOnlyKeys(ctx context.Context, js jetstream.JetStream, bucket string, migratableSet map[string]struct{}, result *KVVerifyResult) error {
	snap, err := SnapshotKVFromStream(ctx, js, bucket, nil)
	if err != nil {
		return fmt.Errorf("scan destination stream: %w", err)
	}

	for _, key := range snap.Migratable {
		if _, ok := migratableSet[key]; ok {
			continue
		}
		result.DestOnly++
		result.DestOnlyKeys = append(result.DestOnlyKeys, key)
	}

	return nil
}

const maxSampleKeys = 20

func PrintKVVerifyReport(bucket string, verify KVVerifyResult) {
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

func WriteKVFailuresFile(path, bucket string, verify KVVerifyResult) error {
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
