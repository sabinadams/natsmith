package cmd

import (
	"strings"
	"testing"
)

func TestRootHelpIncludesOutputFlags(t *testing.T) {
	buf := new(strings.Builder)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	out := buf.String()
	for _, flag := range []string{"--quiet", "--json"} {
		if !strings.Contains(out, flag) {
			t.Fatalf("help missing %q:\n%s", flag, out)
		}
	}
}
