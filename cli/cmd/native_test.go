package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

var todosCases = []goldenCase{
	{
		name: "source-scan-excludes: by directory, not by substring",
		files: map[string]string{
			".rebarrc":          "source_dirs = src\nsource_extensions = sh\n",
			"src/distance.sh":   "# @todo one\n",
			"src/words.sh":      "# @todo two — see vendor docs\n",
			"src/vendor/lib.sh": "# @todo skipped\n",
			"src/a.sh":          "# @todo move this into TODO.md\n",
			"src/c.sh":          "# @todo see the check-todos docs\n",
		},
	},
	{
		name:  "no source directories",
		files: map[string]string{"tools/a.go": "// @todo not scanned\n"},
	},
	{
		name: "tracked only",
		files: map[string]string{
			"src/a.go":      "// @tracked later\n",
			"internal/b.py": "# @todo @tracked both on one line\n",
		},
	},
	{
		name: "default scope",
		files: map[string]string{
			"src/a.go":                    "package a\n// @todo first\n\n// @todo second @todo twice\n",
			"src/a/b.go":                  "// @todo a directory sorts before a.go\n",
			"src/nested/deep/b.ts":        "// @todo deep\n",
			"internal/c.rs":               "// @tracked tracked\n// @todo untracked\n",
			"cmd/d.jsx":                   "{/* @todo jsx */}\n",
			"cmd/e.txt":                   "@todo wrong extension\n",
			"lib/node_modules/x/f.js":     "// @todo node_modules\n",
			"app/dist/g.js":               "// @todo dist\n",
			"packages/p/.claude/w/h.go":   "// @todo agent worktree\n",
			"client/.github/i.js":         "// @todo hidden but scanned\n",
			"client/check-todos.sh":       "# @todo the script itself\n",
			"src/distance_vendor_dist.go": "// @todo names are not directories\n",
		},
	},
	{
		name: "line formats",
		files: map[string]string{
			"src/crlf.go":    "one\r\n// @todo crlf\r\nthree\r\n",
			"src/tail.go":    "x\n// @todo no trailing newline",
			"src/escapes.go": "// @todo 100% \\n \\t -e $HOME `x` 'q' \"d\"\n",
			"src/utf8.go":    "// café @todo ünïcode ✓\n",
			"src/only.go":    "@todo",
			"src/empty.go":   "",
			"src/many.go":    strings.Repeat("x\n", 9) + "// @todo line 10 after line 9\n",
		},
	},
	{
		name: ".rebarrc scope with comments, dots, globs",
		files: map[string]string{
			".rebarrc":            "tier = 1\nsource_dirs = . # whole tree\nsource_extensions = .go md\nsource_exclude_dirs = gen* vendor\n",
			"TODO.md":             "- @todo the list itself\n",
			"notes/plan.md":       "@todo in markdown\n",
			"a.go":                "// @todo top level\n",
			"generated/x.go":      "// @todo generated\n",
			"gen2/y.go":           "// @todo gen2\n",
			"vendor/z.go":         "// @todo vendor\n",
			"node_modules/n/x.go": "// @todo no longer excluded\n",
			".claude/w/old.go":    "// @todo always excluded\n",
		},
	},
	{
		name: "operands",
		files: map[string]string{
			"src/a.go":    "// @todo a\n",
			"src/b.txt":   "// @todo b\n",
			"lib/c.go":    "// @todo c\n",
			"vendor/d.go": "// @todo d\n",
		},
		args: []string{"./src/", "lib", "missing", "vendor", "src/b.txt", "lib/c.go"},
	},
	{
		name: "REBAR_RCFILE replaces .rebarrc",
		files: map[string]string{
			".rebarrc":  "source_dirs = src\n",
			"alt.rc":    "source_dirs = lib\nsource_extensions = txt\n",
			"src/a.go":  "// @todo project .rebarrc scope\n",
			"lib/b.go":  "// @todo wrong extension for alt.rc\n",
			"lib/c.txt": "// @todo alt.rc scope\n",
		},
		env: []string{"REBAR_RCFILE=$ROOT/alt.rc"},
	},
	{
		name: "exclude-dir: [!x] negation, POSIX classes, escapes",
		files: map[string]string{
			".rebarrc":          "source_dirs = src\nsource_exclude_dirs = [!x]en [[:digit:]]* \\tmp\n",
			"src/gen/a.go":      "// @todo excluded by [!x]en\n",
			"src/xen/b.go":      "// @todo kept: x is negated\n",
			"src/deep/gen/c.go": "// @todo excluded at any depth\n",
			"src/2024/d.go":     "// @todo excluded by the class\n",
			"src/tmp/e.go":      "// @todo excluded by the quoted name\n",
			"src/keep/f.go":     "// @todo kept\n",
		},
	},
	{
		name: "exclude-dir: operands by path and by name",
		files: map[string]string{
			".rebarrc":      "source_exclude_dirs = src/gen other\n",
			"src/gen/a.go":  "// @todo excluded operand (its path)\n",
			"x/other/b.go":  "// @todo excluded operand (its name)\n",
			"src/keep/c.go": "// @todo kept\n",
		},
		args: []string{"src/gen", "./x/other", "src/keep"},
	},
	{
		name: "exclude-dir: a tail of an operand's path",
		files: map[string]string{
			".rebarrc":      "source_exclude_dirs = src/gen\n",
			"src/gen/a.go":  "// @todo excluded: src/gen ends ./src/gen\n",
			"src/keep/b.go": "// @todo kept\n",
		},
		args: []string{"./src/gen", "src/keep"},
		// BSD grep matches an operand by its whole path or last component
		// only; GNU grep and the native check by any tail after a slash.
		divergesOn: "bsd",
		want: `FAIL: 1 untracked TODO(s) found:

src/keep/b.go:1:// @todo kept

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
	{
		name: "exclude-dir: a pattern with a slash below an operand",
		files: map[string]string{
			".rebarrc":      "source_dirs = src\nsource_exclude_dirs = src/gen\n",
			"src/gen/a.go":  "// @todo matched by name only: not excluded\n",
			"src/keep/b.go": "// @todo kept\n",
		},
		// BSD grep also matches the pattern against the path below the
		// operand and skips src/gen; GNU grep and the native check do not.
		divergesOn: "bsd",
		want: `FAIL: 2 untracked TODO(s) found:

src/gen/a.go:1:// @todo matched by name only: not excluded
src/keep/b.go:1:// @todo kept

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
	{
		name: "exclude-dir: a trailing slash",
		files: map[string]string{
			".rebarrc":      "source_dirs = src\nsource_exclude_dirs = gen/\n",
			"src/gen/a.go":  "// @todo excluded: the slash is dropped\n",
			"src/keep/b.go": "// @todo kept\n",
		},
		// GNU grep drops the slash, as the native check; BSD grep keeps it,
		// and the pattern matches nothing.
		divergesOn: "bsd",
		want: `FAIL: 1 untracked TODO(s) found:

src/keep/b.go:1:// @todo kept

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
	{
		name: "binary: NUL in the first 32 KiB skips the file",
		files: map[string]string{
			"src/bin.go":  "// @todo in a binary file\n\x00\n",
			"src/edge.go": "// @todo NUL in the last byte of the block\n" + strings.Repeat("x", 32<<10-len("// @todo NUL in the last byte of the block\n")-1) + "\x00",
			"src/ok.go":   "// @todo text\n",
		},
	},
	{
		name: "binary: TODO before a NUL 300 KiB in",
		files: map[string]string{
			"src/late.go": "// @todo before a NUL 300 KiB in\n" + filler(300<<10) + "\x00\n",
		},
	},
	{
		name: "binary: TODO before a NUL 70 KiB in",
		files: map[string]string{
			"src/mid.go": "// @todo before a NUL 70 KiB in\n" + filler(70<<10) + "\x00\n",
		},
		// BSD grep looks at the first 32 KiB, as the native check, and
		// reports the TODO; GNU grep's first buffer is larger and holds
		// the NUL, so it skips the file.
		divergesOn: "gnu",
		want: `FAIL: 1 untracked TODO(s) found:

src/mid.go:1:// @todo before a NUL 70 KiB in

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
	{
		name: "binary: TODO after a NUL past the first 32 KiB",
		files: map[string]string{
			"src/after.go": filler(40<<10) + "\x00\n// @todo after the NUL\n",
		},
		// BSD grep decides on the first 32 KiB, as the native check, and
		// reports the TODO; GNU grep stops at the NUL.
		divergesOn: "gnu",
		want: `FAIL: 1 untracked TODO(s) found:

src/after.go:3416:// @todo after the NUL

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
	{
		name: "Latin-1 bytes in the C locale",
		files: map[string]string{
			"src/latin1.go": "// caf\xe9 @todo latin-1\n",
		},
		env: []string{"LC_ALL=C"},
	},
	{
		name: "symlinked source directory",
		files: map[string]string{
			"real/a.go":  "// @todo through the src link\n",
			"other/b.go": "// @todo behind a link met while walking: skipped\n",
		},
		links: map[string]string{"src": "real", "real/inner": "../other"},
		// GNU grep -r follows a symlinked operand, as the native check;
		// BSD grep -r skips it.
		divergesOn: "bsd",
		want: `FAIL: 1 untracked TODO(s) found:

src/a.go:1:// @todo through the src link

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 0 tracked tasks (OK).
`,
		wantExit: 1,
	},
}

// TestTodosGolden: check-todos on every case's tree.
func TestTodosGolden(t *testing.T) {
	runGolden(t, goldenCheck{
		name:   "check-todos",
		golden: "check-todos",
		exercised: func(out string) bool {
			return strings.Contains(out, "TODO") || strings.Contains(out, "No source directories")
		},
	}, todosCases)
}

// TestRunWithoutInstall: rebar run and rebar check need nothing but the
// binary (no rebar tree, REBAR_ROOT, or project scripts).
func TestRunWithoutInstall(t *testing.T) {
	root := writeTree(t, map[string]string{
		".rebarrc":  "tier = 1\n",
		"src/a.go":  "// @todo untracked\n// @tracked tracked\n",
		"cmd/b.go":  "// @todo another\n",
		"lib/c.txt": "// @todo not source\n",
	}, nil)
	bin, home := testBinary(t), t.TempDir()
	if out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "run", "check-todos"); code != 1 || !strings.Contains(out, "FAIL: 2 untracked TODO(s) found") {
		t.Errorf("rebar run check-todos: exit %d\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "run", "check-todos", "cmd"); code != 1 || !strings.Contains(out, "FAIL: 1 untracked TODO(s) found") {
		t.Errorf("rebar run check-todos cmd: exit %d\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "check", "--pre-commit"); code != 1 || !strings.Contains(out, "untracked TODO(s) found") {
		t.Errorf("rebar check --pre-commit: exit %d\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "run", "no-such-check"); code == 0 || !strings.Contains(out, "no check or script") {
		t.Errorf("rebar run no-such-check: exit %d\n%s", code, out)
	}
}

// TestLegacyCopiesRunThemselves: a project's own copy of one of rebar's
// scripts (made before rebar 3.2) is the adopter's code: `rebar run` and the
// suites run it, with the deprecation notice, instead of the built-in
// check, and keep its customizations; a thin wrapper is never run, and
// other project scripts run without the notice.
func TestLegacyCopiesRunThemselves(t *testing.T) {
	requireBash(t)
	bin, home := testBinary(t), t.TempDir()
	plain := writeTree(t, map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"}, nil)
	for _, args := range [][]string{{"run", "check-todos"}, {"check", "--pre-commit"}} {
		out, _ := testutil.RunEnv(t, plain, cliEnv(home), bin, args...)
		if !strings.Contains(out, "OK: 0 untracked TODOs. 0 tracked tasks.") || strings.Contains(out, "deprecated") {
			t.Errorf("rebar %v in a plain project did not run the built-in check:\n%s", args, out)
		}
	}

	project := writeTree(t, map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"}, nil)
	testutil.WriteExecutable(t, project, "scripts/check-todos.sh", "#!/usr/bin/env bash\necho \"CUSTOM OK: 0 untracked TODOs. root=$REBAR_PROJECT_ROOT\"\n")
	testutil.WriteExecutable(t, project, "scripts/check-contract-refs.sh", "#!/usr/bin/env bash\n# rebar-thin-wrapper\necho 'thin wrapper ran'; exit 1\n")
	testutil.WriteExecutable(t, project, "scripts/deploy.sh", "#!/usr/bin/env bash\necho \"deploying $*\"\n")
	out, code := testutil.RunEnv(t, project, cliEnv(home), bin, "run", "check-todos")
	if code != 0 || !strings.Contains(out, "CUSTOM OK: 0 untracked TODOs. root="+physical(t, project)) || !strings.Contains(out, "rebar: running the project's own scripts/check-todos.sh (deprecated") {
		t.Errorf("rebar run check-todos with a project copy: exit %d\n%s", code, out)
	}
	out, _ = testutil.RunEnv(t, project, cliEnv(home), bin, "check", "--pre-commit")
	if !strings.Contains(out, "CUSTOM OK") || !strings.Contains(out, "running project copy scripts/check-todos.sh (deprecated") || strings.Contains(out, "thin wrapper ran") {
		t.Errorf("rebar check --pre-commit with project copies:\n%s", out)
	}
	out, code = testutil.RunEnv(t, project, cliEnv(home), bin, "run", "deploy", "now")
	if code != 0 || strings.TrimSpace(out) != "deploying now" {
		t.Errorf("rebar run deploy: exit %d\n%s", code, out)
	}
	out, _ = testutil.RunEnv(t, project, cliEnv(home), bin, "run")
	for _, want := range []string{"Built into rebar:", "  check-todos\n", "check-todos  (legacy copy: runs instead of the built-in check)", "  deploy\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("rebar run (list) lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "check-contract-refs  (legacy") {
		t.Errorf("rebar run lists a thin wrapper as a legacy copy:\n%s", out)
	}
}

// TestNativeArgsResolveAgainstWorkingDir: like the script, `rebar native`
// resolves its arguments and the default source directories against the
// directory it runs in, and reads settings from the project root.
func TestNativeArgsResolveAgainstWorkingDir(t *testing.T) {
	root := writeTree(t, map[string]string{
		".rebarrc":      "source_extensions = go txt\n",
		"src/a.go":      "// @todo root src\n",
		"sub/src/b.go":  "// @todo sub src\n",
		"sub/lib/c.txt": "// @todo sub lib\n",
	}, nil)
	sub := filepath.Join(root, "sub")
	env := testutil.IsolatedEnv("REBAR_TEST_RUN_CLI=1", "REBAR_PROJECT_ROOT="+root)
	for _, tc := range []struct {
		args      []string
		want, not []string
	}{
		{nil, []string{"src/b.go:1:// @todo sub src", "lib/c.txt:1:// @todo sub lib"}, []string{"root src"}},
		{[]string{"src"}, []string{"src/b.go:1:// @todo sub src"}, []string{"sub lib", "root src"}},
		{[]string{"lib", "../src"}, []string{"lib/c.txt:1:// @todo sub lib", "../src/a.go:1:// @todo root src"}, []string{"sub src"}},
	} {
		out, code := testutil.RunEnv(t, sub, env, testBinary(t), append([]string{"native", "check-todos"}, tc.args...)...)
		if code != 1 {
			t.Errorf("%v in sub/: exit %d\n%s", tc.args, code, out)
		}
		for _, w := range tc.want {
			if w = testutil.Tags.Replace(w); !strings.Contains(out, w) {
				t.Errorf("%v in sub/ lacks %q:\n%s", tc.args, w, out)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(out, n) {
				t.Errorf("%v in sub/ has %q:\n%s", tc.args, n, out)
			}
		}
	}
	_ = os.Getenv
}

// TestNativeRepoRootFlag: root flags before `native` are taken by the CLI
// (hoistRootFlags), so `rebar --repo-root X native <check>` reads X's
// settings (relative paths still resolve against the working directory, as
// for the script), while arguments after the check name reach the check.
func TestNativeRepoRootFlag(t *testing.T) {
	root := writeTree(t, map[string]string{".rebarrc": "source_dirs = only\n"}, nil)
	work := writeTree(t, map[string]string{"only/a.go": "// @todo in only\n", "src/b.go": "// @todo in src\n"}, nil)
	env := testutil.IsolatedEnv("REBAR_TEST_RUN_CLI=1")
	out, code := testutil.RunEnv(t, work, env, testBinary(t), "--repo-root", root, "native", "check-todos")
	if code != 1 || !strings.Contains(out, "only/a.go") || strings.Contains(out, "src/b.go") {
		t.Errorf("--repo-root before native: exit %d\n%s", code, out)
	}
	out, code = testutil.RunEnv(t, work, env, testBinary(t), "-v", "native", "check-todos", "src")
	if code != 1 || !strings.Contains(out, "src/b.go") || strings.Contains(out, "only/a.go") {
		t.Errorf("-v before native, operand after: exit %d\n%s", code, out)
	}
}
