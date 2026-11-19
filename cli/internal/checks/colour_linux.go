//go:build linux

package checks

import "syscall"

const ioctlGetTermios = syscall.TCGETS
