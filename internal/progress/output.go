package progress

import (
	"os"
	"sync"

	"golang.org/x/term"
)

var (
	outputMu sync.RWMutex
	quiet    bool
	jsonMode bool
)

// SetOutputMode configures global CLI output behavior (--quiet, --json).
func SetOutputMode(q, j bool) {
	outputMu.Lock()
	quiet = q
	jsonMode = j
	outputMu.Unlock()
}

// IsQuiet reports whether non-essential human output is suppressed.
func IsQuiet() bool {
	outputMu.RLock()
	defer outputMu.RUnlock()
	return quiet
}

// IsJSON reports whether structured JSON events are emitted to stdout.
func IsJSON() bool {
	outputMu.RLock()
	defer outputMu.RUnlock()
	return jsonMode
}

// UseColor reports whether ANSI styling is allowed (TTY + NO_COLOR unset).
func UseColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// ShowHumanOutput reports whether decorated stderr lines should be printed.
func ShowHumanOutput() bool {
	return !IsQuiet() && !IsJSON()
}
