package progress

import (
	"fmt"
	"os"
	"strings"
)

// PrintHeader prints a styled command title.
func PrintHeader(title string) {
	width := headerWidth()
	line := strings.Repeat("─", width)
	if UseColor() {
		fmt.Fprintf(os.Stderr, "\n%s\n", Dim(line))
		fmt.Fprintf(os.Stderr, "  %s\n", Bold(Cyan(title)))
		fmt.Fprintf(os.Stderr, "%s\n", Dim(line))
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", title)
}
