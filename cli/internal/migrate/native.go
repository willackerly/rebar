package migrate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/willackerly/rebar/cli/internal/assets"
	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"

	// The steward, suites and utilities register their native checks.
	_ "github.com/willackerly/rebar/cli/internal/steward"
	_ "github.com/willackerly/rebar/cli/internal/suite"
	_ "github.com/willackerly/rebar/cli/internal/utils"
)

// runNative runs rebar's built-in implementation of the script name
// (check-todos.sh) in process on root, as the script would run there: env
// is the whole environment it sees and rcFile the settings it reads (""
// for root/.rebarrc), through config.WithEnv, so neither REBAR_RCFILE nor
// the caller's environment is involved. Runs are serialized.
func runNative(root, rcFile string, env []string, name string, args ...string) (r runResult) {
	c, ok := checks.Lookup(name)
	if !ok {
		return runResult{Code: -1, Err: fmt.Errorf("rebar has no built-in %s", strings.TrimSuffix(name, ".sh"))}
	}
	var out, errb bytes.Buffer
	defer func() {
		if p := recover(); p != nil {
			r = runResult{Stdout: out.String(), Stderr: errb.String(), Code: -1, Err: fmt.Errorf("%s panicked: %v", name, p)}
		}
	}()
	config.WithEnv(&config.Env{Vars: env, RCFile: rcFile}, func() {
		r.Code = c.Run(&checks.Context{Root: root, Dir: root, Stdout: &out, Stderr: &errb}, args)
	})
	r.Stdout, r.Stderr = out.String(), errb.String()
	return r
}

// isRebarScript reports whether a project's scripts/<name> is a copy of a
// script rebar shipped: one with a built-in implementation, the shared
// _rebar-config.sh, or any other script a release carried
// (released-blobs.txt; rebar ships no scripts since it became a single
// binary).
func isRebarScript(name string) bool {
	if _, ok := checks.Lookup(name); ok && strings.HasSuffix(name, ".sh") {
		return true
	}
	if name == "_rebar-config.sh" {
		return true
	}
	return strings.HasSuffix(name, ".sh") && manifestBlobs()[name] != nil
}

// templateScript is templates/project-bootstrap/scripts/<name> as compiled
// into rebar (nil when the template has no such file).
func templateScript(name string) []byte {
	data, err := assets.ReadFile(assets.BootstrapDir + "/scripts/" + name)
	if err != nil {
		return nil
	}
	return data
}

// nativeFilters are the filters rebar's source scanners apply, written as
// their scripts wrote them (grep -v patterns, --exclude / --exclude-dir
// arguments): exclusionBait compares a legacy copy's filters with these to
// find the ones it has that rebar lacks. TestNativeFiltersMatchScripts
// holds them to the scripts while those exist.
var nativeFilters = map[string]string{
	"check-todos.sh": `EXCLUDES=(--exclude=TODO.md --exclude=check-todos.sh)
  EXCLUDES+=("--exclude-dir=$dir")
  | grep -v "TRACKED-TASK:" \
`,
	"check-contract-refs.sh":    `  INCLUDES+=("--exclude-dir=$dir")` + "\n",
	"check-contract-headers.sh": "",
	"check-freshness.sh":        "",
}

// filterSet is the filters exclusionBait reads from a script's text.
func filterSet(src string) []string {
	var out []string
	for _, mm := range grepVRe.FindAllStringSubmatch(src, -1) {
		out = append(out, "grep -v "+grepVPattern(mm))
	}
	for _, mm := range excludeArgs.FindAllStringSubmatch(src, -1) {
		out = append(out, "--exclude "+mm[1])
	}
	return out
}
