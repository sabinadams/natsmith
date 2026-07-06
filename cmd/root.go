package cmd

import (
	"errors"
	"os"

	backupcmd "github.com/sabinadams/natsmith/cmd/backup"
	migratecmd "github.com/sabinadams/natsmith/cmd/migrate"
	restorecmd "github.com/sabinadams/natsmith/cmd/restore"
	"github.com/sabinadams/natsmith/internal/migration"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "natsmith",
	Short: "CLI tooling for NATS and JetStream",
	Long:  "Unofficial CLI toolkit for NATS and JetStream. Not affiliated with Synadia.",
}

func init() {
	rootCmd.AddCommand(migratecmd.Command())
	rootCmd.AddCommand(backupcmd.Command())
	rootCmd.AddCommand(restorecmd.Command())
}

// Execute runs the natsmith CLI and exits with a non-zero status on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var exitErr *migration.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
