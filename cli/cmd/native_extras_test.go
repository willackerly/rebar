package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The project's own checks (.rebarrc checks_extra, checks_extra_pre_commit)
// are a native-only feature: goldens in testdata/<suite>-extras hold the
// native suites' output (regenerate with
// go test ./cmd -run Extras -update-golden).

// extraScripts writes the project's extra checks, then commits: one that
// passes and shows where it runs, one that fails, a file that is not
// executable, one with no #! line, and one in another language.
func extraScripts(t *testing.T, root string) {
	t.Helper()
	testutil.WriteExecutable(t, root, "scripts/check-openapi.sh", "#!/usr/bin/env bash\nhere=no; [ \"$(pwd -P)\" = \"$(cd \"$REBAR_PROJECT_ROOT\" && pwd -P)\" ] && here=yes\necho \"openapi ok, in the project root: $here, args:$#\"\n")
	testutil.WriteExecutable(t, root, "tools/check-licenses", "#!/bin/sh\necho 'FAIL: GPL in vendor/x' >&2\nexit 3\n")
	testutil.WriteFile(t, root, "tools/not-executable.sh", "#!/bin/sh\necho never\n")
	testutil.WriteExecutable(t, root, "tools/plain.sh", "echo 'plain ran under bash'\n")
	testutil.WriteExecutable(t, root, "tools/lint.awk", "#!/usr/bin/awk -f\nBEGIN { print \"awk lint ok\" }\n")
	withRegistry(t, root)
}

// extrasProject is suiteProject with every kind of extra check listed.
var extrasProject = withFiles(suiteProject, map[string]string{
	".rebarrc": "tier = 1\n" +
		"checks_extra = scripts/check-openapi.sh tools/check-licenses tools/not-executable.sh tools/missing.sh tools/plain.sh tools/lint.awk # mine\n" +
		"checks_extra_pre_commit = scripts/check-openapi.sh tools/lint.awk\n",
})

var ciExtrasCases = []parityCase{
	{name: "every kind of extra", files: extrasProject, git: true, setup: extraScripts, env: noSteward},
	{name: "extras that pass", files: withFiles(extrasProject, map[string]string{".rebarrc": "tier = 1\nchecks_extra = scripts/check-openapi.sh tools/lint.awk\n"}), git: true, setup: extraScripts, env: noSteward},
	{name: "a failing extra with --no-strict", files: extrasProject, git: true, setup: extraScripts, env: noSteward, args: []string{"--no-strict"}},
	{name: "SKIP_CHECKS_EXTRA", files: extrasProject, git: true, setup: extraScripts, env: []string{"SKIP_STEWARD=1", "SKIP_CHECKS_EXTRA=1"}},
	{name: "REBAR_CHECKS_EXTRA overrides .rebarrc", files: extrasProject, git: true, setup: extraScripts, env: []string{"SKIP_STEWARD=1", "REBAR_CHECKS_EXTRA=tools/plain.sh"}},
	{name: "an absolute path", files: suiteProject, git: true, setup: func(t *testing.T, root string) {
		extraScripts(t, root)
		testutil.WriteFile(t, root, ".rebarrc", "tier = 1\nchecks_extra = "+filepath.Join(root, "tools/plain.sh")+"\n")
	}, env: noSteward},
}

func TestCICheckExtras(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1")
	t.Setenv("REBAR_CHECKS_EXTRA", "")
	runGolden(t, parityCheck{name: "ci-check", golden: "ci-check-extras", nativeOnly: true}, ciExtrasCases)
}

var preCommitExtrasCases = []parityCase{
	{name: "extras that pass", files: extrasProject, git: true, setup: extraScripts},
	{name: "a failing and a missing extra", files: withFiles(extrasProject, map[string]string{".rebarrc": "tier = 1\nchecks_extra_pre_commit = tools/check-licenses tools/missing.sh tools/lint.awk\n"}), git: true, setup: extraScripts},
	{name: "SKIP_CHECKS_EXTRA", files: withFiles(extrasProject, map[string]string{".rebarrc": "tier = 1\nchecks_extra_pre_commit = tools/check-licenses\n"}), git: true, setup: extraScripts, env: []string{"SKIP_CHECKS_EXTRA=1"}},
	{name: "checks_extra alone does not run here", files: withFiles(extrasProject, map[string]string{".rebarrc": "tier = 1\nchecks_extra = tools/check-licenses\n"}), git: true, setup: extraScripts},
}

func TestPreCommitExtras(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1")
	t.Setenv("REBAR_CHECKS_EXTRA_PRE_COMMIT", "")
	runGolden(t, parityCheck{name: "pre-commit", golden: "pre-commit-extras", nativeOnly: true}, preCommitExtrasCases)
}

// TestExtrasCLI runs the CLI (this binary): rebar check counts the extras
// in the summary; rebar run finds an extra by name and passes its exit code
// and arguments through; REBAR_CHECK_IMPL=shell and a project's own
// ci-check.sh say they do not run them.
func TestExtrasCLI(t *testing.T) {
	requireBash(t)
	root := writeTree(t, extrasProject, nil)
	testutil.Git(t, root, "init", "-q", "-b", "main")
	extraScripts(t, root)
	bin, home, src := testBinary(t), t.TempDir(), testutil.RebarSource(t)
	env := func(impl string) []string { return append(cliEnv(home, impl, src), "SKIP_STEWARD=1") }

	out, code := testutil.RunEnv(t, root, env(""), bin, "check")
	if code != 1 || !strings.Contains(out, "PASS  extra: check-openapi") || !strings.Contains(out, "FAIL  extra: check-licenses") ||
		!strings.Contains(out, "FAIL  extra: missing (tools/missing.sh not found or not executable)") {
		t.Errorf("rebar check: exit %d\n%s", code, out)
	}
	out, code = testutil.RunEnv(t, root, env(""), bin, "run", "check-openapi", "a", "b")
	if code != 0 || !strings.Contains(out, "openapi ok, in the project root: yes, args:2") || strings.Contains(out, "deprecated") {
		t.Errorf("rebar run check-openapi: exit %d\n%s", code, out)
	}
	if out, code = testutil.RunEnv(t, root, env(""), bin, "run", "tools/check-licenses"); code != 3 || !strings.Contains(out, "GPL") {
		t.Errorf("rebar run tools/check-licenses: exit %d\n%s", code, out)
	}
	if out, code = testutil.RunEnv(t, root, env(""), bin, "run", "not-executable"); code == 0 || !strings.Contains(out, "not an executable file") {
		t.Errorf("rebar run not-executable: exit %d\n%s", code, out)
	}
	// rebar's own checks keep their names.
	if out, _ = testutil.RunEnv(t, root, env(""), bin, "run", "check-todos"); !strings.Contains(out, "untracked TODOs") {
		t.Errorf("rebar run check-todos:\n%s", out)
	}

	out, _ = testutil.RunEnv(t, root, env("shell"), bin, "check")
	if !strings.Contains(out, "rebar: .rebarrc checks_extra not run: REBAR_CHECK_IMPL=shell runs the suite script") || strings.Contains(out, "extra: check-openapi") {
		t.Errorf("shell: rebar check\n%s", out)
	}
	out, _ = testutil.RunEnv(t, root, env("shell"), bin, "check", "--pre-commit")
	if !strings.Contains(out, "checks_extra_pre_commit not run") {
		t.Errorf("shell: rebar check --pre-commit\n%s", out)
	}
	if out, _ = testutil.RunEnv(t, root, env(""), bin, "check", "--cold-start"); strings.Contains(out, "not run") {
		t.Errorf("cold start warned about extras:\n%s", out)
	}

	copySrc, err := os.ReadFile(filepath.Join(src, "scripts", "ci-check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteExecutable(t, root, "scripts/ci-check.sh", string(copySrc))
	out, _ = testutil.RunEnv(t, root, env(""), bin, "check")
	if !strings.Contains(out, "checks_extra not run: the project's own scripts/ci-check.sh runs instead") {
		t.Errorf("project copy: rebar check\n%s", out)
	}
}
