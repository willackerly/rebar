package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// TestMain lets the test binary stand in for the rebar CLI: with
// REBAR_TEST_RUN_CLI=1 it runs its command line instead of the tests, so a
// script can hand off to it (REBAR_BIN) exactly as to a built rebar.
func TestMain(m *testing.M) {
	if os.Getenv("REBAR_TEST_RUN_CLI") == "1" {
		if err := Execute(); err != nil { // Execute prints the error, as in main
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func testBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
}

// physical resolves symlinks in dir, as `pwd -P` does.
func physical(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// handOffEnv is the environment under which a script of the install whose
// scripts are in scriptsDir hands check-todos off to this test binary, as
// `REBAR_CHECK_IMPL=go rebar check` sets it up.
func handOffEnv(t *testing.T, scriptsDir string) []string {
	return []string{
		"REBAR_BIN=" + testBinary(t),
		"REBAR_NATIVE_CHECKS=" + strings.Join(checks.Names(), " "),
		"REBAR_NATIVE_SCRIPTS=" + physical(t, scriptsDir),
		"REBAR_TEST_RUN_CLI=1",
	}
}

// grepFlavour is "gnu" or "bsd" for the grep the scripts run here.
func grepFlavour(t *testing.T) string {
	t.Helper()
	out, _ := testutil.RunEnv(t, t.TempDir(), testutil.IsolatedEnv(), "bash", "-c", "grep --version 2>&1")
	switch {
	case strings.Contains(out, "GNU grep"):
		return "gnu"
	case strings.Contains(out, "BSD grep"):
		return "bsd"
	}
	return "other"
}

// findingLine is a grep -n match line (path:line:text) in a check's output.
var findingLine = regexp.MustCompile(`^([^ :]+):[0-9]+:`)

// inWalkOrder reorders the finding lines of a script's output into the
// order the native checks list them, keeping every other line in place:
// operands in the order given, files below each in lexical order of their
// path components (a directory's files before a sibling file whose name
// extends the directory's), lines of a file in file order. grep -r lists
// files in directory order, which differs between filesystems; nothing
// else is normalized, so the native output must match it byte for byte.
func inWalkOrder(out string, operands []string) string {
	lines := strings.Split(out, "\n")
	type finding struct {
		line string
		op   int
		path []string
	}
	var slots []int
	var found []finding
	for i, l := range lines {
		m := findingLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		f := finding{line: l, op: len(operands)}
		for j, op := range operands {
			op = strings.TrimRight(op, "/")
			if m[1] == op || strings.HasPrefix(m[1], op+"/") {
				f.op, f.path = j, strings.Split(strings.TrimPrefix(m[1], op), "/")
				break
			}
		}
		slots = append(slots, i)
		found = append(found, f)
	}
	sort.SliceStable(found, func(a, b int) bool {
		fa, fb := found[a], found[b]
		if fa.op != fb.op {
			return fa.op < fb.op
		}
		for k := 0; k < len(fa.path) && k < len(fb.path); k++ {
			if fa.path[k] != fb.path[k] {
				return fa.path[k] < fb.path[k]
			}
		}
		return len(fa.path) < len(fb.path)
	})
	for k, i := range slots {
		lines[i] = found[k].line
	}
	return strings.Join(lines, "\n")
}

// filler is n bytes or more of text lines.
func filler(n int) string {
	return strings.Repeat("filler text\n", n/12+1)
}

// todosParityCase is a fixture tree for check-todos. The script and the
// native check must give the same output and exit code on it, except on
// the grep flavour named by divergesOn, where the script's behaviour is
// platform-dependent and the native check picks the other platform's (want
// is then the native output, checked everywhere). Bodies spell the tags
// @todo / @tracked (testutil.Tags); "$ROOT" in env is the tree's root.
type todosParityCase struct {
	name       string
	files      map[string]string
	links      map[string]string // link → target, relative to the link
	args       []string
	env        []string
	divergesOn string
	want       string
}

var todosParityCases = []todosParityCase{
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
	},
}

func writeTree(t *testing.T, files, links map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		testutil.WriteFile(t, root, rel, testutil.Tags.Replace(body))
	}
	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Skip("symlinks unsupported:", err)
		}
	}
	return root
}

// TestTodosParity runs scripts/check-todos.sh, the native check in process,
// and the script handing off to this binary (the REBAR_CHECK_IMPL=go path)
// on the same trees, and requires the same exit code and output, finding
// lines aside put in the native checks' order.
func TestTodosParity(t *testing.T) {
	requireBash(t)
	scriptsDir := filepath.Join(testutil.RebarSource(t), "scripts")
	script := filepath.Join(scriptsDir, "check-todos.sh")
	c, ok := checks.Lookup("check-todos")
	if !ok {
		t.Fatal("check-todos is not registered")
	}
	flavour := grepFlavour(t)
	for _, tc := range todosParityCases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, tc.files, tc.links)
			var env []string
			for _, kv := range tc.env {
				kv = strings.ReplaceAll(kv, "$ROOT", root)
				env = append(env, kv)
				k, v, _ := strings.Cut(kv, "=")
				if strings.HasPrefix(k, "REBAR_") {
					t.Setenv(k, v) // for the in-process run
				}
			}
			args := append([]string{script}, tc.args...)

			shellOut, shellCode := testutil.RunEnv(t, root, testutil.IsolatedEnv(append([]string{"REBAR_PROJECT_ROOT=" + root}, env...)...), "bash", args...)

			var native strings.Builder
			nativeCode := c.Run(&checks.Context{Root: root, Stdout: &native, Stderr: &native}, tc.args)

			handEnv := append(append([]string{"REBAR_PROJECT_ROOT=" + root}, handOffEnv(t, scriptsDir)...), env...)
			handOut, handCode := testutil.RunEnv(t, root, testutil.IsolatedEnv(handEnv...), "bash", args...)

			operands := tc.args
			if len(operands) == 0 {
				for _, d := range sourcescan.ScopeFor(root).Dirs {
					if info, err := os.Stat(filepath.Join(root, d)); err == nil && info.IsDir() {
						operands = append(operands, d)
					}
				}
			}
			want, wantCode, from := inWalkOrder(shellOut, operands), shellCode, "the script"
			if tc.want != "" {
				want, wantCode, from = testutil.Tags.Replace(tc.want), 1, "want"
				if strings.HasPrefix(want, "OK:") {
					wantCode = 0
				}
				if flavour != tc.divergesOn && (shellCode != wantCode || inWalkOrder(shellOut, operands) != want) {
					t.Errorf("on %s grep the script should give want:\nscript (exit %d):\n%s\nwant (exit %d):\n%s", flavour, shellCode, shellOut, wantCode, want)
				}
				if flavour == tc.divergesOn {
					t.Logf("%s grep diverges here, as documented; the script gave (exit %d):\n%s", flavour, shellCode, shellOut)
				}
			}
			for _, got := range []struct {
				how  string
				out  string
				code int
			}{
				{"native", native.String(), nativeCode},
				{"hand-off", handOut, handCode},
			} {
				if got.code != wantCode || got.out != want {
					t.Errorf("%s differs from %s:\n%s (exit %d):\n%s\n%s (exit %d):\n%s",
						got.how, from, from, wantCode, want, got.how, got.code, got.out)
				}
			}
			if !strings.Contains(shellOut, "TODO") && !strings.Contains(shellOut, "No source directories") {
				t.Errorf("fixture exercised nothing: %s", shellOut)
			}
		})
	}
}

// TestShellRegressionTestsAgainstNative runs every script regression test
// that exercises a ported check with REBAR_TEST_NATIVE_BIN, so its
// assertions hold against the native checks: lib.sh's run_in hands the
// ported checks off to this binary, and REBAR_CHECK_IMPL=go reaches the
// tests that drive the CLI. REBAR_TEST_NATIVE_BIN is a wrapper that logs
// each hand-off, so a test that calls run_in must show one.
func TestShellRegressionTestsAgainstNative(t *testing.T) {
	requireBash(t)
	src := testutil.RebarSource(t)
	tests, err := filepath.Glob(filepath.Join(src, "scripts", "tests", "*.test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, test := range tests {
		data, err := os.ReadFile(test)
		if err != nil {
			t.Fatal(err)
		}
		var ported []string
		for _, name := range checks.Names() {
			if strings.Contains(string(data), name) {
				ported = append(ported, name)
			}
		}
		if len(ported) == 0 {
			continue
		}
		ran++
		usesRunIn := strings.Contains(string(data), "run_in ")
		t.Run(filepath.Base(test), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			log := filepath.Join(dir, "hand-offs.log")
			wrapper := testutil.WriteExecutable(t, dir, "rebar",
				"#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+log+"'\nexec '"+testBinary(t)+"' \"$@\"\n")
			out, code := testutil.RunEnv(t, src, testutil.IsolatedEnv("REBAR_TEST_NATIVE_BIN="+wrapper, "REBAR_TEST_RUN_CLI=1"), "bash", test)
			if code != 0 {
				t.Fatalf("%s against the native checks (exit %d):\n%s", filepath.Base(test), code, out)
			}
			if !usesRunIn {
				return
			}
			logged, _ := os.ReadFile(log)
			for _, name := range ported {
				if !strings.Contains(string(logged), "native "+name) {
					t.Errorf("%s never handed %s off to the native check; hand-offs:\n%s\noutput:\n%s", filepath.Base(test), name, logged, out)
				}
			}
		})
	}
	if ran == 0 {
		t.Fatal("no script test exercises a ported check")
	}
}

func TestNativeFor(t *testing.T) {
	install := resolvedScript{Path: "/install/scripts/check-todos.sh"}
	legacy := resolvedScript{Path: "/project/scripts/check-todos.sh", Legacy: true}
	cases := []struct {
		impl, name string
		s          resolvedScript
		found      bool
		want       bool
	}{
		{"", "check-todos", install, true, false},
		{"go", "check-todos", install, true, true},
		{"go", "check-todos.sh", resolvedScript{}, false, true}, // binary-only install
		{"go", "check-todos", legacy, true, false},              // a legacy copy wins
		{"go", "steward", install, true, false},                 // not ported
	}
	for _, tc := range cases {
		t.Setenv("REBAR_CHECK_IMPL", tc.impl)
		if _, got := nativeFor(tc.name, tc.s, tc.found); got != tc.want {
			t.Errorf("impl=%q %s %+v found=%v: native = %v, want %v", tc.impl, tc.name, tc.s, tc.found, got, tc.want)
		}
	}
}

// cliEnv is the environment for running this binary as the rebar CLI.
func cliEnv(home, impl, rebarRoot string) []string {
	return testutil.IsolatedEnv("REBAR_TEST_RUN_CLI=1", "REBAR_CHECK_IMPL="+impl, "REBAR_ROOT="+rebarRoot, "HOME="+home)
}

// TestCheckImplSwitch runs the CLI (this binary) end to end: rebar run
// check-todos and rebar check --pre-commit give the same result under
// REBAR_CHECK_IMPL=shell and go, and under go rebar run check-todos works
// with no rebar install at all.
func TestCheckImplSwitch(t *testing.T) {
	requireBash(t)
	root := writeTree(t, map[string]string{
		".rebarrc":  "tier = 1\n",
		"src/a.go":  "// @todo untracked\n// @tracked tracked\n",
		"cmd/b.go":  "// @todo another\n",
		"lib/c.txt": "// @todo not source\n",
	}, nil)
	bin := testBinary(t)
	home := t.TempDir()
	src := testutil.RebarSource(t)

	for _, args := range [][]string{{"run", "check-todos"}, {"run", "check-todos", "cmd"}, {"check", "--pre-commit"}} {
		shellOut, shellCode := testutil.RunEnv(t, root, cliEnv(home, "shell", src), bin, args...)
		goOut, goCode := testutil.RunEnv(t, root, cliEnv(home, "go", src), bin, args...)
		if shellCode != goCode || shellOut != goOut {
			t.Errorf("rebar %v: shell (exit %d):\n%s\ngo (exit %d):\n%s", args, shellCode, shellOut, goCode, goOut)
		}
		if !strings.Contains(goOut, "untracked TODO(s) found") {
			t.Errorf("rebar %v: check-todos did not run:\n%s", args, goOut)
		}
	}

	// No install: the shell implementation has nothing to run, the native
	// check needs nothing but the binary.
	if out, code := testutil.RunEnv(t, root, cliEnv(home, "shell", ""), bin, "run", "check-todos"); code == 0 || !strings.Contains(out, "no script") {
		t.Errorf("shell without an install: exit %d\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, root, cliEnv(home, "go", ""), bin, "run", "check-todos"); code != 1 || !strings.Contains(out, "FAIL: 2 untracked TODO(s) found") {
		t.Errorf("go without an install: exit %d\n%s", code, out)
	}
}

// TestNativeHandOffOnlyFromInstall: under REBAR_CHECK_IMPL=go the install's
// check-todos.sh hands off to the native check, but a project's own copy,
// even one made from a script that carries the hand-off hook, runs itself
// and keeps its customizations, whether `rebar run` starts it or the
// install's pre-commit.sh reaches it through _rebar_script.
func TestNativeHandOffOnlyFromInstall(t *testing.T) {
	requireBash(t)
	src := testutil.RebarSource(t)
	hook := `_rebar_native check-todos "$@"`

	// An install whose check-todos.sh says so when it runs past the hook.
	install := t.TempDir()
	scripts, err := filepath.Glob(filepath.Join(src, "scripts", "*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range scripts {
		body, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if filepath.Base(s) == "check-todos.sh" {
			if !strings.Contains(text, hook) {
				t.Fatalf("check-todos.sh has no hand-off hook %q", hook)
			}
			text = strings.Replace(text, hook, hook+"\necho 'install script ran'; exit 0", 1)
		}
		testutil.WriteExecutable(t, install, filepath.Join("scripts", filepath.Base(s)), text)
	}
	testutil.WriteFile(t, install, "templates/project-bootstrap/README.md", "marks a rebar install\n")
	home := t.TempDir()
	bin := testBinary(t)

	plain := writeTree(t, map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"}, nil)
	for _, args := range [][]string{{"run", "check-todos"}, {"check", "--pre-commit"}} {
		out, _ := testutil.RunEnv(t, plain, cliEnv(home, "go", install), bin, args...)
		if strings.Contains(out, "install script ran") || !strings.Contains(out, "OK: 0 untracked TODOs. 0 tracked tasks.") {
			t.Errorf("go: rebar %v did not run the native check:\n%s", args, out)
		}
		out, _ = testutil.RunEnv(t, plain, cliEnv(home, "shell", install), bin, args...)
		if !strings.Contains(out, "install script ran") {
			t.Errorf("shell: rebar %v did not run the install's script:\n%s", args, out)
		}
	}

	// A project copy of the hooked script and its _rebar-config.sh, with a
	// customization of its own.
	project := writeTree(t, map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"}, nil)
	for _, name := range []string{"check-todos.sh", "_rebar-config.sh"} {
		body, err := os.ReadFile(filepath.Join(src, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(body), `echo "OK: 0 untracked TODOs.`, `echo "CUSTOM OK: 0 untracked TODOs.`)
		testutil.WriteExecutable(t, project, filepath.Join("scripts", name), text)
	}
	for _, args := range [][]string{{"run", "check-todos"}, {"check", "--pre-commit"}} {
		out, _ := testutil.RunEnv(t, project, cliEnv(home, "go", install), bin, args...)
		if !strings.Contains(out, "CUSTOM OK: 0 untracked TODOs.") {
			t.Errorf("go: rebar %v did not run the project's customized copy:\n%s", args, out)
		}
	}
}

// TestNativeArgsResolveAgainstWorkingDir: like the script, `rebar native`
// resolves its arguments and the default source directories against the
// directory it runs in, and reads settings from the project root.
func TestNativeArgsResolveAgainstWorkingDir(t *testing.T) {
	requireBash(t)
	root := writeTree(t, map[string]string{
		".rebarrc":      "source_extensions = go txt\n",
		"src/a.go":      "// @todo root src\n",
		"sub/src/b.go":  "// @todo sub src\n",
		"sub/lib/c.txt": "// @todo sub lib\n",
	}, nil)
	sub := filepath.Join(root, "sub")
	scriptsDir := filepath.Join(testutil.RebarSource(t), "scripts")
	script := filepath.Join(scriptsDir, "check-todos.sh")
	for _, args := range [][]string{nil, {"src"}, {"lib", "../src"}} {
		shellOut, shellCode := testutil.RunEnv(t, sub, testutil.IsolatedEnv("REBAR_PROJECT_ROOT="+root), "bash", append([]string{script}, args...)...)
		for _, how := range []struct {
			name string
			env  []string
			argv []string
		}{
			{"rebar native", testutil.IsolatedEnv("REBAR_TEST_RUN_CLI=1"), append([]string{testBinary(t), "native", "check-todos"}, args...)},
			{"hand-off", testutil.IsolatedEnv(append([]string{"REBAR_PROJECT_ROOT=" + root}, handOffEnv(t, scriptsDir)...)...), append([]string{"bash", script}, args...)},
		} {
			out, code := testutil.RunEnv(t, sub, how.env, how.argv[0], how.argv[1:]...)
			if code != shellCode || out != shellOut {
				t.Errorf("%s %v in sub/: exit %d\n%s\nscript: exit %d\n%s", how.name, args, code, out, shellCode, shellOut)
			}
		}
		if !strings.Contains(shellOut, "TODO") {
			t.Errorf("%v exercised nothing: %s", args, shellOut)
		}
	}
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
