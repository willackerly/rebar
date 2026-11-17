package migrate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// nativeFilters spell the filters of the scanner scripts the native checks
// were ported from (as rebar 3.2 shipped them), as exclusionBait reads
// them.
func TestNativeFiltersMatchScripts(t *testing.T) {
	scripts := map[string][]string{
		"check-todos.sh":            {"grep -v TRACKED-TASK:", "--exclude TODO.md", "--exclude check-todos.sh)", "--exclude $dir"},
		"check-contract-refs.sh":    {"--exclude $dir"},
		"check-contract-headers.sh": nil,
		"check-freshness.sh":        nil,
	}
	for name, filters := range nativeFilters {
		if got, want := strings.Join(filterSet(filters), "\n"), strings.Join(scripts[name], "\n"); got != want {
			t.Errorf("%s: nativeFilters give\n%s\nthe script had\n%s", name, got, want)
		}
	}
	if len(nativeFilters) != len(scripts) {
		t.Errorf("nativeFilters has %d scanners, the scripts %d", len(nativeFilters), len(scripts))
	}
}

// runNative is isolated from the caller's environment and REBAR_RCFILE:
// only the given variables and rc file count.
func TestRunNativeIsolation(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/a.go", "// "+"TODO"+": untracked\n")
	testutil.WriteFile(t, root, "other/b.go", "// "+"TODO"+": untracked\n")
	rc := testutil.WriteFile(t, t.TempDir(), "rc", "source_dirs = other\n")
	t.Setenv("REBAR_RCFILE", filepath.Join(root, "nope"))
	t.Setenv("REBAR_TIER", "1")
	r := runNative(root, rc, scrubbedEnv("REBAR_PROJECT_ROOT="+root, "REBAR_TIER=3"), "check-todos.sh")
	if r.Err != nil || r.Code == 0 || !strings.Contains(r.Stdout, "other/b.go") || strings.Contains(r.Stdout, "src/a.go") {
		t.Fatalf("runNative with source_dirs = other: %+v", r)
	}
	if r := runNative(root, "", nil, "no-such-check.sh"); r.Err == nil {
		t.Fatal("an unknown check ran")
	}
}

// Without any rebar tree on disk (a binary install), migrate decides as it
// does with the source tree at hand, and apply verifies with the built-in
// checks.
func TestBuildPlanWithoutInstall(t *testing.T) {
	install := installRoot(t)
	t.Setenv("HOME", t.TempDir())
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh", "#!/usr/bin/env bash\nexit 0\n")
	commitAll(t, p)
	with, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	without, err := BuildPlan(opts(p, ""))
	if err != nil {
		t.Fatal(err)
	}
	summary := func(pl *Plan) string {
		var b strings.Builder
		for _, s := range pl.Scripts {
			b.WriteString(s.Name + " " + s.Verdict.String() + ": " + s.Reason + "\n")
		}
		return b.String() + string(pl.NewRC)
	}
	if summary(with) != summary(without) {
		t.Fatalf("with the source tree:\n%s\nwithout:\n%s", summary(with), summary(without))
	}
	if !strings.Contains(render(without), "compared with: rebar's built-in checks   tier: 1") {
		t.Errorf("render:\n%s", render(without))
	}
	res, err := Apply(without, opts(p, ""), ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	for _, c := range res.Checks {
		if c.Regressed() {
			t.Fatalf("%s regressed:\n%s", c.Name, c.Output)
		}
		ran = ran || c.Ran == "rebar"
	}
	if !ran || res.RolledBack {
		t.Fatalf("apply ran no built-in check: %+v", res.Checks)
	}
}
