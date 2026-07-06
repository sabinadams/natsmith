package workpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRunParallelSequential(t *testing.T) {
	t.Parallel()

	var seen []int
	err := RunParallel(context.Background(), 1, []int{1, 2, 3}, func(ctx context.Context, n int) error {
		seen = append(seen, n)
		return nil
	})
	if err != nil || len(seen) != 3 {
		t.Fatalf("sequential run: err=%v seen=%v", err, seen)
	}
}

func TestRunParallelEmpty(t *testing.T) {
	t.Parallel()

	if err := RunParallel(context.Background(), 4, nil, func(context.Context, int) error { return nil }); err != nil {
		t.Fatalf("empty items: %v", err)
	}
}

func TestRunParallelWorkers(t *testing.T) {
	t.Parallel()

	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	var mu sync.Mutex
	seen := make(map[int]struct{})
	err := RunParallel(context.Background(), 4, items, func(ctx context.Context, n int) error {
		mu.Lock()
		seen[n] = struct{}{}
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("parallel run: %v", err)
	}
	if len(seen) != len(items) {
		t.Fatalf("processed %d items, want %d", len(seen), len(items))
	}
}

func TestRunParallelFirstError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	err := RunParallel(context.Background(), 4, []int{1, 2, 3, 4, 5}, func(ctx context.Context, n int) error {
		if n == 3 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("got err %v, want %v", err, want)
	}
}

func TestRunParallelCapsWorkersToItems(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	err := RunParallel(context.Background(), 100, []int{1, 2}, func(ctx context.Context, n int) error {
		calls.Add(1)
		return nil
	})
	if err != nil || calls.Load() != 2 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}
