// Package testutil holds helpers shared by rebar's Go tests: writing files
// into temp trees, running commands (git in throwaway repositories, rebar's
// scripts in an environment free of rebar settings), and locating the rebar
// checkout the tests run in.
package testutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitArgs isolates test repos from the developer's git config: a fixed
// identity, no commit signing (which would need the developer's signer), and
// no hooks.
var gitArgs = []string{
	"-c", "user.name=t",
	"-c", "user.email=t@example.com",
	"-c", "commit.gpgsign=false",
	"-c", "tag.gpgsign=false",
	"-c", "core.hooksPath=/dev/null",
}

// WriteFile writes body to root/rel with mode 0644, creating parent
// directories, and returns the full path.
func WriteFile(t testing.TB, root, rel, body string) string {
	t.Helper()
	return write(t, root, rel, body, 0o644)
}

// WriteExecutable is WriteFile with mode 0755.
func WriteExecutable(t testing.TB, root, rel, body string) string {
	t.Helper()
	return write(t, root, rel, body, 0o755)
}

func write(t testing.TB, root, rel, body string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return p
}

// Run runs a command in dir, fails the test if it errors, and returns its
// trimmed combined output.
func Run(t testing.TB, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Git runs git in dir with the isolating config above, fails the test if it
// errors, and returns its trimmed combined output.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	return Run(t, dir, "git", append(append([]string{}, gitArgs...), args...)...)
}

// TryGit is Git for commands expected to fail sometimes (a merge that may
// conflict): it returns the error instead of failing the test.
func TryGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append(append([]string{}, gitArgs...), args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// IsolateGitConfig points git at a throwaway global config (fixed identity,
// no signing, no hooks) and ignores the system config, for tests that run
// code which shells out to git itself (e.g. `rebar commit`).
func IsolateGitConfig(t testing.TB) {
	t.Helper()
	dir := t.TempDir()
	cfg := WriteFile(t, dir, "gitconfig", "[user]\n\tname = t\n\temail = t@example.com\n"+
		"[commit]\n\tgpgsign = false\n[tag]\n\tgpgsign = false\n[core]\n\thooksPath = /dev/null\n"+
		"[init]\n\tdefaultBranch = main\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// InitRepo returns a new temp directory holding an empty git repository on
// branch main. It skips the test when git is not installed.
func InitRepo(t testing.TB) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	Git(t, dir, "init", "-q", "-b", "main")
	return dir
}

// IsolatedEnv is the test process's environment without rebar's settings
// (REBAR_*, _REBAR_*), plus extra, so a developer's REBAR_TIER,
// REBAR_CHECK_IMPL or REBAR_RCFILE cannot leak into a fixture run.
func IsolatedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "REBAR_") || strings.HasPrefix(kv, "_REBAR_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// RunEnv runs name in dir with exactly env and returns its untrimmed
// combined output and exit code. A non-zero exit is a result, not a test
// failure; failing to start the command is.
func RunEnv(t testing.TB, dir string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(out), 0
}

// Tags spells the TODO tags in fixture bodies: @todo becomes the untracked
// tag and @tracked the tracked one, so that rebar's own check-todos,
// scanning the test sources, does not count the fixtures.
var Tags = strings.NewReplacer("@todo", "TO"+"DO:", "@tracked", "TRACKED"+"-TASK:")

// RebarSource returns the root of the rebar checkout the test runs in (the
// directory holding scripts/_rebar-config.sh above the working directory),
// and skips the test when there is none.
func RebarSource(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "_rebar-config.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("rebar scripts not found above the test directory")
		}
		dir = parent
	}
}
