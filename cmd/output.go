package cmd

import (
	"github.com/sabinadams/natsmith/internal/progress"
	"github.com/spf13/cobra"
)

var (
	globalQuiet bool
	globalJSON  bool
)

func registerOutputFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.BoolVar(&globalQuiet, "quiet", false, "print errors and final summary only")
	flags.BoolVar(&globalJSON, "json", false, "emit structured JSON events to stdout")
	cmd.PersistentPreRun = func(*cobra.Command, []string) {
		progress.SetOutputMode(globalQuiet, globalJSON)
	}
}
