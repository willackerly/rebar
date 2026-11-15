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

// rebarSource is the rebar checkout these tests run in.
func rebarSource(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func testBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// isolatedEnv is the test's environment without rebar settings, so a
// developer's REBAR_TIER or REBAR_CHECK_IMPL cannot leak into a fixture.
func isolatedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "REBAR_") || strings.HasPrefix(kv, "_REBAR_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// run runs name in dir and returns its combined output and exit code.
func run(t *testing.T, dir string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out), 0
}

// findingLine is a grep -n match line (path:line:text) in a check's output.
var findingLine = regexp.MustCompile(`^[^ :]+:[0-9]+:`)

// normalizeFindings sorts the finding lines among the other lines, which
// keep their place. grep -r lists files in directory order, which differs
// between filesystems; the native checks list them in lexical order.
func normalizeFindings(out string) string {
	lines := strings.Split(out, "\n")
	var idx []int
	var found []string
	for i, l := range lines {
		if findingLine.MatchString(l) {
			idx = append(idx, i)
			found = append(found, l)
		}
	}
	sort.Strings(found)
	for i, j := range idx {
		lines[j] = found[i]
	}
	return strings.Join(lines, "\n")
}

// todosParityCases are fixture trees for check-todos: the cases of
// scripts/tests/source-scan-excludes.test.sh plus the scan-scope and
// line-format edges. Each must give the same output and exit code from the
// script and the native check. Cases where grep implementations disagree
// with each other (binary files, invalid UTF-8, symlinked operands) are
// covered by the sourcescan unit tests instead.
var todosParityCases = []struct {
	name  string
	files map[string]string
	args  []string
}{
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
}

// fixtureMarkers spell the tags in fixture bodies so that rebar's own
// check-todos, pointed at this file, does not count them.
var fixtureMarkers = strings.NewReplacer("@todo", "TO"+"DO:", "@tracked", "TRACKED"+"-TASK:")

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		testutil.WriteFile(t, root, rel, fixtureMarkers.Replace(body))
	}
	return root
}

// TestTodosParity runs scripts/check-todos.sh, the native check in process,
// and the script handing off to this binary (the REBAR_CHECK_IMPL=go path)
// on the same trees, and requires the same output and exit code.
func TestTodosParity(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	script := filepath.Join(rebarSource(t), "scripts", "check-todos.sh")
	c, ok := checks.Lookup("check-todos")
	if !ok {
		t.Fatal("check-todos is not registered")
	}
	for _, tc := range todosParityCases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, tc.files)
			args := append([]string{script}, tc.args...)

			shellOut, shellCode := run(t, root, isolatedEnv("REBAR_PROJECT_ROOT="+root), "bash", args...)

			var native strings.Builder
			nativeCode := c.Run(&checks.Context{Root: root, Stdout: &native, Stderr: &native}, tc.args)

			handOut, handCode := run(t, root, isolatedEnv(
				"REBAR_PROJECT_ROOT="+root,
				"REBAR_BIN="+testBinary(t),
				"REBAR_NATIVE_CHECKS=check-todos",
				"REBAR_TEST_RUN_CLI=1",
			), "bash", args...)

			want := normalizeFindings(shellOut)
			for _, got := range []struct {
				how  string
				out  string
				code int
			}{
				{"native", native.String(), nativeCode},
				{"hand-off", handOut, handCode},
			} {
				if got.code != shellCode || normalizeFindings(got.out) != want {
					t.Errorf("%s differs from the script:\nscript (exit %d):\n%s\n%s (exit %d):\n%s",
						got.how, shellCode, shellOut, got.how, got.code, got.out)
				}
			}
			if !strings.Contains(shellOut, "TODO") && !strings.Contains(shellOut, "No source directories") {
				t.Errorf("fixture exercised nothing: %s", shellOut)
			}
		})
	}
}

// TestShellRegressionTestsAgainstNative runs the script regression tests
// that cover check-todos with REBAR_TEST_NATIVE_BIN, so their assertions
// run against the native check (lib.sh run_in hands off to this binary).
func TestShellRegressionTestsAgainstNative(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	src := rebarSource(t)
	test := filepath.Join(src, "scripts", "tests", "source-scan-excludes.test.sh")
	out, code := run(t, src, isolatedEnv("REBAR_TEST_NATIVE_BIN="+testBinary(t), "REBAR_TEST_RUN_CLI=1"), "bash", test)
	if code != 0 {
		t.Fatalf("source-scan-excludes.test.sh against the native check-todos (exit %d):\n%s", code, out)
	}
	if !strings.Contains(out, "ok check-todos catches TODOs by directory") {
		t.Fatalf("check-todos assertions did not run:\n%s", out)
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

// TestCheckImplSwitch runs the CLI (this binary) end to end: rebar run
// check-todos and rebar check --pre-commit give the same result under
// REBAR_CHECK_IMPL=shell and go, and under go rebar run check-todos works
// with no rebar install at all.
func TestCheckImplSwitch(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	root := writeTree(t, map[string]string{
		".rebarrc":  "tier = 1\n",
		"src/a.go":  "// @todo untracked\n// @tracked tracked\n",
		"cmd/b.go":  "// @todo another\n",
		"lib/c.txt": "// @todo not source\n",
	})
	bin := testBinary(t)
	home := t.TempDir()
	env := func(impl, rebarRoot string) []string {
		return isolatedEnv("REBAR_TEST_RUN_CLI=1", "REBAR_CHECK_IMPL="+impl, "REBAR_ROOT="+rebarRoot, "HOME="+home)
	}
	src := rebarSource(t)

	for _, args := range [][]string{{"run", "check-todos"}, {"run", "check-todos", "cmd"}, {"check", "--pre-commit"}} {
		shellOut, shellCode := run(t, root, env("shell", src), bin, args...)
		goOut, goCode := run(t, root, env("go", src), bin, args...)
		if shellCode != goCode || normalizeFindings(shellOut) != normalizeFindings(goOut) {
			t.Errorf("rebar %v: shell (exit %d):\n%s\ngo (exit %d):\n%s", args, shellCode, shellOut, goCode, goOut)
		}
		if !strings.Contains(goOut, "untracked TODO(s) found") {
			t.Errorf("rebar %v: check-todos did not run:\n%s", args, goOut)
		}
	}

	// No install: the shell implementation has nothing to run, the native
	// check needs nothing but the binary.
	if out, code := run(t, root, env("shell", ""), bin, "run", "check-todos"); code == 0 || !strings.Contains(out, "no script") {
		t.Errorf("shell without an install: exit %d\n%s", code, out)
	}
	if out, code := run(t, root, env("go", ""), bin, "run", "check-todos"); code != 1 || !strings.Contains(out, "FAIL: 2 untracked TODO(s) found") {
		t.Errorf("go without an install: exit %d\n%s", code, out)
	}
}
