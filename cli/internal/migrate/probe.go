package migrate

import (
	"bytes"
	"context"
	"crypto/sha1"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

// runResult is one script run.
type runResult struct {
	Stdout string
	Stderr string
	Code   int
	Err    error // could not run, or timed out
}

func (r runResult) ok() bool { return r.Err == nil && r.Code == 0 }

// steersScripts reports whether an environment variable changes what rebar
// scripts do: every REBAR_* (tier, contracts dir, checks_enable, impl
// counts, the project root, the .rebarrc, …), the overrides
// _rebar_config is called with (CONTRACT_EXTENSIONS), the SKIP_* switches
// of ci-check, and the locale, which changes sorting.
func steersScripts(k string) bool {
	switch {
	case strings.HasPrefix(k, "REBAR_"), strings.HasPrefix(k, "SKIP_"), strings.HasPrefix(k, "LC_"):
		return true
	case k == "CONTRACT_EXTENSIONS", k == "LANG", k == "GIT_DIR", k == "GIT_WORK_TREE", k == "GIT_INDEX_FILE":
		return true
	}
	return false
}

// scrubbedEnv is the caller's environment without the variables that steer
// rebar scripts, so a decision never depends on the shell migrate was run
// from; every probe states its own.
func scrubbedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if steersScripts(k) {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// SteeringEnv lists the variables set in the caller's environment that
// migrate ignores but `rebar check` would honor (REBAR_ROOT, which only
// locates the install, is left out).
func SteeringEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if steersScripts(k) && k != "REBAR_ROOT" && !strings.HasPrefix(k, "LC_") && k != "LANG" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// run executes `bash args…` in dir with env, a timeout, and stdin closed.
// The script runs in its own process group so a timeout kills everything it
// started.
func run(timeout time.Duration, dir string, env []string, args ...string) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.Command("bash", args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return runResult{Err: err, Code: -1}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
		// Anything the script left running in the background goes too.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return runResult{Stdout: out.String(), Stderr: errb.String(), Code: -1,
			Err: fmt.Errorf("timed out after %s", timeout)}
	}
	res := runResult{Stdout: out.String(), Stderr: errb.String()}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.Code = ee.ExitCode()
		} else {
			res.Err, res.Code = err, -1
		}
	}
	return res
}

// legacyHarness sources a legacy check-ground-truth.sh with its output
// discarded and prints what its compute_metrics() emits, whether the script
// returns (it guards its main block) or exits from top-level verification.
// $0 is a file next to the copy, so `$(dirname "$0")/..` is the project
// root while the sourced file's BASH_SOURCE differs from $0.
const legacyHarness = `exec 3>&1
__rebar_migrate_emit() {
  trap - EXIT
  if declare -F compute_metrics >/dev/null; then
    compute_metrics >&3 2>/dev/null
  fi
}
trap __rebar_migrate_emit EXIT
source "$REBAR_MIGRATE_LEGACY" >/dev/null 2>&1 </dev/null
`

var (
	metricLineRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)=(\S*)\s*$`)
	ansiRe       = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	// Value-bearing lines of the known check-ground-truth outputs.
	outputValueRes = []*regexp.Regexp{
		regexp.MustCompile(`^(?:OK|NEW):\s+([A-Za-z0-9_.-]+)\s*=\s*([0-9]+)`),
		regexp.MustCompile(`^DRIFT:\s+([A-Za-z0-9_.-]+)\s.*(?:code says|repo has|actual(?: is)?)\s+([0-9]+)`),
	}
)

// metricValues is an ordered key → value list.
type metricValues struct {
	Keys []string
	Val  map[string]string
}

func (m *metricValues) add(k, v string) {
	if m.Val == nil {
		m.Val = map[string]string{}
	}
	if _, ok := m.Val[k]; !ok {
		m.Keys = append(m.Keys, k)
	}
	m.Val[k] = v
}

func parseMetricLines(s string) metricValues {
	var mv metricValues
	for _, line := range strings.Split(s, "\n") {
		if m := metricLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			mv.add(m[1], m[2])
		}
	}
	return mv
}

// parseCheckOutput reads metric values from OK/DRIFT/NEW lines.
func parseCheckOutput(s string) metricValues {
	var mv metricValues
	for _, line := range strings.Split(ansiRe.ReplaceAllString(s, ""), "\n") {
		line = strings.TrimSpace(line)
		for _, re := range outputValueRes {
			if m := re.FindStringSubmatch(line); m != nil {
				mv.add(m[1], m[2])
				break
			}
		}
	}
	return mv
}

// gitBlobHash is `git hash-object` of data.
func gitBlobHash(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

//go:embed released-blobs.txt
var releasedManifest string

//go:embed released
var releasedSources embed.FS

// releasedSource is the content of a released scanner version: embedded in
// the CLI, else from the install's git history (nil when neither has it).
func releasedSource(rebarRoot, blob string) []byte {
	if data, err := releasedSources.ReadFile("released/" + blob); err == nil {
		return data
	}
	if rebarRoot == "" {
		return nil
	}
	if data, err := exec.Command("git", "-C", rebarRoot, "cat-file", "blob", blob).Output(); err == nil && gitBlobHash(data) == blob {
		return data
	}
	return nil
}

// manifestBlobs indexes released-blobs.txt by file name.
func manifestBlobs() map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, line := range strings.Split(releasedManifest, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if out[f[1]] == nil {
			out[f[1]] = map[string]bool{}
		}
		out[f[1]][f[0]] = true
	}
	return out
}

// releasedBlobs returns the blob ids of every released version of a
// bootstrap file (scripts/<name>, templates/project-bootstrap/scripts/<name>):
// the manifest embedded in the CLI (scripts/gen-released-blobs.sh; works for
// installs without git history, such as installer-made release trees), the
// install's release history when it is a git clone (tags, main, release
// branches; not --all, which would count stashes and unmerged branches), and
// the install's own current files.
func releasedBlobs(rebarRoot, name string) map[string]bool {
	out := map[string]bool{}
	for h := range manifestBlobs()[name] {
		out[h] = true
	}
	if data := templateScript(name); data != nil {
		out[gitBlobHash(data)] = true // this rebar's own template file
	}
	if rebarRoot == "" {
		return out // ("git -C ''" would read the working directory's history)
	}
	args := []string{"-C", rebarRoot, "log", "--format=", "--raw", "--no-abbrev", "--no-renames",
		"--tags", "--branches=main", "--branches=release/*", "--remotes=origin/main", "--remotes=origin/release/*", "HEAD",
		"--", "templates/project-bootstrap/scripts/" + name, "scripts/" + name}
	if data, err := exec.Command("git", args...).Output(); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			f := strings.Fields(line)
			if len(f) >= 5 && strings.HasPrefix(f[0], ":") {
				for _, h := range f[2:4] {
					if strings.Trim(h, "0") != "" {
						out[h] = true
					}
				}
			}
		}
	}
	for _, p := range []string{filepath.Join(rebarRoot, "scripts", name), filepath.Join(rebarRoot, "templates/project-bootstrap/scripts", name)} {
		if data, err := os.ReadFile(p); err == nil {
			out[gitBlobHash(data)] = true
		}
	}
	return out
}

var (
	intRe        = regexp.MustCompile(`\b[0-9]+\b`)
	listMarkerRe = regexp.MustCompile(`(?m)^\s*[0-9]+[.)]\s`)
	pathRe       = regexp.MustCompile(`[A-Za-z0-9_.@-]*(?:/[A-Za-z0-9_.@-]+)+(?::[0-9]+)?|[A-Za-z0-9_.@-]+\.[A-Za-z0-9]+:[0-9]+`)
)

// facts reduces check output to what it found: the file references it
// prints (sorted) and the numbers it reports (in order, not list numbering). Two outputs with the same
// facts and exit status differ only in wording.
func facts(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = listMarkerRe.ReplaceAllString(s, " ")
	paths := pathRe.FindAllString(s, -1)
	rest := pathRe.ReplaceAllString(s, " ")
	ints := intRe.FindAllString(rest, -1)
	// File references may come out in another order; numbers keep theirs,
	// so "3 errors, 0 warnings" never equals "0 errors, 3 warnings".
	sort.Strings(paths)
	return strings.Join(paths, "\n") + "\x00" + strings.Join(ints, " ")
}

// compareRuns decides whether the install's run behaves like the legacy
// copy's: same exit status and the same output, or output that differs only
// in wording (same file references and numbers).
func compareRuns(legacy, install runResult) (bool, string) {
	if legacy.Err != nil {
		return false, "legacy copy did not run: " + legacy.Err.Error()
	}
	if install.Err != nil {
		return false, "install's script did not run: " + install.Err.Error()
	}
	if legacy.Code != install.Code {
		return false, fmt.Sprintf("exit status %d (legacy) vs %d (install); %s", legacy.Code, install.Code, firstDiff(legacy.Stdout, install.Stdout))
	}
	le, ie := stderrOf(legacy), stderrOf(install)
	if facts(le) != facts(ie) {
		return false, "stderr differs: " + firstDiff(le, ie)
	}
	if legacy.Stdout == install.Stdout {
		return true, "identical output"
	}
	if facts(legacy.Stdout) == facts(install.Stdout) {
		return true, "same exit status, findings and counts (wording differs)"
	}
	return false, firstDiff(legacy.Stdout, install.Stdout)
}

// stderrOf is a run's stderr without rebar's own notices (deprecation,
// checks_enable warnings), which only one side prints.
func stderrOf(r runResult) string {
	var keep []string
	for _, l := range strings.Split(r.Stderr, "\n") {
		if !strings.HasPrefix(l, "rebar:") && !strings.HasPrefix(l, "rebar ") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// firstDiff describes the first line where the copy's and the install's
// outputs differ.
func firstDiff(a, b string) string { return firstDiffAs(a, b, "legacy", "install") }

// firstDiffAs is firstDiff with other names for the two sides.
func firstDiffAs(a, b, la, lb string) string {
	al := strings.Split(ansiRe.ReplaceAllString(a, ""), "\n")
	bl := strings.Split(ansiRe.ReplaceAllString(b, ""), "\n")
	n := 0
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			n++
		}
	}
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("%d line(s) differ; first at line %d: %s %q, %s %q", n, i+1, la, clip(x), lb, clip(y))
		}
	}
	return "outputs differ"
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 70 {
		return s[:67] + "..."
	}
	return s
}
