//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func TestStartNATSPair(t *testing.T) {
	pair := StartNATSPair(t)

	sourceJS := JetStream(t, pair.SourceURL)
	destJS := JetStream(t, pair.DestURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := sourceJS.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SMOKE"}); err != nil {
		t.Fatalf("source kv: %v", err)
	}
	if _, err := destJS.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "SMOKE"}); err != nil {
		t.Fatalf("dest kv: %v", err)
	}
}
