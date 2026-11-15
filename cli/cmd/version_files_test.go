package cmd

import (
	"os"
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

var badgeVersion = regexp.MustCompile(`\*\*rebar (v[^*]+)\*\*`)

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
	if got := strings.TrimSpace(string(version)); got != rootCmd.Version {
		t.Errorf(".rebar-version = %q, want %q", got, rootCmd.Version)
	}

	readme, err := os.ReadFile(filepath.Join("probe", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := badgeVersion.FindStringSubmatch(string(readme))
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
	if strings.TrimSpace(string(got)) != rootCmd.Version {
		t.Errorf(".rebar-version = %q, want %q", strings.TrimSpace(string(got)), rootCmd.Version)
	}
}
