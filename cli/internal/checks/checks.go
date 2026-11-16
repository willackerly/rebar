// Package checks holds native (Go) implementations of rebar's check
// scripts. Each one is registered under its script's name (check-todos for
// scripts/check-todos.sh) and behaves like the script: same arguments, same
// .rebarrc settings, same output and exit code.
//
// A check registered here is the default for `rebar check` and `rebar run`;
// REBAR_CHECK_IMPL=shell selects the scripts instead until they are deleted
// (see feedback/2026-11-14-go-native-checks.md).
package checks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Context is what a check runs against.
type Context struct {
	// Root is the project root, whose .rebarrc the check reads.
	Root string
	// Dir is the working directory the script would run in: relative
	// arguments and the default source directories resolve against it, and
	// paths in the output are relative to it, as the script's grep prints
	// them. Empty means Root (`rebar check` and `rebar run` run the scripts
	// in the project root).
	Dir string
	// Install is the scripts directory of the rebar install the CLI found
	// ("" when there is none). The suites (ci-check, pre-commit,
	// cold-start-checks) run the checks that have no native implementation
	// from it, as the suite scripts run their sibling scripts.
	Install string
	Stdout  io.Writer
	Stderr  io.Writer
}

// WorkDir is the directory relative paths resolve against: Dir, else Root.
func (c *Context) WorkDir() string {
	if c.Dir != "" {
		return c.Dir
	}
	return c.Root
}

// Check is a native implementation of one check script.
type Check struct {
	// Name is the script's base name without .sh.
	Name string
	// Run takes the script's arguments and returns its exit code.
	Run func(ctx *Context, args []string) int
}

var registry = map[string]*Check{}

func register(c *Check) { registry[c.Name] = c }

// Register adds a native implementation from another package (the suites
// in internal/suite, which run the checks registered here).
func Register(c *Check) { register(c) }

// Lookup returns the native check for a script name, with or without .sh.
func Lookup(name string) (*Check, bool) {
	c, ok := registry[strings.TrimSuffix(name, ".sh")]
	return c, ok
}

// Names returns the registered checks, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Implementation selectors for REBAR_CHECK_IMPL.
const (
	ImplShell = "shell"
	ImplGo    = "go"
)

var warnImplOnce sync.Once

// Impl is the implementation REBAR_CHECK_IMPL selects: ImplShell for
// "shell", otherwise ImplGo (the default). An unknown value is warned about
// once and treated as go.
func Impl() string {
	switch v := strings.TrimSpace(os.Getenv("REBAR_CHECK_IMPL")); v {
	case ImplShell:
		return ImplShell
	case "", ImplGo:
		return ImplGo
	default:
		warnImplOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "rebar: ignoring REBAR_CHECK_IMPL=%q (use shell or go); running the native checks\n", v)
		})
		return ImplGo
	}
}

// Native returns the native check to run for a script name, if the
// selected implementation is go and one is registered.
func Native(name string) (*Check, bool) {
	if Impl() != ImplGo {
		return nil, false
	}
	return Lookup(name)
}

// ScriptEnv is the environment the CLI adds to a script it runs so that the
// scripts of the install at installScripts that have a native counterpart
// hand off to it (_rebar_native in scripts/_rebar-config.sh):
// REBAR_NATIVE_CHECKS lists the checks to run natively, REBAR_BIN names this
// binary, and REBAR_NATIVE_SCRIPTS is the install's scripts directory
// (symlinks resolved). Only a script in that directory hands off, so a
// project's own copy of a check, which may be customized, always runs
// itself, even one made from a script that has the hand-off line.
//
// Pass "" for installScripts when the script being run is a project's copy
// (its siblings are copies too), and under the shell implementation: the
// hand-off variables are then cleared, so values inherited from the
// caller's environment cannot route anything to a binary.
func ScriptEnv(installScripts string) []string {
	off := []string{"REBAR_NATIVE_CHECKS=", "REBAR_NATIVE_SCRIPTS="}
	if Impl() != ImplGo || installScripts == "" {
		return off
	}
	exe, err := os.Executable()
	if err != nil {
		return off
	}
	dir, err := filepath.Abs(installScripts)
	if err != nil {
		return off
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return []string{"REBAR_BIN=" + exe, "REBAR_NATIVE_CHECKS=" + strings.Join(Names(), " "), "REBAR_NATIVE_SCRIPTS=" + dir}
}
