package checks

import (
	"io"
	"os"
)

// UseColour reports whether a check writing to w colours its output the way
// the scripts decide it (`[ -t 1 ] && [ -z "${NO_COLOR:-}" ]`, no-color.org):
// w is a terminal and NO_COLOR is unset or empty. Checks that colour call
// it once per run with ctx.Stdout.
func UseColour(w io.Writer) bool {
	return os.Getenv("NO_COLOR") == "" && IsTerminal(w)
}

// IsTerminal is the shell's `test -t` for w: w is an *os.File open on a
// terminal (a character device that answers the terminal ioctl, so not
// /dev/null).
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminal(f.Fd())
}
