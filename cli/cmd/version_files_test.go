package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// isolateGit points git at a throwaway global config (identity, no signing,
// no hooks), so commands that run git don't depend on the developer's setup.
func isolateGit(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, ".gitconfig")
	body := "[user]\n\tname = t\n\temail = t@example.com\n[commit]\n\tgpgsign = false\n[core]\n\thooksPath = /dev/null\n[init]\n\tdefaultBranch = main\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
}

var badgeRE = regexp.MustCompile(`\*\*rebar (v[^*]+)\*\*`)

// A project created by `rebar new` must pass its own compliance check, which
// requires the README badge and .rebar-version to name the same version.
func TestNewWritesMatchingVersionFiles(t *testing.T) {
	isolateGit(t)
	t.Chdir(t.TempDir())
	newDescription = ""
	if err := runNew(newCmd, []string{"probe"}); err != nil {
		t.Fatalf("rebar new: %v", err)
	}

	version, err := os.ReadFile(filepath.Join("probe", ".rebar-version"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(version)); got != badgeVersion() {
		t.Errorf(".rebar-version = %q, want %q", got, badgeVersion())
	}

	readme, err := os.ReadFile(filepath.Join("probe", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := badgeRE.FindStringSubmatch(string(readme))
	if m == nil {
		t.Fatalf("README.md has no rebar badge:\n%s", readme)
	}
	if m[1] != strings.TrimSpace(string(version)) {
		t.Errorf("README badge says %s but .rebar-version says %s", m[1], strings.TrimSpace(string(version)))
	}
}

// `rebar init` / `rebar audit --fix` create a missing .rebar-version with the
// running CLI's version.
func TestBootstrapWritesCurrentVersion(t *testing.T) {
	root := t.TempDir()
	bootstrapV2Files(root)
	got, err := os.ReadFile(filepath.Join(root, ".rebar-version"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != badgeVersion() {
		t.Errorf(".rebar-version = %q, want %q", strings.TrimSpace(string(got)), badgeVersion())
	}
}

// A branch build stamps Version as <branch>@<sha7>; .rebar-version gets the
// release string, as the README badge does, so the two still agree.
func TestBranchBuildWritesReleaseVersion(t *testing.T) {
	// -ldflags sets Version before init copies it into rootCmd.Version.
	saved, savedRoot := Version, rootCmd.Version
	Version, rootCmd.Version = "feat/x@abc1234", "feat/x@abc1234"
	t.Cleanup(func() { Version, rootCmd.Version = saved, savedRoot })
	root := t.TempDir()
	bootstrapV2Files(root)
	got, err := os.ReadFile(filepath.Join(root, ".rebar-version"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != releaseVersion {
		t.Errorf(".rebar-version = %q on a branch build, want %q", strings.TrimSpace(string(got)), releaseVersion)
	}
}

// A project created by `rebar new` passes the registry check, which runs at
// every tier, without the user first running compute-registry.
func TestNewWritesCurrentRegistry(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(root, "architecture", "CONTRACT-REGISTRY.md")); err != nil {
		t.Fatalf("rebar new wrote no registry: %v", err)
	}
	check := exec.Command("bash", filepath.Join(src, "scripts", "compute-registry.sh"), "--check")
	check.Env = append(os.Environ(), "REBAR_PROJECT_ROOT="+root)
	check.Dir = root
	if out, err := check.CombinedOutput(); err != nil {
		t.Fatalf("compute-registry --check on a new project: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output(); err != nil || len(out) != 0 {
		t.Fatalf("registry not in the initial commit: %q %v", out, err)
	}
}
