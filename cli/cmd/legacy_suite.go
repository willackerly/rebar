package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/migrate"
	"github.com/willackerly/rebar/cli/internal/suite"
)

// legacySuites are rebar's suite scripts, which a project may still have
// copies of. A copy runs the project's copies of the checks next to it and
// skips, without failing, any that is missing or not executable.
var legacySuites = map[string]bool{"ci-check.sh": true, "pre-commit.sh": true, "cold-start-checks.sh": true}

var shellScriptNameRe = regexp.MustCompile(`[A-Za-z0-9_-]+\.sh\b`)

// missingSiblings lists the scripts of rebar's checks that the project's
// copy of a suite at path names but that the copy would skip: missing from
// the project's scripts/, or not executable. Comments and echo/printf lines
// do not count, nor a step of the full suite that its SKIP_* variable
// turns off.
func missingSiblings(root, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	suiteName := filepath.Base(path)
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") || fields[0] == "echo" || fields[0] == "printf" {
			continue
		}
		for _, name := range shellScriptNameRe.FindAllString(line, -1) {
			if seen[name] || legacySuites[name] {
				continue
			}
			seen[name] = true
			if _, builtIn := checks.Lookup(name); !builtIn {
				continue
			}
			if suiteName == "ci-check.sh" {
				if v := suite.StepSkipVar(name); v != "" && config.Getenv(v) == "1" {
					continue
				}
			}
			if !isExecutableFile(filepath.Join(root, "scripts", name)) {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func isExecutableFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// legacySuiteDecision is what runs for a project's copy of a suite.
type legacySuiteDecision int

const (
	runCopy    legacySuiteDecision = iota // the copy, as before
	runBuiltIn                            // the built-in suite instead
	refuseCopy                            // nothing: the copy would pass without running checks
)

// decideLegacySuite decides what runs for the project's copy s of a suite
// (ci-check.sh, pre-commit.sh, cold-start-checks.sh), and explains on w
// when it is not the copy. A copy whose sibling checks are all there runs.
// Otherwise the copy would skip rebar's own checks and still pass (a
// project that deleted the check copies but kept the suite): an unmodified
// rebar copy is replaced by the built-in suite, which runs the same checks
// (the project's remaining copies, the built-in check for the rest); a
// customized copy is refused, since running the built-in suite would drop
// its customizations, except the advisory cold-start hook, which still
// runs it after the warning.
func decideLegacySuite(w io.Writer, s resolvedScript) legacySuiteDecision {
	base := filepath.Base(s.Path)
	if !s.Legacy || !legacySuites[base] {
		return runCopy
	}
	missing := missingSiblings(cfg.RepoRoot, s.Path)
	if len(missing) == 0 {
		return runCopy
	}
	list := "scripts/" + strings.Join(missing, ", scripts/")
	native := strings.TrimSuffix(base, ".sh")
	if data, err := os.ReadFile(s.Path); err == nil && migrate.IsReleasedScript(base, data) {
		fmt.Fprintf(w, "rebar: scripts/%s is an unmodified rebar copy, but it would skip %s (missing or not executable); "+
			"running the built-in %s instead, which runs the project's remaining copies and the built-in checks for the rest. "+
			"Run `rebar migrate` to replace the copies.\n", base, list, native)
		return runBuiltIn
	}
	if base == "cold-start-checks.sh" {
		fmt.Fprintf(w, "rebar: scripts/%s skips %s (missing or not executable). Run `rebar migrate`.\n", base, list)
		return runCopy
	}
	first := strings.TrimSuffix(missing[0], ".sh")
	fmt.Fprintf(w, "rebar: scripts/%s runs %s, which is missing or not executable: the copy would skip that check and still pass, so it does not run.\n"+
		"rebar: restore the file (chmod +x), have the copy call `rebar run %s` instead, or run `rebar migrate`, which moves the copy's own steps to .rebarrc checks_extra and replaces it.\n",
		base, list, first)
	return refuseCopy
}
