//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package mcp

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// inOwnGroup starts cmd as the leader of a new process group, so that
// everything it starts (rebar ask's claude or other LLM backend, git, …)
// can be signalled with it, and makes cancelling cmd send SIGTERM to the
// whole group.
func inOwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		} else if err != nil {
			return err
		}
		return nil
	}
}

// killGroup sends SIGKILL to the process group pid leads (gone groups are
// ignored).
func killGroup(pid int) { syscall.Kill(-pid, syscall.SIGKILL) }

// groupAlive reports whether the process group pid led has members left.
func groupAlive(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
