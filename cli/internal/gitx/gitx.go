// Package gitx runs git the way rebar's check scripts do: as the git
// command, in a given directory, with the user's configuration (so quoting,
// abbreviation and rename settings apply exactly as they do to the
// scripts), and reads its output as the shell would.
package gitx

import (
	"bytes"
	"errors"
	"github.com/willackerly/rebar/cli/internal/config"
	"io"
	"os/exec"
	"strings"
)

// NotFound is the exit code Run reports when git cannot be started, as a
// shell reports a missing command.
const NotFound = 127

// Run runs git with args in dir and returns its standard output and exit
// code. Its standard error goes to errw (nil discards it, as 2>/dev/null).
func Run(dir string, errw io.Writer, args ...string) (string, int) {
	cmd := config.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if errw != nil {
		cmd.Stderr = errw
	}
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.String(), exit.ExitCode()
	}
	return out.String(), NotFound
}

// Subst is $(git args 2>/dev/null) run in dir: standard output without its
// trailing newlines, whatever the exit code.
func Subst(dir string, args ...string) string {
	out, _ := Run(dir, nil, args...)
	return strings.TrimRight(out, "\n")
}

// Lines splits output into the lines `while IFS= read -r line` reads from
// it: every newline-terminated line, without its newline. (A last line
// with no newline makes read fail, so the loop never sees it; git always
// ends its output with one.)
func Lines(out string) []string {
	if i := strings.LastIndexByte(out, '\n'); i >= 0 {
		return strings.Split(out[:i], "\n")
	}
	return nil
}

// Available reports whether a git command can be found (`command -v git`).
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Message is a commit's subject and full message as the scripts read them:
// $(git log -1 --format=%s REV) and $(git log -1 --format=%B REV), with
// git's standard error passed to errw (nil discards it).
func Message(dir string, errw io.Writer, rev string) (subject, body string) {
	s, _ := Run(dir, errw, "log", "-1", "--format=%s", rev)
	b, _ := Run(dir, errw, "log", "-1", "--format=%B", rev)
	return strings.TrimRight(s, "\n"), strings.TrimRight(b, "\n")
}
