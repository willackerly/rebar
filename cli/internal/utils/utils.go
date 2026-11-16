// Package utils holds the native (Go) implementations of rebar's utility
// scripts: refresh-context, agent-health, scan-consumers, rebar-doc,
// inbox-watch and flush-notifications. They are registered with the checks
// package under their scripts' names, so `rebar run <name>` and `rebar
// native <name>` find them and the install's scripts hand off to them
// (_rebar_native), and they behave like the scripts: same arguments,
// environment variables, output, files and exit codes.
package utils

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/gitx"
)

func init() {
	for name, run := range map[string]func(*checks.Context, []string) int{
		"refresh-context":     runRefreshContext,
		"agent-health":        runAgentHealth,
		"scan-consumers":      runScanConsumers,
		"rebar-doc":           runRebarDoc,
		"inbox-watch":         runInboxWatch,
		"flush-notifications": runFlushNotifications,
	} {
		checks.Register(&checks.Check{Name: name, Run: run})
	}
}

// toplevel is $(git rev-parse --show-toplevel 2>/dev/null || pwd) run in
// dir.
func toplevel(dir string) string {
	if out, code := gitx.Run(dir, nil, "rev-parse", "--show-toplevel"); code == 0 {
		return strings.TrimRight(out, "\n")
	}
	return dir
}

// shellBasename is basename(1): the last path component, trailing slashes
// ignored ("/" for "/").
func shellBasename(p string) string {
	t := strings.TrimRight(p, "/")
	if t == "" {
		if p == "" {
			return ""
		}
		return "/"
	}
	return filepath.Base(t)
}

// run runs name with args in dir, its output on stdout and stderr (nil
// discards), and returns its exit code as the shell reports it: 127 when
// it cannot be found, 126 when it cannot be run, 128+N when a signal ended
// it.
func run(dir string, stdout, stderr io.Writer, name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		if st, ok := exit.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			return 128 + int(st.Signal())
		}
		return exit.ExitCode()
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return 127
	}
	return 126
}

// cmdSubst is $(...) of output: trailing newlines and NUL bytes dropped.
func cmdSubst(out []byte) string {
	return strings.TrimRight(string(bytes.ReplaceAll(out, []byte{0}, nil)), "\n")
}

// readLines are the lines `while IFS= read -r line` reads from data: every
// newline-terminated line (a last line without one is not read).
func readLines(data []byte) []string {
	i := bytes.LastIndexByte(data, '\n')
	if i < 0 {
		return nil
	}
	return strings.Split(string(data[:i]), "\n")
}

// splitLines splits data into lines, keeping a last line that has no
// newline (grep, sed and awk read it).
func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// resolve is a path the script would open relative to its working
// directory dir.
func resolve(dir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}
