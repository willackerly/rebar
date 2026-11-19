package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Under WithEnv, settings come from the override only: its variables (not
// the process's), its rc file (not REBAR_RCFILE), and child processes get
// its environment.
func TestWithEnv(t *testing.T) {
	root := t.TempDir()
	own := filepath.Join(root, ".rebarrc")
	alt := filepath.Join(root, "candidate")
	os.WriteFile(own, []byte("source_dirs = own\n"), 0o644)
	os.WriteFile(alt, []byte("source_dirs = candidate\n"), 0o644)
	proc := filepath.Join(root, "process")
	os.WriteFile(proc, []byte("source_dirs = process\n"), 0o644)
	t.Setenv("REBAR_RCFILE", proc)
	t.Setenv("REBAR_CONTRACTS_DIR", "from-process")

	WithEnv(&Env{Vars: []string{"PATH=/bin", "REBAR_TIER=2"}, RCFile: alt}, func() {
		if !Overridden() {
			t.Error("Overridden() = false inside WithEnv")
		}
		if got := RCValue(root, "source_dirs"); got != "candidate" {
			t.Errorf("RCValue = %q, want the override's file", got)
		}
		if got := Getenv("REBAR_TIER"); got != "2" {
			t.Errorf("Getenv(REBAR_TIER) = %q", got)
		}
		if got := ContractsDirFor(root); got != DefaultContractsDir {
			t.Errorf("ContractsDirFor = %q: the process's REBAR_CONTRACTS_DIR leaked in", got)
		}
		if env := Command("true").Env; len(env) != 2 || env[0] != "PATH=/bin" {
			t.Errorf("Command env = %v", env)
		}
	})
	WithEnv(&Env{}, func() {
		if got := RCValue(root, "source_dirs"); got != "own" {
			t.Errorf("no RCFile: RCValue = %q, want <root>/.rebarrc", got)
		}
	})
	if Overridden() || Command("true").Env != nil {
		t.Error("override still in effect after WithEnv")
	}
	if got := RCValue(root, "source_dirs"); got != "process" {
		t.Errorf("after WithEnv: RCValue = %q, want REBAR_RCFILE's again", got)
	}
}

// Setenv/Getenv/Environ from a goroutine outside WithEnv while overrides
// come and go must not panic (assignment to a nil map) or race: each call
// sees either the process environment or one whole override. Run with
// -race.
func TestEnvConcurrentWithOverride(t *testing.T) {
	t.Setenv("_REBAR_ENV_RACE", "")
	root := t.TempDir()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2000; i++ {
			WithEnv(&Env{Vars: []string{"A=1"}}, func() {
				if v := Getenv("A"); v != "1" {
					t.Errorf("Getenv(A) = %q inside WithEnv", v)
				}
			})
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
		}
		Setenv("_REBAR_ENV_RACE", "x")
		_ = Getenv("_REBAR_ENV_RACE")
		_ = Environ()
		_ = RCFile(root)
		_ = Overridden()
	}
}
