// Package testutil holds helpers shared by rebar's Go tests: writing files
// into temp trees and running git in throwaway repositories.
package testutil

import (
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
