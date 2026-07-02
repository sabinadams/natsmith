package objects

import (
	"errors"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

func TestReportMessages(t *testing.T) {
	t.Parallel()

	snap := BucketSnapshot{
		Listed:       []string{"a.txt"},
		Migratable:   []*jetstream.ObjectInfo{{ObjectMeta: jetstream.ObjectMeta{Name: "a.txt"}}},
		MessageCount: 3,
	}
	testErr := errors.New("boom")

	tests := []struct {
		name string
		got  string
		want []string
	}{
		{
			name: "scan fail",
			got:  ScanFailMessage("files", 1, 2, testErr),
			want: []string{"Object store files (1/2)", "failed to scan bucket", "boom"},
		},
		{
			name: "dry run",
			got:  DryRunScanMessage("files", 2, 2, snap, 1),
			want: []string{"1 listed", "1 meta-active", "1 meta-omitted", "3 meta messages"},
		},
		{
			name: "scan ok",
			got:  ScanOKMessage("files", 1, 2, 1, 1, 2, 3),
			want: []string{"1 listed", "1 migratable", "2 omitted", "3 meta messages"},
		},
		{
			name: "dest missing",
			got:  DestBucketMissingMessage("files", 1, 2, testErr),
			want: []string{"destination bucket not found", "boom"},
		},
		{
			name: "source open fail",
			got:  SourceOpenFailMessage("files", 1, 2, testErr),
			want: []string{"failed to open source", "boom"},
		},
		{
			name: "copy fail",
			got:  CopyFailMessage("files", 1, 2, testErr),
			want: []string{"migration failed", "boom"},
		},
		{
			name: "copy count mismatch",
			got:  CopyCountMismatchMessage("files", 5, 3, 1, 1),
			want: []string{"expected 5 processed", "migrated=3", "skipped=1", "failed=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, fragment := range tt.want {
				if !strings.Contains(tt.got, fragment) {
					t.Fatalf("message %q missing %q", tt.got, fragment)
				}
			}
		})
	}
}
