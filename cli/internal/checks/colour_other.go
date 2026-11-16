//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || linux)

package checks

// isTerminal: without a terminal ioctl to ask, nothing is a terminal, so
// checks print no colour codes.
func isTerminal(uintptr) bool { return false }
