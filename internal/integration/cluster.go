//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
)

const natsImage = "nats:2.10-alpine"

// Pair holds client URLs for a source and destination NATS server started in separate containers.
type Pair struct {
	SourceURL string
	DestURL   string
}

// StartNATSPair starts two JetStream-enabled NATS containers that simulate separate clusters.
func StartNATSPair(t *testing.T) Pair {
	t.Helper()
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	source, err := tcnats.Run(ctx, natsImage, tcnats.WithArgument("js", ""))
	if err != nil {
		t.Fatalf("start source nats: %v", err)
	}
	t.Cleanup(func() {
		_ = source.Terminate(context.Background())
	})

	dest, err := tcnats.Run(ctx, natsImage, tcnats.WithArgument("js", ""))
	if err != nil {
		t.Fatalf("start destination nats: %v", err)
	}
	t.Cleanup(func() {
		_ = dest.Terminate(context.Background())
	})

	sourceURL, err := source.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("source connection string: %v", err)
	}
	destURL, err := dest.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("destination connection string: %v", err)
	}

	return Pair{SourceURL: sourceURL, DestURL: destURL}
}

func requireDocker(t *testing.T) {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("docker not available: %v", err)
	}
	defer cli.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(pingCtx); err != nil {
		t.Skipf("docker not reachable: %v", err)
	}
}
