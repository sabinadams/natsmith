package progress

import "fmt"

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

func styled(code, s string) string {
	if !UseColor() {
		return s
	}
	return code + s + ansiReset
}

func Bold(s string) string   { return styled(ansiBold, s) }
func Dim(s string) string    { return styled(ansiDim, s) }
func Green(s string) string  { return styled(ansiGreen, s) }
func Red(s string) string    { return styled(ansiRed, s) }
func Yellow(s string) string { return styled(ansiYellow, s) }
func Cyan(s string) string   { return styled(ansiCyan, s) }

func SuccessMark() string { return successMark() }
func FailMark() string    { return failMark() }
func WarnMark() string    { return warnMark() }
func InfoMark() string    { return infoMark() }

func successMark() string {
	if UseColor() {
		return Green("✓")
	}
	return "✓"
}

func failMark() string {
	if UseColor() {
		return Red("✗")
	}
	return "✗"
}

func warnMark() string {
	if UseColor() {
		return Yellow("!")
	}
	return "!"
}

func infoMark() string {
	if UseColor() {
		return Dim("·")
	}
	return "·"
}

func labelKV(s string) string {
	if UseColor() {
		return Cyan(s)
	}
	return s
}

func formatIndexedLine(mark, kind, name string, index, total int, detail string) string {
	return fmt.Sprintf("  %s %s %s (%d/%d) — %s", mark, labelKV(kind), name, index, total, detail)
}
