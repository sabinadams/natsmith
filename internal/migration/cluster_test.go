package migration

import (
	"strings"
	"testing"
	"time"

	"github.com/sabinadams/natsmith/internal/testutil"
)

func TestConnectClusters(t *testing.T) {
	srv := testutil.StartServer(t)
	url := srv.ClientURL()

	cfg := BaseConfig{
		SourceURL:      url,
		DestURL:        url,
		Workers:        1,
		RequestTimeout: 5 * time.Second,
	}

	var status []string
	clusters, err := ConnectClusters(cfg, func(msg string) { status = append(status, msg) })
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer clusters.Close()

	if clusters.Ctx == nil || clusters.SourceJS == nil || clusters.DestJS == nil {
		t.Fatal("expected non-nil cluster handles")
	}
	if len(status) != 2 || status[0] != "Connecting to source..." || status[1] != "Connecting to destination..." {
		t.Fatalf("status: %v", status)
	}

	info, err := clusters.SourceJS.AccountInfo(clusters.Ctx)
	if err != nil {
		t.Fatalf("source jetstream: %v", err)
	}
	if info == nil {
		t.Fatal("expected account info")
	}
}

func TestConnectClustersNilStatus(t *testing.T) {
	srv := testutil.StartServer(t)
	url := srv.ClientURL()

	cfg := BaseConfig{
		SourceURL: url,
		DestURL:   url,
		Workers:   1,
	}

	clusters, err := ConnectClusters(cfg, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	clusters.Close()
}

func TestConnectClustersSourceError(t *testing.T) {
	cfg := BaseConfig{
		SourceURL:      "nats://127.0.0.1:1",
		DestURL:        "nats://127.0.0.1:1",
		RequestTimeout: time.Second,
	}

	_, err := ConnectClusters(cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectClustersDestError(t *testing.T) {
	srv := testutil.StartServer(t)

	cfg := BaseConfig{
		SourceURL:      srv.ClientURL(),
		DestURL:        "nats://127.0.0.1:1",
		RequestTimeout: time.Second,
	}

	_, err := ConnectClusters(cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "destination") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClustersCloseNil(t *testing.T) {
	var clusters *Clusters
	clusters.Close()
}
