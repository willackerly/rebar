// Package checks holds native (Go) implementations of rebar's check
// scripts. Each one is registered under its script's name (check-todos for
// scripts/check-todos.sh) and behaves like the script: same arguments, same
// .rebarrc settings, same output and exit code.
//
// During the transition the shell script stays the default; REBAR_CHECK_IMPL=go
// makes `rebar check` and `rebar run` use the native implementation of every
// check registered here (see feedback/2026-11-14-go-native-checks.md).
package checks

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

// Context is what a check runs against.
type Context struct {
	// Root is the project root. Relative arguments and the default source
	// directories resolve against it (the scripts run there).
	Root   string
	Stdout io.Writer
	Stderr io.Writer
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

// Impl is the implementation REBAR_CHECK_IMPL selects: ImplGo for "go",
// otherwise ImplShell (the default). An unknown value is warned about once
// and treated as shell.
func Impl() string {
	switch v := strings.TrimSpace(os.Getenv("REBAR_CHECK_IMPL")); v {
	case "", ImplShell:
		return ImplShell
	case ImplGo:
		return ImplGo
	default:
		warnImplOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "rebar: ignoring REBAR_CHECK_IMPL=%q (use shell or go); running the shell checks\n", v)
		})
		return ImplShell
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

// ScriptEnv is the environment the CLI adds to the scripts it runs so that
// a script with a native counterpart hands off to it (_rebar_native in
// scripts/_rebar-config.sh): REBAR_NATIVE_CHECKS lists the checks to run
// natively and REBAR_BIN names this binary. Under the shell implementation
// it is empty, and a REBAR_NATIVE_CHECKS inherited from the caller's
// environment is cleared.
func ScriptEnv() []string {
	if Impl() != ImplGo {
		return []string{"REBAR_NATIVE_CHECKS="}
	}
	exe, err := os.Executable()
	if err != nil {
		return []string{"REBAR_NATIVE_CHECKS="}
	}
	return []string{"REBAR_BIN=" + exe, "REBAR_NATIVE_CHECKS=" + strings.Join(Names(), " ")}
}
