package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

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
	return inWalkOrderBy(out, operands, findingLine, 1)
}

// inWalkOrderBy is inWalkOrder for findings that start with a line
// matching finding (whose first group is the file's path) and span that
// many lines (BROKEN: and its Expected: line).
func inWalkOrderBy(out string, operands []string, finding *regexp.Regexp, span int) string {
	lines := strings.Split(out, "\n")
	type record struct {
		lines []string
		op    int
		path  []string
	}
	var slots []int
	var found []record
	for i := 0; i < len(lines); i++ {
		m := finding.FindStringSubmatch(lines[i])
		if m == nil || i+span > len(lines) {
			continue
		}
		f := record{lines: append([]string(nil), lines[i:i+span]...), op: len(operands)}
		for j, op := range operands {
			if op != "/" {
				op = strings.TrimRight(op, "/")
			}
			if m[1] == op || strings.HasPrefix(m[1], op+"/") {
				f.op, f.path = j, strings.Split(strings.TrimPrefix(m[1], op), "/")
				break
			}
		}
		slots = append(slots, i)
		found = append(found, f)
		i += span - 1
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
		copy(lines[i:], found[k].lines)
	}
	return strings.Join(lines, "\n")
}

// filler is n bytes or more of text lines.
func filler(n int) string {
	return strings.Repeat("filler text\n", n/12+1)
}

// parityCase is a fixture tree for a ported check. The script and the
// native check must give the same output and exit code on it, except on
// the userland named by divergesOn ("bsd" or "gnu" grep, find and date),
// where the script's behaviour is platform-dependent and the native check
// picks the other platform's (want and wantExit are then the native
// result, checked everywhere). Bodies and want go through fixtureText
// (testutil.Tags spellings, dates relative to today); "$ROOT" in env is
// the tree's root.
type parityCase struct {
	name       string
	files      map[string]string
	links      map[string]string // link → target, relative to the link
	args       []string
	env        []string
	divergesOn string
	want       string
	wantExit   int
}

var todosParityCases = []parityCase{
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

// agoRE is a date placeholder in fixture text: {{ago:N}} is the date N days
// before today (negative N: after).
var agoRE = regexp.MustCompile(`\{\{ago:(-?[0-9]+)\}\}`)

// fixtureText spells the tags (testutil.Tags) and expands {{ago:N}} to a
// date in loc.
func fixtureText(s string, loc *time.Location) string {
	today := time.Now().In(loc)
	return agoRE.ReplaceAllStringFunc(testutil.Tags.Replace(s), func(m string) string {
		n, _ := strconv.Atoi(agoRE.FindStringSubmatch(m)[1])
		return today.AddDate(0, 0, -n).Format("2006-01-02")
	})
}

// scriptSettings are the variables a check script reads from the
// environment. Each in-process run starts with them unset, as the script
// runs do (testutil.IsolatedEnv drops REBAR_* and _REBAR_*), and then gets
// the case's values.
var scriptSettings = []string{
	"REBAR_TIER", "REBAR_CHECKS_ENABLE", "REBAR_CONTRACTS_DIR", "REBAR_CONTRACT_NAMESPACE", "REBAR_RCFILE",
	"_REBAR_CHECKS_VALIDATED", "_REBAR_NAMESPACE_VALIDATED", "CONTRACT_EXTENSIONS", "SKIP_JTBD", "SKIP_PREFIX_UNIQUENESS",
}

// testErrorLine is bash's complaint when a script compares a setting that
// is not a number (REBAR_TIER=two): the comparison fails and the script
// goes on, as the native check does, silently.
var testErrorLine = regexp.MustCompile(`(?m)^.*: line [0-9]+: \[: .*: integer (expression )?expected\n`)

// loggingCLI returns a rebar command that runs this binary (as the CLI,
// with REBAR_TEST_RUN_CLI=1) and appends each command line to log.
func loggingCLI(t *testing.T) (wrapper, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "hand-offs.log")
	wrapper = testutil.WriteExecutable(t, dir, "rebar",
		"#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+log+"'\nexec '"+testBinary(t)+"' \"$@\"\n")
	return wrapper, log
}

// skippedEarly reports whether env makes a script exit before its hand-off
// line (SKIP_JTBD=1, SKIP_PREFIX_UNIQUENESS=1).
func skippedEarly(env []string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, "SKIP_") && strings.HasSuffix(kv, "=1") {
			return true
		}
	}
	return false
}

// parityCheck says how to compare a check's script and native outputs.
type parityCheck struct {
	name string
	// order puts the script's output into the native order (nil: as is).
	order func(out, root string, args []string) string
	// exercised reports whether a fixture's script output shows it tested
	// something (nil: anything goes).
	exercised func(out string) bool
}

// runParity runs scripts/<check>.sh, the native check in process, and the
// script handing off to this binary (the default `rebar check` path) on
// every case's tree, and requires the same exit code and output, after
// check.order. Script runs pin LC_COLLATE=C: where the scripts sort (globs,
// sort) they follow the locale, and the native checks sort bytes.
func runParity(t *testing.T, check parityCheck, cases []parityCase) {
	t.Helper()
	requireBash(t)
	scriptsDir := filepath.Join(testutil.RebarSource(t), "scripts")
	script := filepath.Join(scriptsDir, check.name+".sh")
	c, ok := checks.Lookup(check.name)
	if !ok {
		t.Fatalf("%s is not registered", check.name)
	}
	flavour := grepFlavour(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loc := time.Local
			var env []string
			for _, kv := range tc.env {
				if tz, ok := strings.CutPrefix(kv, "TZ="); ok {
					l, err := time.LoadLocation(tz)
					if err != nil {
						t.Skip("no zone data:", err)
					}
					loc = l
				}
			}
			files := map[string]string{}
			for rel, body := range tc.files {
				files[rel] = fixtureText(body, loc)
			}
			root := writeTree(t, files, tc.links)
			for _, k := range scriptSettings {
				t.Setenv(k, "")
			}
			inProcess := true
			for _, kv := range tc.env {
				kv = strings.ReplaceAll(kv, "$ROOT", root)
				env = append(env, kv)
				k, v, _ := strings.Cut(kv, "=")
				switch {
				case k == "TZ":
					inProcess = false // time.Local is set once per process
				case !strings.HasPrefix(k, "LC_") && k != "LANG":
					t.Setenv(k, v)
				}
			}
			args := append([]string{script}, tc.args...)
			base := append([]string{"REBAR_PROJECT_ROOT=" + root, "LC_COLLATE=C"}, env...)

			shellOut, shellCode := testutil.RunEnv(t, root, testutil.IsolatedEnv(base...), "bash", args...)
			shellOut = testErrorLine.ReplaceAllString(shellOut, "")

			var native strings.Builder
			nativeCode := c.Run(&checks.Context{Root: root, Stdout: &native, Stderr: &native}, tc.args)

			// The hand-off goes through a wrapper that logs it, so a script
			// that ran itself instead cannot pass for the native check.
			wrapper, log := loggingCLI(t)
			handEnv := append(append([]string{"REBAR_PROJECT_ROOT=" + root, "LC_COLLATE=C"}, handOffEnv(t, scriptsDir)...), env...)
			handEnv = append(handEnv, "REBAR_BIN="+wrapper)
			handOut, handCode := testutil.RunEnv(t, root, testutil.IsolatedEnv(handEnv...), "bash", args...)
			if logged, _ := os.ReadFile(log); !strings.Contains(string(logged), "native "+check.name) && !skippedEarly(env) {
				t.Errorf("the script did not hand off to the native check:\n%s", handOut)
			}

			order := func(out string) string { return out }
			if check.order != nil {
				order = func(out string) string { return check.order(out, root, tc.args) }
			}
			want, wantCode, from := order(shellOut), shellCode, "the script"
			if tc.want != "" {
				want, wantCode, from = fixtureText(tc.want, loc), tc.wantExit, "want"
				if flavour != tc.divergesOn && (shellCode != wantCode || order(shellOut) != want) {
					t.Errorf("on %s userland the script should give want:\nscript (exit %d):\n%s\nwant (exit %d):\n%s", flavour, shellCode, shellOut, wantCode, want)
				}
				if flavour == tc.divergesOn {
					t.Logf("%s userland diverges here, as documented; the script gave (exit %d):\n%s", flavour, shellCode, shellOut)
				}
			}
			results := []struct {
				how  string
				out  string
				code int
			}{{"hand-off", handOut, handCode}}
			if inProcess {
				results = append(results, struct {
					how  string
					out  string
					code int
				}{"native", native.String(), nativeCode})
			}
			for _, got := range results {
				if got.code != wantCode || got.out != want {
					t.Errorf("%s differs from %s:\n%s (exit %d):\n%s\n%s (exit %d):\n%s",
						got.how, from, from, wantCode, want, got.how, got.code, got.out)
				}
			}
			if check.exercised != nil && !check.exercised(shellOut) {
				t.Errorf("fixture exercised nothing: %s", shellOut)
			}
		})
	}
}

// TestTodosParity: check-todos.sh and its native port (runParity), finding
// lines put in the native order.
func TestTodosParity(t *testing.T) {
	runParity(t, parityCheck{
		name: "check-todos",
		order: func(out, root string, args []string) string {
			operands := args
			if len(operands) == 0 {
				for _, d := range sourcescan.ScopeFor(root).Dirs {
					if info, err := os.Stat(filepath.Join(root, d)); err == nil && info.IsDir() {
						operands = append(operands, d)
					}
				}
			}
			return inWalkOrder(out, operands)
		},
		exercised: func(out string) bool {
			return strings.Contains(out, "TODO") || strings.Contains(out, "No source directories")
		},
	}, todosParityCases)
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
			wrapper, log := loggingCLI(t)
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
		{"shell", "check-todos", install, true, false},
		{"", "check-todos", install, true, true}, // go is the default
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
// scripts of the ported checks hand off to the native checks (and under
// shell they run), but a project's own copy,
// even one made from a script that carries the hand-off hook, runs itself
// and keeps its customizations, whether `rebar run` starts it or the
// install's pre-commit.sh reaches it through _rebar_script.
func TestNativeHandOffOnlyFromInstall(t *testing.T) {
	requireBash(t)
	src := testutil.RebarSource(t)

	// An install whose ported scripts say so when they run past the hook.
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
		name := strings.TrimSuffix(filepath.Base(s), ".sh")
		if _, ported := checks.Lookup(name); ported {
			h := `_rebar_native ` + name + ` "$@"`
			if !strings.Contains(text, h) {
				t.Fatalf("%s.sh has no hand-off hook %q", name, h)
			}
			text = strings.Replace(text, h, h+"\necho 'install script ran: "+name+"'; exit 0", 1)
		}
		testutil.WriteExecutable(t, install, filepath.Join("scripts", filepath.Base(s)), text)
	}
	testutil.WriteFile(t, install, "templates/project-bootstrap/README.md", "marks a rebar install\n")
	home := t.TempDir()
	bin := testBinary(t)

	plain := writeTree(t, map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"}, nil)
	for _, name := range checks.Names() {
		out, _ := testutil.RunEnv(t, plain, cliEnv(home, "", install), bin, "run", name)
		if strings.Contains(out, "install script ran") {
			t.Errorf("go: rebar run %s ran the script:\n%s", name, out)
		}
		out, _ = testutil.RunEnv(t, plain, cliEnv(home, "shell", install), bin, "run", name)
		if !strings.Contains(out, "install script ran: "+name) {
			t.Errorf("shell: rebar run %s did not run the install's script:\n%s", name, out)
		}
	}
	tier3 := writeTree(t, map[string]string{".rebarrc": "tier = 3\n", "src/a.go": "package a\n", "architecture/README.md": "x\n"}, nil)
	if out, _ := testutil.RunEnv(t, tier3, append(cliEnv(home, "", install), "SKIP_STEWARD=1", "SKIP_SCRIPT_TESTS=1"), bin, "check"); strings.Contains(out, "install script ran") || !strings.Contains(out, "TODO Tracking") {
		t.Errorf("go: rebar check ran a ported script instead of handing off:\n%s", out)
	}
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

// TestScannerChecksImplSwitch runs the CLI (this binary) end to end on a
// project that trips every ported check: `rebar run <check>` and `rebar
// check` print the same and exit the same under REBAR_CHECK_IMPL=shell and
// the default (go). (TestNativeHandOffOnlyFromInstall shows that go runs
// the native checks.)
func TestScannerChecksImplSwitch(t *testing.T) {
	requireBash(t)
	files := map[string]string{
		".rebarrc":                            "tier = 3\nsource_dirs = src\n",
		"architecture/CONTRACT-S1-FOO.1.0.md": "# S1-FOO\n\n**Status:** active\n\n## Why this exists\n\nx\n",
		"architecture/CONTRACT-S1-BAR.1.0.md": contract,
		"architecture/CONTRACT-REGISTRY.md":   registry("CONTRACT-S1-FOO.1.0.md", "CONTRACT-S1-BAR.1.0.md"),
		"src/a.go":                            "// @contract:S1-FOO.1.0\npackage a\n",
		"src/b.go":                            "package b\n// @contract:S9-GONE.1.0\n// @todo untracked\n",
		"docs/old.md":                         "<!-- freshness: {{ago:40}} -->\n",
	}
	for rel, body := range files {
		files[rel] = fixtureText(body, time.Local)
	}
	project := func() string {
		root := writeTree(t, files, nil)
		testutil.Git(t, root, "init", "-q", "-b", "main")
		testutil.Git(t, root, "add", ".")
		testutil.Git(t, root, "commit", "-q", "-m", "init")
		return root
	}
	bin := testBinary(t)
	home := t.TempDir()
	src := testutil.RebarSource(t)
	ported := []string{"check-freshness", "check-contract-refs", "check-registry", "check-contract-headers", "check-prefix-uniqueness", "check-jtbd-presence", "check-todos"}
	runs := [][]string{{"check", "--strict=false"}, {"check", "--pre-commit"}}
	for _, c := range ported {
		runs = append(runs, []string{"run", c})
	}
	skips := []string{"SKIP_STEWARD=1", "SKIP_SCRIPT_TESTS=1", "SKIP_BOOTSTRAP_SYNC=1"}
	for _, args := range runs {
		shellOut, shellCode := testutil.RunEnv(t, project(), append(cliEnv(home, "shell", src), skips...), bin, args...)
		goOut, goCode := testutil.RunEnv(t, project(), append(cliEnv(home, "", src), skips...), bin, args...)
		if shellCode != goCode || shellOut != goOut {
			t.Errorf("rebar %v: shell (exit %d):\n%s\ngo (exit %d):\n%s", args, shellCode, shellOut, goCode, goOut)
		}
		if args[0] == "check" && !strings.Contains(goOut, "TODO") {
			t.Errorf("rebar %v ran no checks:\n%s", args, goOut)
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
