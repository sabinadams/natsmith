package nats

import (
	"context"
	"sync"

	"github.com/sabinadams/natsmith/internal/progress"
)

var runContext struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// RunContext returns a cancellable context for a single-cluster operation.
func RunContext() context.Context {
	runContext.mu.Lock()
	defer runContext.mu.Unlock()

	if runContext.cancel != nil {
		runContext.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	runContext.cancel = cancel
	progress.RegisterInterruptCancel(cancel)
	return ctx
}
