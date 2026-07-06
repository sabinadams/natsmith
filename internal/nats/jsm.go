package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

// ConnectJSM opens a NATS connection and JetStream manager (stream admin APIs).
func ConnectJSM(url, creds string, requestTimeout time.Duration) (*nats.Conn, *jsm.Manager, error) {
	if requestTimeout <= 0 {
		requestTimeout = DefaultRequestTimeout
	}

	opts := []nats.Option{
		nats.Name("natsmith"),
		nats.MaxReconnects(-1),
		nats.Timeout(requestTimeout),
	}
	if creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", url, err)
	}

	mgr, err := jsm.New(nc, jsm.WithTimeout(requestTimeout))
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create jetstream manager: %w", err)
	}

	return nc, mgr, nil
}

// RunContext returns a background context for a single-cluster operation.
func RunContext() context.Context {
	return context.Background()
}
