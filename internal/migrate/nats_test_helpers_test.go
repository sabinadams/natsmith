package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func startNATSServer(t *testing.T) *server.Server {
	t.Helper()

	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := test.RunServer(&opts)
	t.Cleanup(func() { srv.Shutdown() })
	return srv
}

func connectNATS(t *testing.T, url string) *nats.Conn {
	t.Helper()

	nc, err := nats.Connect(url, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return nc
}

func newJetStream(t *testing.T, nc *nats.Conn) jetstream.JetStream {
	t.Helper()

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	return js
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}
