//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package ask

import "syscall"

// accessOK is access(path, X_OK) == 0, what `[ -x path ]` asks.
func accessOK(path string) bool { return syscall.Access(path, 1) == nil }
