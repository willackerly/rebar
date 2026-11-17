package config

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Env stands in for the process environment and the .rebarrc location
// while rebar runs its native checks in process on settings that are not
// written yet (`rebar migrate` compares a project's legacy copies against
// them under a scrubbed environment and candidate settings). See WithEnv.
type Env struct {
	// Vars is the whole environment, KEY=VALUE as os.Environ returns it.
	Vars []string
	// RCFile is the settings file to read instead of <root>/.rebarrc ("":
	// REBAR_RCFILE in Vars, else <root>/.rebarrc; the process's
	// REBAR_RCFILE is not consulted).
	RCFile string
}

var (
	envMu   sync.Mutex          // held for the whole of a WithEnv call
	envOver atomic.Pointer[Env] // the override in effect
	varsMu  sync.Mutex
	vars    map[string]string // the override's variables, as Setenv changes them
)

// WithEnv runs f with e in place of the process environment for everything
// that reads rebar's settings (Getenv, Setenv, RCFile, Environ and Command
// for child processes). Calls are serialized: one override at a time, so
// code outside f that reads settings while another goroutine is inside
// must use WithEnv too.
func WithEnv(e *Env, f func()) {
	envMu.Lock()
	m := map[string]string{}
	for _, kv := range e.Vars {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v // the last of duplicate keys wins, as in exec
		}
	}
	varsMu.Lock()
	vars = m
	varsMu.Unlock()
	envOver.Store(e)
	defer func() {
		envOver.Store(nil)
		varsMu.Lock()
		vars = nil
		varsMu.Unlock()
		envMu.Unlock()
	}()
	f()
}

// Getenv is os.Getenv, or the value in the WithEnv override.
func Getenv(key string) string {
	if envOver.Load() == nil {
		return os.Getenv(key)
	}
	varsMu.Lock()
	defer varsMu.Unlock()
	return vars[key]
}

// Setenv is os.Setenv, or a change to the WithEnv override (the process
// environment is left alone), for markers such as _REBAR_CHECKS_VALIDATED
// that a run sets for the checks after it.
func Setenv(key, value string) {
	if envOver.Load() == nil {
		os.Setenv(key, value)
		return
	}
	varsMu.Lock()
	defer varsMu.Unlock()
	vars[key] = value
}

// Environ is the environment for a child process: nil (inherit the
// process's) without an override, else the override's variables.
func Environ() []string {
	if envOver.Load() == nil {
		return nil
	}
	varsMu.Lock()
	defer varsMu.Unlock()
	out := make([]string, 0, len(vars))
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// Overridden reports whether a WithEnv override is in effect.
func Overridden() bool { return envOver.Load() != nil }

// Command is exec.Command with the WithEnv override's environment, if any.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = Environ()
	return cmd
}
