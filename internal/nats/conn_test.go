package nats

import (
	"strings"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestConnectSuccess(t *testing.T) {
	srv := testutil.StartServer(t)

	nc, js, err := Connect(srv.ClientURL(), "", time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer nc.Close()

	if js == nil {
		t.Fatal("expected jetstream context")
	}
}

func TestConnectWithMissingCredsFile(t *testing.T) {
	srv := testutil.StartServer(t)

	_, _, err := Connect(srv.ClientURL(), "/path/does/not/exist.creds", time.Second)
	if err == nil {
		t.Fatal("expected error for missing creds file")
	}
}

func TestConnectInvalidURL(t *testing.T) {
	t.Parallel()

	_, _, err := Connect("nats://127.0.0.1:1", "", time.Second)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "connect to") {
		t.Fatalf("error = %v", err)
	}
}

func TestConnectUsesDefaultTimeout(t *testing.T) {
	t.Parallel()

	_, _, err := Connect("nats://127.0.0.1:1", "", 0)
	if err == nil {
		t.Fatal("expected connection error")
	}
}
