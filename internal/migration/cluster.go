package migration

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	sm "github.com/sabinadams/natsmith/internal/nats"
	"github.com/sabinadams/natsmith/internal/progress"
)

// Clusters holds JetStream contexts for a source/destination pair.
type Clusters struct {
	Ctx      context.Context
	SourceJS jetstream.JetStream
	DestJS   jetstream.JetStream

	sourceNC *nats.Conn
	destNC   *nats.Conn
	cancel   context.CancelFunc
}

// ConnectClusters opens source and destination NATS connections.
// status is called with human-readable progress lines (may be nil).
func ConnectClusters(cfg BaseConfig, status func(string)) (*Clusters, error) {
	ctx, cancel := context.WithCancel(context.Background())
	progress.RegisterInterruptCancel(cancel)

	if status != nil {
		status("Connecting to source...")
	}
	sourceNC, sourceJS, err := sm.Connect(cfg.SourceURL, cfg.SourceCreds, cfg.RequestTimeout)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("source: %w", err)
	}

	if status != nil {
		status("Connecting to destination...")
	}
	destNC, destJS, err := sm.Connect(cfg.DestURL, cfg.DestCreds, cfg.RequestTimeout)
	if err != nil {
		sourceNC.Close()
		cancel()
		return nil, fmt.Errorf("destination: %w", err)
	}

	return &Clusters{
		Ctx:      ctx,
		SourceJS: sourceJS,
		DestJS:   destJS,
		sourceNC: sourceNC,
		destNC:   destNC,
		cancel:   cancel,
	}, nil
}

// Close releases connections and cancels the run context.
func (c *Clusters) Close() {
	if c == nil {
		return
	}
	if c.destNC != nil {
		c.destNC.Close()
	}
	if c.sourceNC != nil {
		c.sourceNC.Close()
	}
	if c.cancel != nil {
		c.cancel()
	}
}
