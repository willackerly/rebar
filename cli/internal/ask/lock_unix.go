//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package ask

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive flock on path, an existing file or
// directory (opened read-only, never written); wait false gives up at once
// when another process holds it. The returned function releases it.
func lockFile(path string, wait bool) (func(), bool) {
	f, err := os.Open(path)
	if err != nil {
		return func() {}, true // nothing to lock: run unlocked, as before
	}
	how := syscall.LOCK_EX
	if !wait {
		how |= syscall.LOCK_NB
	}
	for {
		err = syscall.Flock(int(f.Fd()), how)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, false
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, true
}
