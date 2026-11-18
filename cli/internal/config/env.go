package config

import (
	"os"
	"os/exec"
	"path/filepath"
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

// envState is one WithEnv override: the Env and its variables as Setenv
// changes them. It is published and withdrawn as a whole (cur), and its map
// is never reset, so a reader that loaded it as a call ended still finds a
// valid map (and a late Setenv only changes the finished override's copy).
type envState struct {
	env  *Env
	mu   sync.Mutex
	vars map[string]string
}

var (
	envMu sync.Mutex               // held for the whole of a WithEnv call
	cur   atomic.Pointer[envState] // the override in effect
)

// WithEnv runs f with e in place of the process environment for everything
// that reads rebar's settings (Getenv, Setenv, RCFile, Environ and Command
// for child processes). Calls are serialized: one override at a time, so
// code outside f that reads settings while another goroutine is inside
// must use WithEnv too.
func WithEnv(e *Env, f func()) {
	envMu.Lock()
	defer envMu.Unlock()
	s := &envState{env: e, vars: map[string]string{}}
	for _, kv := range e.Vars {
		if k, v, ok := strings.Cut(kv, "="); ok {
			s.vars[k] = v // the last of duplicate keys wins, as in exec
		}
	}
	cur.Store(s)
	defer cur.Store(nil)
	f()
}

// override is the Env in effect (nil without WithEnv).
func override() *Env {
	if s := cur.Load(); s != nil {
		return s.env
	}
	return nil
}

// Getenv is os.Getenv, or the value in the WithEnv override.
func Getenv(key string) string {
	s := cur.Load()
	if s == nil {
		return os.Getenv(key)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vars[key]
}

// Setenv is os.Setenv, or a change to the WithEnv override (the process
// environment is left alone), for markers such as _REBAR_CHECKS_VALIDATED
// that a run sets for the checks after it.
func Setenv(key, value string) {
	s := cur.Load()
	if s == nil {
		os.Setenv(key, value)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vars[key] = value
}

// Environ is the environment for a child process: nil (inherit the
// process's) without an override, else the override's variables.
func Environ() []string {
	s := cur.Load()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.vars))
	for k, v := range s.vars {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// Overridden reports whether a WithEnv override is in effect.
func Overridden() bool { return cur.Load() != nil }

// Command is exec.Command with the WithEnv override's environment, if any.
// Under an override a name without a slash is looked up in the override's
// PATH (LookPath), not the process's.
func Command(name string, args ...string) *exec.Cmd {
	if !Overridden() || strings.Contains(name, "/") {
		cmd := exec.Command(name, args...)
		cmd.Env = Environ()
		return cmd
	}
	path, err := LookPath(name)
	if err != nil {
		cmd := exec.Command(name, args...)
		cmd.Path, cmd.Err = name, err
		cmd.Env = Environ()
		return cmd
	}
	cmd := exec.Command(path, args...)
	cmd.Args[0] = name
	cmd.Env = Environ()
	return cmd
}

// LookPath is exec.LookPath against Getenv("PATH"): the WithEnv override's
// PATH when one is in effect.
func LookPath(file string) (string, error) {
	if !Overridden() || strings.Contains(file, "/") {
		return exec.LookPath(file)
	}
	for _, dir := range filepath.SplitList(Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, file)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return path, nil
		}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}
