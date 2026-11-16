//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package checks

import "syscall"

const ioctlGetTermios = syscall.TIOCGETA
