package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/integrity"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// A protected file whose deletion (or rename) is staged must leave the
// manifest when it is committed, or `rebar verify --strict` reports it as
// Missing forever (seen after `rebar migrate --apply` deleted script copies).
func TestCommitPrunesStagedDeletions(t *testing.T) {
	testutil.IsolateGitConfig(t)
	repo := testutil.InitRepo(t)
	testutil.WriteExecutable(t, repo, "scripts/old.sh", "#!/bin/sh\necho old\n")
	testutil.WriteExecutable(t, repo, "scripts/moved.sh", "#!/bin/sh\necho moved\n")
	testutil.WriteExecutable(t, repo, "scripts/kept.sh", "#!/bin/sh\necho kept\n")
	// A project copy of pre-commit.sh wins over the install's (legacy
	// override), so the commit runs no real checks against this bare repo.
	testutil.WriteExecutable(t, repo, "scripts/pre-commit.sh", "#!/bin/sh\nexit 0\n")
	m := integrity.NewManifest("repo")
	for _, p := range []string{"scripts/old.sh", "scripts/moved.sh", "scripts/kept.sh"} {
		m.Checksums[integrity.CategoryEnforcement][p] = integrity.FileEntry{SHA256: p, Role: "steward"}
	}
	setupCfg(repo)
	testutil.WriteFile(t, repo, ".rebar/repo-id", "repo\n")
	if err := m.Save(cfg.RebarDir); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-q", "-m", "base")

	testutil.Git(t, repo, "rm", "-q", "scripts/old.sh")
	testutil.Git(t, repo, "mv", "scripts/moved.sh", "scripts/renamed.sh")
	commitMessage, commitRole = "remove and rename", ""
	t.Cleanup(func() { commitMessage = "" })
	if err := runCommit(commitCmd, nil); err != nil {
		t.Fatalf("rebar commit: %v", err)
	}

	got, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		t.Fatal(err)
	}
	files := got.Checksums[integrity.CategoryEnforcement]
	for _, gone := range []string{"scripts/old.sh", "scripts/moved.sh"} {
		if _, ok := files[gone]; ok {
			t.Errorf("%s is still in the manifest after its deletion was committed", gone)
		}
	}
	if _, ok := files["scripts/kept.sh"]; !ok {
		t.Error("scripts/kept.sh was dropped from the manifest")
	}
	if _, ok := files["scripts/renamed.sh"]; !ok {
		t.Error("scripts/renamed.sh (the rename target) was not added to the manifest")
	}
	if dirty := testutil.Git(t, repo, "status", "--porcelain"); dirty != "" {
		t.Errorf("manifest change was not committed: %s", dirty)
	}
}

// A failing command prints its error: with SilenceErrors on and nothing in
// main, errors were swallowed and the CLI exited 1 without a word.
func TestExecutePrintsTheError(t *testing.T) {
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"commit", "--repo-root", t.TempDir()})
	t.Cleanup(func() { rootCmd.SetErr(nil); rootCmd.SetArgs(nil) })
	if err := Execute(); err == nil {
		t.Fatal("rebar commit without -m succeeded")
	}
	if !strings.Contains(stderr.String(), "rebar: commit message required") {
		t.Fatalf("stderr = %q, want the error message", stderr.String())
	}
}
