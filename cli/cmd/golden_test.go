package cmd

import (
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The golden harness of rebar's checks: every case of a check is a fixture
// tree whose expected output lives in testdata/<check>/<case>.golden. The
// goldens were captured from the bash scripts (rebar 3.2 required the
// script, the native check and the script handing off to the native check
// to agree on every case), so coverage survived the scripts' deletion.
// Regenerate a check's goldens, after a deliberate change, with
//
//	go test ./cmd -run 'TestComputeRegistry' -update-golden
//
// Contract references in fixture bodies are spelled @contract:.

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

// filler is n bytes or more of text lines.
func filler(n int) string {
	return strings.Repeat("filler text\n", n/12+1)
}

// goldenCase is a fixture tree for one of rebar's checks, with its
// expected output and exit code in a golden file (runGolden). The goldens
// were captured from the check scripts while they existed (rebar 3.2 ran
// both and required the same output), so they hold the scripts' behaviour;
// rebar 4 deleted the scripts. divergesOn documents a case where the
// script's behaviour depended on the platform's userland ("bsd" or "gnu"
// grep, find and date; "all" where the script failed in a way that named
// itself, like bash's "unbound variable"): the check picks the other
// platform's, and want and wantExit spell that out. Bodies and want go
// through fixtureText (testutil.Tags spellings, dates relative to today);
// "$ROOT" in env and args is the tree's root.
type goldenCase struct {
	name       string
	files      map[string]string
	links      map[string]string // link → target, relative to the link
	args       []string
	env        []string
	divergesOn string
	want       string
	wantExit   int
	// git makes the tree a git work tree with files (and links) staged;
	// untracked files are written after staging.
	git       bool
	untracked map[string]string
	// setup runs after the tree is written (and staged, with git), for
	// fixtures that need more: commits, branches, clones.
	setup func(t *testing.T, root string)
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

// undoAgo puts back the {{ago:N}} tokens of a case's fixtures for the
// dates they became (fixtureText), so goldens hold the tokens and do not
// change with the date.
func undoAgo(out string, tc goldenCase, loc *time.Location) string {
	var text strings.Builder
	for _, body := range tc.files {
		text.WriteString(body)
	}
	text.WriteString(tc.want)
	text.WriteString(strings.Join(tc.env, "\n"))
	text.WriteString(strings.Join(tc.args, "\n"))
	today := time.Now().In(loc)
	for _, m := range agoRE.FindAllStringSubmatch(text.String(), -1) {
		n, _ := strconv.Atoi(m[1])
		out = strings.ReplaceAll(out, today.AddDate(0, 0, -n).Format("2006-01-02"), m[0])
	}
	return out
}

// scriptSettings are the variables a check reads from the environment.
// Each run starts with them unset (testutil.IsolatedEnv drops REBAR_* and
// _REBAR_*) and then gets the case's values.
var scriptSettings = []string{
	"REBAR_TIER", "REBAR_CHECKS_ENABLE", "REBAR_CONTRACTS_DIR", "REBAR_CONTRACT_NAMESPACE", "REBAR_RCFILE",
	"_REBAR_CHECKS_VALIDATED", "_REBAR_NAMESPACE_VALIDATED", "CONTRACT_EXTENSIONS", "SKIP_JTBD", "SKIP_PREFIX_UNIQUENESS",
	"REBAR_REGISTRY_IMPL_COUNTS", "REBAR_METRICS_STRICT",
}

// caseEnv is testutil.IsolatedEnv(extra...) without the check settings
// the process may have (CONTRACT_EXTENSIONS, SKIP_JTBD, …), so a CLI run
// sees only the case's.
func caseEnv(extra ...string) []string {
	var env []string
	for _, kv := range testutil.IsolatedEnv() {
		if k, _, _ := strings.Cut(kv, "="); !slices.Contains(scriptSettings, k) {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

// goldenCheck says how to run a check's cases.
type goldenCheck struct {
	name string
	// exercised reports whether a case's output shows it tested something
	// (nil: anything goes).
	exercised func(out string) bool
	// writes makes the files a run writes or removes part of its output
	// (see takeChanges); the tree is restored after each run.
	writes bool
	// golden names the testdata directory holding the expected output of
	// every case (see checkGolden).
	golden string
	// normalize rewrites what differs between runs (a timestamp) in the
	// output before it is compared (nil: as is).
	normalize func(out string) string
}

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden files from the checks' output")

// gitTree makes root a git work tree with its files staged, then writes
// the untracked files.
func gitTree(t *testing.T, root string, untracked map[string]string, loc *time.Location) {
	t.Helper()
	testutil.Git(t, root, "init", "-q", "-b", "main")
	testutil.Git(t, root, "add", "-A")
	for rel, body := range untracked {
		testutil.WriteFile(t, root, rel, fixtureText(body, loc))
	}
}

// treeState maps every file below root, outside .git, to its contents.
func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		state[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// takeChanges describes the files a run wrote or removed below root since
// before, and puts root back as it was.
func takeChanges(t *testing.T, root string, before map[string]string) string {
	t.Helper()
	after := treeState(t, root)
	var paths []string
	for p := range after {
		if body, ok := before[p]; !ok || body != after[p] {
			paths = append(paths, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		full := filepath.Join(root, p)
		body, existed := before[p]
		if now, ok := after[p]; ok {
			b.WriteString("--- wrote " + p + " ---\n" + now)
		} else {
			b.WriteString("--- removed " + p + " ---\n")
		}
		if existed {
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Remove(full); err != nil {
			t.Fatal(err)
		}
	}
	return b.String()
}

// goldenName is the golden file of a case.
func goldenName(dir, name string) string {
	return filepath.Join("testdata", dir, strings.Trim(nonWord.ReplaceAllString(strings.ToLower(name), "-"), "-")+".golden")
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// goldenText is output as a golden file keeps it: the tree's root spelled
// $ROOT, the exit code on the last line.
func goldenText(out, root string, code int) string {
	return strings.ReplaceAll(out, root, "$ROOT") + "--- exit " + strconv.Itoa(code) + " ---\n"
}

// checkGolden compares out (exit code) with a case's golden file, or
// rewrites it under -update-golden.
func checkGolden(t *testing.T, dir, name, out, root string, code int) {
	t.Helper()
	path := goldenName(dir, name)
	got := goldenText(out, root, code)
	if *updateGolden {
		testutil.WriteFile(t, ".", path, got)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden file (run with -update-golden): %v", err)
	}
	if got != string(want) {
		t.Errorf("differs from %s:\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// runGolden runs the check on every case's tree and compares its output
// and exit code, and the files it wrote (check.writes), with the case's
// golden file; -update-golden rewrites them. A check runs in process, under
// config.WithEnv with the case's environment, so cases run in parallel; a
// case that sets TZ (which time.Local reads once per process) runs through
// the CLI instead (this binary as `rebar native <check>`), from the tree's
// root as the script ran.
func runGolden(t *testing.T, check goldenCheck, cases []goldenCase) {
	t.Helper()
	c, ok := checks.Lookup(check.name)
	if !ok {
		t.Fatalf("%s is not registered", check.name)
	}
	for _, tc := range cases {
		if tc.setup != nil {
			fixedGit(t) // setups commit (commits), which must not t.Setenv in a parallel case
			break
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			loc := time.Local
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
			if tc.git {
				gitTree(t, root, tc.untracked, loc)
			}
			// The in-process environment: the process's without the check
			// settings, plus the case's (but not locale or zone).
			var nativeVars, env []string
			for _, kv := range os.Environ() {
				if k, _, _ := strings.Cut(kv, "="); !slices.Contains(scriptSettings, k) {
					nativeVars = append(nativeVars, kv)
				}
			}
			inProcess := true
			for _, kv := range tc.env {
				kv = strings.ReplaceAll(kv, "$ROOT", root)
				env = append(env, kv)
				k, _, _ := strings.Cut(kv, "=")
				switch {
				case k == "TZ":
					inProcess = false
				case !strings.HasPrefix(k, "LC_") && k != "LANG":
					nativeVars = append(nativeVars, kv)
				}
			}
			inEnv := func(f func()) { config.WithEnv(&config.Env{Vars: nativeVars}, f) }
			if tc.setup != nil {
				inEnv(func() { tc.setup(t, root) })
			}
			var before map[string]string
			if check.writes {
				before = treeState(t, root)
			}
			tcArgs := rootArgs(tc.args, root)
			var got string
			var code int
			if inProcess {
				var out strings.Builder
				inEnv(func() {
					code = c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, tcArgs)
				})
				got = out.String()
			} else {
				cliEnv := caseEnv(append([]string{"REBAR_PROJECT_ROOT=" + root, "REBAR_TEST_RUN_CLI=1"}, env...)...)
				got, code = testutil.RunEnv(t, root, cliEnv, testBinary(t), append([]string{"native", check.name}, tcArgs...)...)
			}
			if check.writes {
				got += takeChanges(t, root, before)
			}
			if check.normalize != nil {
				got = check.normalize(got)
			}
			if tc.want != "" {
				if want := fixtureText(tc.want, loc); got != want || code != tc.wantExit {
					t.Errorf("differs from want:\ngot (exit %d):\n%s\nwant (exit %d):\n%s", code, got, tc.wantExit, want)
				}
			}
			checkGolden(t, check.golden, tc.name, undoAgo(got, tc, loc), root, code)
			if check.exercised != nil && !check.exercised(got) {
				t.Errorf("fixture exercised nothing: %s", got)
			}
		})
	}
}

// rootArgs is args with "$ROOT" spelling the tree's root, as in env.
func rootArgs(args []string, root string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, "$ROOT", root)
	}
	return out
}

// cliEnv is the environment for running this binary as the rebar CLI,
// with no rebar tree to find (REBAR_ROOT empty, HOME a fresh directory).
func cliEnv(home string) []string {
	return testutil.IsolatedEnv("REBAR_TEST_RUN_CLI=1", "REBAR_ROOT=", "HOME="+home)
}

// checkGoldenText compares text with the golden file dir/name as is, or
// rewrites it under -update-golden: for sessions whose output a test
// assembles itself.
func checkGoldenText(t *testing.T, dir, name, text string) {
	t.Helper()
	path := goldenName(dir, name)
	if *updateGolden {
		testutil.WriteFile(t, ".", path, text)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden file (run with -update-golden): %v", err)
	}
	if text != string(want) {
		t.Errorf("differs from %s:\ngot:\n%s\nwant:\n%s", path, text, want)
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
