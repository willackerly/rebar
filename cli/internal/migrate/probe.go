package migrate

import (
	"bytes"
	"context"
	"crypto/sha1"
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

// scrubbedEnv is the caller's environment without the variables that steer
// rebar scripts, so every probe states its own.
func scrubbedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "REBAR_PROJECT_ROOT", "REBAR_RCFILE", "REBAR_TIER", "REBAR_REGISTRY_IMPL_COUNTS", "LC_ALL":
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
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

// releasedBlobs returns the blob hashes rebar has ever shipped for a
// bootstrap file (templates/project-bootstrap/scripts/<name> or
// scripts/<name>), from the install's git history. Empty when the install
// is not a git clone.
func releasedBlobs(rebarRoot, name string) map[string]bool {
	out := map[string]bool{}
	cmd := exec.Command("git", "-C", rebarRoot, "log", "--all", "--format=", "--raw", "--no-abbrev", "--no-renames",
		"--", "templates/project-bootstrap/scripts/"+name, "scripts/"+name)
	data, err := cmd.Output()
	if err != nil {
		return out
	}
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
	// Uncommitted install changes: the working copies count too.
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
// prints and the numbers it reports (not list numbering), each sorted. Two outputs with the same
// facts and exit status differ only in wording.
func facts(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = listMarkerRe.ReplaceAllString(s, " ")
	paths := pathRe.FindAllString(s, -1)
	rest := pathRe.ReplaceAllString(s, " ")
	ints := intRe.FindAllString(rest, -1)
	sort.Strings(paths)
	sort.Strings(ints)
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
	if legacy.Stdout == install.Stdout {
		return true, "identical output"
	}
	if facts(legacy.Stdout) == facts(install.Stdout) {
		return true, "same exit status, findings and counts (wording differs)"
	}
	return false, firstDiff(legacy.Stdout, install.Stdout)
}

// firstDiff describes the first line where two outputs differ.
func firstDiff(a, b string) string {
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
			return fmt.Sprintf("%d line(s) differ; first at line %d: legacy %q, install %q", n, i+1, clip(x), clip(y))
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
