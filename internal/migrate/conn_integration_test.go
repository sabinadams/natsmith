package migrate

import (
	"testing"
	"time"
)

func TestConnectInvalidURL(t *testing.T) {
	t.Parallel()

	_, _, err := Connect("nats://127.0.0.1:1", "", time.Second)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestConnectUsesDefaultTimeout(t *testing.T) {
	t.Parallel()

	_, _, err := Connect("nats://127.0.0.1:1", "", 0)
	if err == nil {
		t.Fatal("expected connection error")
	}
}
