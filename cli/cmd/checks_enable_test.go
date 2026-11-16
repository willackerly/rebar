package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/scripts"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// checksEnableProject is a tier-1 project with one declared metric whose
// METRICS floor is broken, so ground truth fails whenever it runs.
func checksEnableProject(t *testing.T, extraRC string) string {
	t.Helper()
	project := t.TempDir()
	testutil.WriteFile(t, project, "src/a_test.go", "package a\n")
	testutil.WriteFile(t, project, "METRICS", "files >= 2\n")
	testutil.WriteFile(t, project, "architecture/README.md", "contracts go here\n")
	testutil.WriteFile(t, project, ".rebarrc",
		"tier = 1\nmetric.files.paths = src\nmetric.files.names = *_test.go\n"+extraRC)
	return project
}

// The CLI runs the install's scripts in the project (`rebar run`, `rebar
// check --pre-commit`), and the project's checks_enable reaches them.
func TestChecksEnableReachesInstallScripts(t *testing.T) {
	install, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("REBAR_TIER", "")
	t.Setenv("REBAR_CHECKS_ENABLE", "")
	t.Setenv("_REBAR_CHECKS_VALIDATED", "")

	run := func(project, name string) (string, int) {
		t.Helper()
		s, ok := resolveScript(project, install, name)
		if !ok || s.Legacy {
			t.Fatalf("resolveScript(%s) = %+v, %v", name, s, ok)
		}
		out, code, err := scripts.OutputInProject(s.Path, project)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(out), code
	}

	unset := checksEnableProject(t, "")
	for _, name := range []string{"check-ground-truth", "pre-commit"} {
		if out, code := run(unset, name); code != 0 || strings.Contains(out, "DRIFT") {
			t.Fatalf("tier 1 without checks_enable: %s ran ground truth (exit %d):\n%s", name, code, out)
		}
	}

	enabled := checksEnableProject(t, "checks_enable = check-ground-truth no-such-check\n")
	for _, name := range []string{"check-ground-truth", "pre-commit"} {
		out, code := run(enabled, name)
		if code == 0 || !strings.Contains(out, "DRIFT") {
			t.Fatalf("checks_enable: %s should run ground truth and fail (exit %d):\n%s", name, code, out)
		}
		if n := strings.Count(out, "unknown check 'no-such-check'"); n != 1 {
			t.Fatalf("%s: unknown-name warning printed %d times, want 1:\n%s", name, n, out)
		}
	}
}

func TestAuditTier(t *testing.T) {
	cases := []struct{ rc, want string }{
		{"", "none"},
		{"tier = 1   # see Q1\n", "1"},
		{"tier = 1\nchecks_enable = check-ground-truth compute-registry.sh\n", "1 +check-ground-truth +compute-registry"},
		{"# checks_enable runs these regardless of tier = 3\ntier = 2\n", "2"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		if tc.rc != "" {
			testutil.WriteFile(t, dir, ".rebarrc", tc.rc)
		}
		if got := auditTier(dir); got != tc.want {
			t.Errorf("auditTier(%q) = %q, want %q", tc.rc, got, tc.want)
		}
	}
}
