package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// A developer config that signs every commit with an unusable signer must not
// break test repos: Git turns signing and hooks off.
func TestGitIgnoresDeveloperSigningAndHooks(t *testing.T) {
	home := t.TempDir()
	WriteFile(t, home, ".gitconfig", "[commit]\n\tgpgsign = true\n[gpg]\n\tprogram = false\n[core]\n\thooksPath = "+filepath.Join(home, "hooks")+"\n")
	WriteExecutable(t, home, "hooks/pre-commit", "#!/bin/sh\nexit 1\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))

	repo := InitRepo(t)
	WriteFile(t, repo, "a.txt", "a\n")
	Git(t, repo, "add", "a.txt")
	Git(t, repo, "commit", "-q", "-m", "a")
	if got := Git(t, repo, "log", "--format=%an <%ae>", "-1"); got != "t <t@example.com>" {
		t.Fatalf("author = %q, want the fixed test identity", got)
	}
}

func TestTryGitReturnsErrors(t *testing.T) {
	repo := InitRepo(t)
	if _, err := TryGit(repo, "rev-parse", "--verify", "no-such-ref"); err == nil {
		t.Fatal("TryGit on a missing ref returned no error")
	}
}

func TestWriteExecutableMode(t *testing.T) {
	p := WriteExecutable(t, t.TempDir(), "bin/tool", "#!/bin/sh\n")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("mode %v is not executable", info.Mode().Perm())
	}
}
