package migrate

import (
	"context"
	"sync"
)

// RunParallel runs fn over items with the given number of workers.
// It cancels remaining work on the first error.
func RunParallel[T any](ctx context.Context, workers int, items []T, fn func(ctx context.Context, item T) error) error {
	if len(items) == 0 {
		return nil
	}
	if workers <= 1 {
		for _, item := range items {
			if err := fn(ctx, item); err != nil {
				return err
			}
		}
		return nil
	}
	if workers > len(items) {
		workers = len(items)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan T, workers)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error

	recordErr := func(err error) {
		if err != nil {
			once.Do(func() {
				firstErr = err
				cancel()
			})
		}
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range ch {
				if ctx.Err() != nil {
					return
				}
				if err := fn(ctx, item); err != nil {
					recordErr(err)
					return
				}
			}
		}()
	}

loop:
	for _, item := range items {
		select {
		case <-ctx.Done():
			break loop
		case ch <- item:
		}
	}
	close(ch)
	wg.Wait()

	return firstErr
}
