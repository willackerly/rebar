//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package testutil

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Alive reports whether pid is a running process (not gone, and not a
// zombie waiting for init to reap it).
func Alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

// WaitGone fails the test unless process pid (described by what) is gone
// within limit.
func WaitGone(t testing.TB, what string, pid int, limit time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(limit); Alive(pid); {
		if time.Now().After(deadline) {
			t.Errorf("%s (pid %d) still running after %v", what, pid, limit)
			syscall.Kill(pid, syscall.SIGKILL)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ReadPIDFile waits up to limit for path to hold a newline-terminated pid
// (written by `echo $$ > path`) and returns it.
func ReadPIDFile(t testing.TB, path string, limit time.Duration) int {
	t.Helper()
	for deadline := time.Now().Add(limit); ; {
		if b, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			return pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pid in %s after %v", path, limit)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
