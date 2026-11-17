package checks

import (
	"github.com/willackerly/rebar/cli/internal/config"
	"io"
	"os"
)

// UseColour reports whether a check writing to w colours its output the way
// the scripts decide it (`[ -t 1 ] && [ -z "${NO_COLOR:-}" ]`, no-color.org):
// w is a terminal and NO_COLOR is unset or empty. Checks that colour call
// it once per run with ctx.Stdout.
func UseColour(w io.Writer) bool {
	return config.Getenv("NO_COLOR") == "" && IsTerminal(w)
}

// IsTerminal is the shell's `test -t` for w: w is an *os.File open on a
// terminal (a character device that answers the terminal ioctl, so not
// /dev/null).
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminal(f.Fd())
}

// TerminalWidth is the column count of the terminal w is open on (the
// TIOCGWINSZ ioctl), 0 when w is not a terminal or does not say.
func TerminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	return terminalWidth(f.Fd())
}
