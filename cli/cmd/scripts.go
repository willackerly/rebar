package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/scripts"
)

// thinWrapperMarker tags a project script that only hands off to the rebar
// CLI (templates/project-bootstrap/scripts/). The CLI never runs such a
// file itself, which would loop back into the CLI.
const thinWrapperMarker = "# rebar-thin-wrapper"

// isThinWrapper reports whether the file at path carries thinWrapperMarker
// on a line of its own.
func isThinWrapper(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if string(bytes.TrimSpace(line)) == thinWrapperMarker {
			return true
		}
	}
	return false
}

// resolvedScript is the script the CLI runs for a name.
type resolvedScript struct {
	Path string
	// Legacy is true for a project's own copy of a rebar script, kept
	// working for adopters who have not moved to the thin wrappers yet.
	Legacy bool
}

// resolveScript finds the script to run for name (with or without .sh). A
// project's own scripts/<name> wins unless it is a thin wrapper; otherwise
// the rebar install's scripts/<name> runs. In the rebar source repo the
// two are the same file and nothing is legacy.
func resolveScript(projectRoot, rebarRoot, name string) (resolvedScript, bool) {
	if !strings.HasSuffix(name, ".sh") {
		name += ".sh"
	}
	local := filepath.Join(projectRoot, "scripts", name)
	if isFile(local) && !isThinWrapper(local) {
		return resolvedScript{Path: local, Legacy: !samePath(projectRoot, rebarRoot)}, true
	}
	if rebarRoot != "" {
		installed := filepath.Join(rebarRoot, "scripts", name)
		if isFile(installed) {
			return resolvedScript{Path: installed}, true
		}
	}
	return resolvedScript{}, false
}

// runRebarScript resolves name for the current project and runs it there.
// found is false when neither the project nor a rebar install has it.
//
// Under REBAR_CHECK_IMPL=go a check with a native implementation runs that
// instead, in process, even when no install has the script; a project's
// own legacy copy still wins, since it may carry customizations the native
// check does not have.
func runRebarScript(name string, args ...string) (exitCode int, found bool, err error) {
	s, ok := resolveScript(cfg.RepoRoot, findRebarRoot(), name)
	if c, native := nativeFor(name, s, ok); native {
		// The script would run in the project root, so its arguments
		// resolve there too.
		return runNativeCheck(c, cfg.RepoRoot, cfg.RepoRoot, args), true, nil
	}
	if !ok {
		return 0, false, nil
	}
	warnLegacy(s)
	code, err := scripts.RunInProject(s.Path, cfg.RepoRoot, s.nativeEnv(), args...)
	return code, true, err
}

// outputRebarScript is runRebarScript for the project at root and the
// install at rebarRoot, returning the combined output instead of printing
// it: a native check runs in process when one is selected, else the script.
func outputRebarScript(root, rebarRoot, name string, args ...string) (out []byte, code int, found bool, err error) {
	s, ok := resolveScript(root, rebarRoot, name)
	if c, native := nativeFor(name, s, ok); native {
		var buf bytes.Buffer
		code := c.Run(&checks.Context{Root: root, Dir: root, Stdout: &buf, Stderr: &buf}, args)
		return buf.Bytes(), code, true, nil
	}
	if !ok {
		return nil, 0, false, nil
	}
	out, code, err = scripts.OutputInProject(s.Path, root, s.nativeEnv(), args...)
	return out, code, true, err
}

// nativeEnv is the hand-off environment for running s (checks.ScriptEnv):
// the scripts of an install may hand off to the native checks; a project's
// legacy copy, and the copies it calls, never do.
func (s resolvedScript) nativeEnv() []string {
	if s.Legacy {
		return checks.ScriptEnv("")
	}
	return checks.ScriptEnv(filepath.Dir(s.Path))
}

// nativeFor returns the native check to run in place of the script
// resolved for name (s, found): one is registered, REBAR_CHECK_IMPL=go, and
// the script is not a project's legacy copy.
func nativeFor(name string, s resolvedScript, found bool) (*checks.Check, bool) {
	c, ok := checks.Native(name)
	return c, ok && !(found && s.Legacy)
}

func warnLegacy(s resolvedScript) {
	if s.Legacy {
		fmt.Fprintf(os.Stderr, "rebar: running the project's own %s (deprecated: rebar runs its checks from the install; move customizations to .rebarrc and delete the copy)\n",
			filepath.Join("scripts", filepath.Base(s.Path)))
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
