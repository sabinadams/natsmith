package progress

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	interruptMu     sync.Mutex
	interruptCancel context.CancelFunc
	interruptOnce   sync.Once
)

// RegisterInterruptCancel cancels the active command context on SIGINT/SIGTERM.
func RegisterInterruptCancel(cancel context.CancelFunc) {
	interruptMu.Lock()
	interruptCancel = cancel
	interruptMu.Unlock()
}

func installInterruptHandler(session *Session) {
	interruptOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		go func() {
			for range ch {
				interruptMu.Lock()
				cancel := interruptCancel
				interruptMu.Unlock()
				if cancel != nil {
					cancel()
				}
				if session != nil {
					session.markInterrupted()
				}
			}
		}()
	})
}
