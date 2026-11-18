package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// legacyFixture is rebar's scripts/<name> as released in v3.1.1 (kept in
// migrate's testdata/legacy).
func legacyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "internal", "migrate", "testdata", "legacy", name+".fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// legacySuiteProject is a tier 1 project with an untracked TODO, holding
// the given scripts (name → body; a leading "-x " makes the file not
// executable), committed.
func legacySuiteProject(t *testing.T, scripts map[string]string) string {
	t.Helper()
	root := writeTree(t, map[string]string{
		".rebarrc": "tier = 1\n",
		"src/a.go": "package a\n// @todo untracked\n",
	}, nil)
	for name, body := range scripts {
		if rest, ok := strings.CutPrefix(body, "-x "); ok {
			testutil.WriteFile(t, root, "scripts/"+name, rest)
		} else {
			testutil.WriteExecutable(t, root, "scripts/"+name, body)
		}
	}
	commits("feat: initial commit")(t, root)
	return root
}

// TestLegacySuiteWithMissingChecks (review S8-1): a project that kept its
// copy of a suite script but deleted the copies of the checks it runs
// used to pass without running them (every step "SKIP (script not found)",
// or pre-commit's "Cannot find check scripts", exit 0). An unmodified
// rebar copy now gives way to the built-in suite, which runs the project's
// remaining copies (with bash when not executable) and the built-in checks
// for the rest; a customized copy is refused.
func TestLegacySuiteWithMissingChecks(t *testing.T) {
	requireBash(t)
	bin, home := testBinary(t), t.TempDir()
	env := append(cliEnv(home), "SKIP_STEWARD=1")
	ciCheck, preCommit, config := legacyFixture(t, "ci-check.sh"), legacyFixture(t, "pre-commit.sh"), legacyFixture(t, "_rebar-config.sh")

	t.Run("unmodified ci-check.sh", func(t *testing.T) {
		root := legacySuiteProject(t, map[string]string{"ci-check.sh": ciCheck, "_rebar-config.sh": config})
		// The copy by itself: the silent pass.
		out, code := testutil.RunEnv(t, root, env, "bash", "scripts/ci-check.sh")
		if code != 0 || !strings.Contains(out, "SKIP  TODO Tracking (script not found or not executable)") {
			t.Fatalf("the v3.1.1 copy no longer reproduces the silent pass (exit %d):\n%s", code, out)
		}
		out, code = testutil.RunEnv(t, root, env, bin, "check")
		for _, want := range []string{
			"scripts/ci-check.sh is an unmodified rebar copy, but it would skip scripts/check-bypass-flags.sh,",
			"running the built-in ci-check instead", "Run `rebar migrate`",
			"FAIL  TODO Tracking", "PASS  Fix Commit Gate",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("rebar check (exit %d) lacks %q:\n%s", code, want, out)
			}
		}
		if code != 1 || strings.Contains(out, "script not found") {
			t.Errorf("rebar check: exit %d, want 1 with no step skipped as not found:\n%s", code, out)
		}
		if out, code := testutil.RunEnv(t, root, env, bin, "run", "ci-check"); code != 1 || !strings.Contains(out, "FAIL  TODO Tracking") {
			t.Errorf("rebar run ci-check: exit %d\n%s", code, out)
		}
	})

	t.Run("unmodified pre-commit.sh", func(t *testing.T) {
		root := legacySuiteProject(t, map[string]string{"pre-commit.sh": preCommit, "_rebar-config.sh": config})
		out, code := testutil.RunEnv(t, root, env, "bash", "scripts/pre-commit.sh")
		if code != 0 || !strings.Contains(out, "Cannot find check scripts. Skipping") {
			t.Fatalf("the v3.1.1 copy no longer reproduces the silent pass (exit %d):\n%s", code, out)
		}
		out, code = testutil.RunEnv(t, root, env, bin, "check", "--pre-commit")
		if code != 1 || !strings.Contains(out, "running the built-in pre-commit instead") || !strings.Contains(out, "FAIL: 1 untracked TODO(s) found") {
			t.Errorf("rebar check --pre-commit: exit %d\n%s", code, out)
		}
	})

	t.Run("remaining copies run, a non-executable one with bash", func(t *testing.T) {
		root := legacySuiteProject(t, map[string]string{
			"pre-commit.sh":          preCommit,
			"check-todos.sh":         "-x " + legacyFixture(t, "check-todos.sh"),
			"check-contract-refs.sh": "#!/usr/bin/env bash\necho 'CUSTOM refs'\n",
		})
		out, code := testutil.RunEnv(t, root, env, bin, "check", "--pre-commit")
		for _, want := range []string{
			"it would skip scripts/check-ground-truth.sh, scripts/check-todos.sh (missing or not executable)",
			"running project copy scripts/check-todos.sh", "FAIL: 1 untracked TODO(s) found", "CUSTOM refs",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("rebar check --pre-commit (exit %d) lacks %q:\n%s", code, want, out)
			}
		}
		if code != 1 {
			t.Errorf("rebar check --pre-commit: exit %d, want 1", code)
		}
	})

	t.Run("a step its SKIP variable turns off is not missing", func(t *testing.T) {
		root := legacySuiteProject(t, map[string]string{"ci-check.sh": ciCheck, "_rebar-config.sh": config})
		out, code := testutil.RunEnv(t, root, append(cliEnv(home), suiteSkipAll...), bin, "check")
		if code != 0 || strings.Contains(out, "unmodified rebar copy") || !strings.Contains(out, "running the project's own scripts/ci-check.sh") {
			t.Errorf("rebar check with every step skipped should run the copy (exit %d):\n%s", code, out)
		}
	})

	t.Run("customized ci-check.sh is refused", func(t *testing.T) {
		custom := strings.Replace(ciCheck, `run_check "Steward"`, "run_check \"Lint\"              SKIP_LINT             lint.sh\nrun_check \"Steward\"", 1)
		root := legacySuiteProject(t, map[string]string{"ci-check.sh": custom, "_rebar-config.sh": config, "lint.sh": "#!/usr/bin/env bash\necho lint\n"})
		out, code := testutil.RunEnv(t, root, env, bin, "check")
		for _, want := range []string{
			"scripts/ci-check.sh runs scripts/check-bypass-flags.sh,", "so it does not run",
			"`rebar run check-bypass-flags`", "run `rebar migrate`",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("rebar check (exit %d) lacks %q:\n%s", code, want, out)
			}
		}
		if code != 1 || strings.Contains(out, "Running contract and documentation checks") {
			t.Errorf("rebar check ran the customized copy, or passed (exit %d):\n%s", code, out)
		}
	})
}

// TestBrokenHooksWarn (review S8-2): git skips a hook whose symlink target
// is gone without a word, so a commit-msg hook linked to a deleted
// check-fix-commit.sh copy stopped enforcing Gate G silently. `rebar check`
// names such hooks and the one-liner that replaces them, hook files that
// call a deleted check, and hooks that run a suite copy which would skip
// rebar's checks.
func TestBrokenHooksWarn(t *testing.T) {
	requireBash(t)
	bin, home := testBinary(t), t.TempDir()
	root := legacySuiteProject(t, map[string]string{
		"pre-commit.sh":    legacyFixture(t, "pre-commit.sh"),
		"_rebar-config.sh": legacyFixture(t, "_rebar-config.sh"),
	})
	hooks := filepath.Join(root, ".git", "hooks")
	for hook, target := range map[string]string{"commit-msg": "../../scripts/check-fix-commit.sh", "pre-commit": "../../scripts/pre-commit.sh"} {
		if err := os.Symlink(target, filepath.Join(hooks, hook)); err != nil {
			t.Skip("symlinks unsupported:", err)
		}
	}
	testutil.WriteExecutable(t, root, ".git/hooks/post-commit", "#!/bin/sh\n# runs scripts/check-todos.sh too (a comment)\n./scripts/check-bypass-flags.sh --warn\n")

	out, _ := testutil.RunEnv(t, root, cliEnv(home), bin, "check", "--pre-commit")
	for _, want := range []string{
		"rebar: git hook .git/hooks/commit-msg links to ../../scripts/check-fix-commit.sh, which does not exist, so git skips the hook without a word; " +
			`replace it: printf '#!/bin/sh\nexec rebar run check-fix-commit "$@"\n' > .git/hooks/commit-msg && chmod +x .git/hooks/commit-msg`,
		"rebar: git hook .git/hooks/post-commit calls scripts/check-bypass-flags.sh, which does not exist, so the hook may skip it and pass; have it run `rebar run check-bypass-flags` instead",
		"rebar: git hook .git/hooks/pre-commit runs the project's own scripts/pre-commit.sh, which skips scripts/check-contract-refs.sh, scripts/check-ground-truth.sh, scripts/check-todos.sh (missing or not executable) and passes; run `rebar migrate`, or point the hook at `rebar check --pre-commit`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rebar check --pre-commit lacks\n%s\nin:\n%s", want, out)
		}
	}
	if strings.Contains(out, "check-todos.sh, which does not exist") {
		t.Errorf("a comment in a hook counted as a call:\n%s", out)
	}

	// The one-liner works: the hook enforces Gate G again.
	testutil.WriteExecutable(t, root, ".git/hooks/commit-msg-new", "#!/bin/sh\nexec rebar run check-fix-commit \"$@\"\n")
	_ = os.Remove(filepath.Join(hooks, "commit-msg"))
	if err := os.Rename(filepath.Join(hooks, "commit-msg-new"), filepath.Join(hooks, "commit-msg")); err != nil {
		t.Fatal(err)
	}
	pathDir := t.TempDir()
	if err := os.Symlink(bin, filepath.Join(pathDir, "rebar")); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, root, "msg.txt", "fix: the widget\n")
	env := append(cliEnv(home), "PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, code := testutil.RunEnv(t, root, env, filepath.Join(hooks, "commit-msg"), "msg.txt"); code == 0 || !strings.Contains(out, "Reproduced on:") {
		t.Errorf("the replacement hook let a fix: commit without Reproduced on: through (exit %d):\n%s", code, out)
	}
}
