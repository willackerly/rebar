package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// ignored reports whether git ignores rel in repo.
func ignored(t *testing.T, repo, rel string) bool {
	t.Helper()
	err := exec.Command("git", "-C", repo, "check-ignore", "-q", "--no-index", rel).Run()
	if err == nil {
		return true
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s: %v", rel, err)
	return false
}

// The steward writes STEWARD_REPORT.md and <contracts_dir>/.state/*.json on
// every scan, and its contract says they are gitignored, so the first `rebar
// check` in a new project must not leave them untracked.
func TestNewThenCheckLeavesTreeClean(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	isolateGit(t)
	t.Setenv("REBAR_ROOT", src)
	t.Chdir(t.TempDir())
	newDescription = ""
	if err := runNew(newCmd, []string{"probe"}); err != nil {
		t.Fatalf("rebar new: %v", err)
	}
	root, _ := filepath.Abs("probe")

	// What `rebar check` runs (the built-in suite; the steward needs no jq).
	c, _ := checks.Lookup("ci-check")
	var out strings.Builder
	if code := c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, []string{"--strict"}); code != 0 {
		t.Fatalf("rebar check on a new project: exit %d\n%s", code, out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "STEWARD_REPORT.md")); err != nil {
		t.Fatalf("the steward did not run (no STEWARD_REPORT.md), so this test proves nothing: %v", err)
	}
	status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil || len(status) != 0 {
		t.Fatalf("rebar check left the tree dirty: %q %v", status, err)
	}
}

// `rebar init` adds the steward outputs under the project's contracts_dir,
// once, to an existing .gitignore, without ignoring the tracked outbox.
func TestEnsureGitignoreFollowsContractsDir(t *testing.T) {
	root := testutil.InitRepo(t)
	testutil.WriteFile(t, root, ".rebarrc", "tier = 1\ncontracts_dir = specs\n")
	testutil.WriteFile(t, root, ".gitignore", "node_modules/\n.rebar/salt\n")

	ensureGitignore(root)
	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	ensureGitignore(root)
	second, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(first) != string(second) {
		t.Errorf("ensureGitignore is not idempotent:\n--- first\n%s--- second\n%s", first, second)
	}
	if n := strings.Count(string(first), ".rebar/salt"); n != 1 {
		t.Errorf(".rebar/salt appears %d times:\n%s", n, first)
	}

	for _, rel := range []string{"STEWARD_REPORT.md", "specs/.state/steward-report.json", "specs/.state/S1-FOO.1.0.json", ".rebar/keys/k"} {
		if !ignored(t, root, rel) {
			t.Errorf("%s not ignored by:\n%s", rel, first)
		}
	}
	for _, rel := range []string{"specs/.state/pending-notifications.md", "specs/CONTRACT-S1-FOO.1.0.md"} {
		if ignored(t, root, rel) {
			t.Errorf("%s is ignored by:\n%s", rel, first)
		}
	}
}

// The bootstrap template's .gitignore (for `cp -r templates/project-bootstrap/.`)
// covers what the steward writes and lists nothing that no rebar tool writes.
func TestTemplateGitignoreCoversStewardOutputs(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "templates", "project-bootstrap", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	root := testutil.InitRepo(t)
	testutil.WriteFile(t, root, ".gitignore", string(body))
	for _, rel := range []string{"STEWARD_REPORT.md", "architecture/.state/steward-report.json", "architecture/.state/S1-FOO.1.0.json"} {
		if !ignored(t, root, rel) {
			t.Errorf("template .gitignore does not ignore %s", rel)
		}
	}
	for _, rel := range []string{"architecture/.state/.gitkeep", "architecture/.state/pending-notifications.md"} {
		if ignored(t, root, rel) {
			t.Errorf("template .gitignore ignores %s", rel)
		}
	}
	if strings.Contains(string(body), "steward-report.md") {
		t.Errorf("template .gitignore lists steward-report.md, which nothing writes (the steward writes STEWARD_REPORT.md)")
	}
}
