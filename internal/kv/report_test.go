package kv

import (
	"errors"
	"strings"
	"testing"
)

func TestReportMessages(t *testing.T) {
	t.Parallel()

	snap := BucketSnapshot{
		Listed:       []string{"a", "b"},
		Migratable:   []string{"a"},
		Omitted:      []string{"b"},
		MessageCount: 5,
	}
	testErr := errors.New("boom")

	tests := []struct {
		name string
		got  string
		want []string
	}{
		{
			name: "scan ok",
			got:  ScanOKMessage("schema", 1, 2, snap),
			want: []string{"KV schema (1/2)", "2 listed", "1 migratable", "1 omitted", "5 stream messages"},
		},
		{
			name: "scan fail",
			got:  ScanFailMessage("schema", 2, 2, testErr),
			want: []string{"✗ KV schema (2/2)", "failed to scan bucket", "boom"},
		},
		{
			name: "dest missing",
			got:  DestBucketMissingMessage("schema", 1, 3, testErr),
			want: []string{"destination bucket not found", "boom"},
		},
		{
			name: "copy fail",
			got:  CopyFailMessage("schema", 1, 3, testErr),
			want: []string{"migration failed", "boom"},
		},
		{
			name: "copy count mismatch",
			got:  CopyCountMismatchMessage("schema", 10, 8, 2),
			want: []string{"expected 10 migrated", "got 8", "skipped=2"},
		},
		{
			name: "verify fail",
			got:  VerifyFailMessage("schema", 1, 1, testErr),
			want: []string{"verify failed", "boom"},
		},
		{
			name: "failures file",
			got:  FailuresFileErrorMessage("schema", testErr),
			want: []string{"failed to write failures file", "boom"},
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
