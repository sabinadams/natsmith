package kv

import (
	"errors"
	"strings"
	"testing"
)

func TestReportMessages(t *testing.T) {
	t.Parallel()

	testErr := errors.New("boom")

	tests := []struct {
		name string
		got  string
		want []string
	}{
		{
			name: "scan ok run",
			got:  ScanOKRunMessage("schema", 1, 2, BucketRunResult{Migratable: 1458}),
			want: []string{"KV schema (1/2)", "1458 migratable keys"},
		},
		{
			name: "scan ok run ghost skipped",
			got:  ScanOKRunMessage("schema", 1, 2, BucketRunResult{Migratable: 1667, GhostSkipped: 3}),
			want: []string{"1667 migratable keys (3 ghost skipped)"},
		},
		{
			name: "scan fail",
			got:  ScanFailMessage("schema", 2, 2, testErr),
			want: []string{"✗ KV schema (2/2)", "failed", "boom"},
		},
		{
			name: "dest missing",
			got:  DestBucketMissingMessage("schema", 1, 3, testErr),
			want: []string{"destination bucket not found", "boom"},
		},
		{
			name: "copy count mismatch",
			got:  CopyCountMismatchMessage("schema", 10, 8, 2),
			want: []string{"expected 10 migrated", "got 8", "skipped=2"},
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
