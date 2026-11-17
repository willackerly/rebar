package config

import (
	"os"
	"os/exec"
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
	// RCFile is the settings file to read instead of <root>/.rebarrc
	// ("" keeps <root>/.rebarrc; REBAR_RCFILE is not consulted).
	RCFile string
}

var (
	envMu    sync.Mutex
	envOver  atomic.Pointer[Env]
	envCache sync.Map // *Env → map[string]string
)

// WithEnv runs f with e in place of the process environment for everything
// that reads rebar's settings (Getenv, RCFile, Environ for child
// processes). Calls are serialized: one override at a time.
func WithEnv(e *Env, f func()) {
	envMu.Lock()
	envOver.Store(e)
	defer func() {
		envOver.Store(nil)
		envCache.Delete(e)
		envMu.Unlock()
	}()
	f()
}

// Getenv is os.Getenv, or the value in the WithEnv override.
func Getenv(key string) string {
	e := envOver.Load()
	if e == nil {
		return os.Getenv(key)
	}
	m, ok := envCache.Load(e)
	if !ok {
		vars := map[string]string{}
		for _, kv := range e.Vars {
			if k, v, ok := strings.Cut(kv, "="); ok {
				vars[k] = v
			}
		}
		m, _ = envCache.LoadOrStore(e, vars)
	}
	return m.(map[string]string)[key]
}

// Environ is the environment for a child process: nil (inherit the
// process's) without an override, else the override's Vars.
func Environ() []string {
	if e := envOver.Load(); e != nil {
		return append([]string{}, e.Vars...)
	}
	return nil
}

// Overridden reports whether a WithEnv override is in effect.
func Overridden() bool { return envOver.Load() != nil }

// Command is exec.Command with the WithEnv override's environment, if any.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = Environ()
	return cmd
}
