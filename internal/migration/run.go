package migration

// CompleteRun prints the final summary and returns ExitError when exitCode is non-zero.
func CompleteRun(kind string, summary Summary, exitCode int) error {
	PrintSummary(kind, summary)
	if exitCode != 0 {
		return &ExitError{Code: exitCode}
	}
	return nil
}
