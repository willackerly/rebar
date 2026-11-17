// Package checks holds native (Go) implementations of rebar's check
// scripts. Each one is registered under its script's name (check-todos for
// scripts/check-todos.sh) and behaves like the script: same arguments, same
// .rebarrc settings, same output and exit code.
//
// The scripts are gone (rebar 3.2): these are rebar's only implementation.
// `rebar check` and `rebar run` run them; a project's own copy of a script
// still runs instead, as the adopter's code (see cmd/scripts.go).
package checks

import (
	"io"
	"sort"
	"strings"
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
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
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
