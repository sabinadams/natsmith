package nats

import (
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestConnectJSM(t *testing.T) {
	srv := testutil.StartServer(t)

	nc, mgr, err := ConnectJSM(srv.ClientURL(), "", time.Second)
	if err != nil {
		t.Fatalf("ConnectJSM: %v", err)
	}
	defer nc.Close()

	if mgr == nil {
		t.Fatal("expected jetstream manager")
	}
}
