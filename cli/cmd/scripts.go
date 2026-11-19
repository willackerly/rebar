package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/scripts"
	"github.com/willackerly/rebar/cli/internal/suite"
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

// resolvedScript is a project's own script that `rebar run` (and the
// commands built on it) runs for a name.
type resolvedScript struct {
	Path string
	// Legacy is true for a project's own copy of one of rebar's scripts (a
	// built-in check has its name): it is the adopter's code, so it still
	// runs instead of the built-in check, with a deprecation notice. In a
	// rebar source checkout nothing is legacy.
	Legacy bool
}

// resolveScript finds the project's own scripts/<name> (name with or
// without .sh), unless it is a thin wrapper, which hands off to the CLI
// and would loop.
func resolveScript(projectRoot, name string) (resolvedScript, bool) {
	if !strings.HasSuffix(name, ".sh") {
		name += ".sh"
	}
	local := filepath.Join(projectRoot, "scripts", name)
	if !isFile(local) || isThinWrapper(local) {
		return resolvedScript{}, false
	}
	_, builtIn := checks.Lookup(name)
	return resolvedScript{Path: local, Legacy: builtIn && !config.IsRebarTree(projectRoot)}, true
}

// runRebarScript runs what name stands for in the current project, in the
// project root, and returns its exit code; found is false when there is
// nothing of that name. In order: the project's own copy of one of rebar's
// scripts (Legacy, with the deprecation notice), the built-in check, an
// extra check the project lists in .rebarrc checks_extra (or
// checks_extra_pre_commit), found by its name and run as the suites run
// it, and any other script in the project's scripts/.
func runRebarScript(name string, args ...string) (exitCode int, found bool, err error) {
	s, ok := resolveScript(cfg.RepoRoot, name)
	if c, builtIn := checks.Lookup(name); builtIn && !s.Legacy {
		return runNativeCheck(c, cfg.RepoRoot, cfg.RepoRoot, args), true, nil
	}
	if e, extra := suite.FindExtra(cfg.RepoRoot, name); extra && !s.Legacy {
		if !e.Runnable(cfg.RepoRoot) {
			return 0, false, fmt.Errorf("extra check %s (.rebarrc checks_extra) is not an executable file", e.Path)
		}
		ctx := &checks.Context{Root: cfg.RepoRoot, Dir: cfg.RepoRoot, Stdout: os.Stdout, Stderr: os.Stderr}
		return e.Run(ctx, args), true, nil
	}
	if !ok {
		return 0, false, nil
	}
	switch decideLegacySuite(os.Stderr, s) {
	case runBuiltIn:
		c, _ := checks.Lookup(name)
		return runNativeCheck(c, cfg.RepoRoot, cfg.RepoRoot, args), true, nil
	case refuseCopy:
		return 1, true, nil
	}
	warnLegacy(s)
	code, err := scripts.RunInProject(s.Path, cfg.RepoRoot, args...)
	return code, true, err
}

func warnLegacy(s resolvedScript) {
	if s.Legacy {
		fmt.Fprintf(os.Stderr, "rebar: running the project's own %s (deprecated: rebar's checks are built in; run `rebar migrate` to move its customizations to .rebarrc and replace it)\n",
			filepath.Join("scripts", filepath.Base(s.Path)))
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
