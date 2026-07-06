package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// DefaultRequestTimeout is the default per-request JetStream API timeout.
const DefaultRequestTimeout = 30 * time.Second

func clientOptions(creds string, requestTimeout time.Duration) []nats.Option {
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
	return opts
}

// Connect opens a NATS connection and JetStream context.
func Connect(url, creds string, requestTimeout time.Duration) (*nats.Conn, jetstream.JetStream, error) {
	if requestTimeout <= 0 {
		requestTimeout = DefaultRequestTimeout
	}

	nc, err := nats.Connect(url, clientOptions(creds, requestTimeout)...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", url, err)
	}

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(requestTimeout))
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return nc, js, nil
}
