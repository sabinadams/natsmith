package migration

import "fmt"

// ExitError signals a non-zero process exit code from a command handler to cmd.Execute.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
