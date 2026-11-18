package cmd

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/assets"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// install-shims writes the kit compiled into the binary, executable where
// the tree has it so, skips identical files, and leaves differing ones
// unless forced.
func TestInstallShims(t *testing.T) {
	dir := t.TempDir()
	written, conflicts, err := installShims(dir, false)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("first run: %v %v", err, conflicts)
	}
	if strings.Join(written, " ") != strings.Join(assets.Kit(), " ") {
		t.Fatalf("wrote %v, want %v", written, assets.Kit())
	}
	for _, rel := range written {
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), assets.Mode("bin/"+rel); got != want {
			t.Errorf("%s: mode %v, want %v", rel, got, want)
		}
	}
	if w, c, err := installShims(dir, false); err != nil || len(w)+len(c) != 0 {
		t.Fatalf("second run should be a no-op: wrote %v, conflicts %v, %v", w, c, err)
	}
	mine := filepath.Join(dir, "ask")
	os.WriteFile(mine, []byte("#!/bin/sh\necho mine\n"), 0o755)
	if w, c, _ := installShims(dir, false); len(w) != 0 || strings.Join(c, " ") != "ask" {
		t.Fatalf("a differing ask: wrote %v, conflicts %v", w, c)
	}
	if data, _ := os.ReadFile(mine); string(data) != "#!/bin/sh\necho mine\n" {
		t.Fatal("install-shims overwrote a differing file without --force")
	}
	if w, _, _ := installShims(dir, true); strings.Join(w, " ") != "ask" {
		t.Fatalf("--force wrote %v", w)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*rebar-tmp*")); len(left) != 0 {
		t.Fatalf("temporary files left behind: %v", left)
	}
}

// install-shims never writes through a symlink: not one at a target (a
// conflict; --force replaces the link itself), and not a planted
// temporary name (the old fixed <file>.rebar-tmp).
func TestInstallShimsSymlinks(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	os.WriteFile(victim, []byte("keep\n"), 0o644)
	os.Symlink(victim, filepath.Join(dir, "ask"))
	os.Symlink(victim, filepath.Join(dir, "ask-server.rebar-tmp"))
	_, conflicts, err := installShims(dir, false)
	if err != nil || strings.Join(conflicts, " ") != "ask" {
		t.Fatalf("conflicts %v, %v; want the symlinked ask", conflicts, err)
	}
	if _, _, err := installShims(dir, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "keep\n" {
		t.Fatalf("wrote through a symlink: victim now %q", data)
	}
	if info, err := os.Lstat(filepath.Join(dir, "ask")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("--force did not replace the ask symlink with the shim: %v %v", info, err)
	}
}

// A bin/install left by rebar before v4.0.0 (it shadows install(1) on PATH)
// is removed once rebar-install is written; anyone else's install is kept.
func TestInstallShimsRemovesLegacyInstall(t *testing.T) {
	legacy := "#!/usr/bin/env bash\n" + legacyInstallMarker + " and optionally configure ASK_SERVER.\n"
	for _, tc := range []struct {
		name, body string
		removed    bool
	}{{"rebar's", legacy, true}, {"someone else's", "#!/bin/sh\nexec /usr/bin/install \"$@\"\n", false}} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "install"), []byte(tc.body), 0o755)
		if _, _, err := installShims(dir, false); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(filepath.Join(dir, "install"))
		if gone := os.IsNotExist(err); gone != tc.removed {
			t.Errorf("%s install: removed = %v, want %v", tc.name, gone, tc.removed)
		}
	}
	for _, rel := range assets.Kit() {
		if rel == "install" {
			t.Error("the kit puts a file named install on PATH")
		}
	}
}

// With no rebar tree anywhere (no REBAR_ROOT, no install beside the binary,
// an empty HOME), new, init, run, check and install-shims work from what is
// compiled into the binary.
func TestBinaryOnly(t *testing.T) {
	requireBash(t)
	testutil.IsolateGitConfig(t)
	bin, home, work := testBinary(t), t.TempDir(), t.TempDir()
	env := append(cliEnv(home), "GIT_CONFIG_GLOBAL="+os.Getenv("GIT_CONFIG_GLOBAL"))

	if out, code := testutil.RunEnv(t, work, env, bin, "new", "demo"); code != 0 {
		t.Fatalf("rebar new (exit %d):\n%s", code, out)
	}
	demo := filepath.Join(work, "demo")
	tmpl := assets.Bootstrap()
	for _, rel := range []string{"scripts/ci-check.sh", "scripts/pre-commit.sh", "scripts/README.md", ".claude/settings.json", ".claude/skills/rebar-coldstart/SKILL.md"} {
		want, _ := fs.ReadFile(tmpl, rel)
		got, err := os.ReadFile(filepath.Join(demo, rel))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("rebar new: %s differs from the template (%v)", rel, err)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(demo, "architecture", "CONTRACT-TEMPLATE.md")); !bytes.Equal(got, assets.ContractTemplate()) {
		t.Error("rebar new: no contract template")
	}
	if info, err := os.Stat(filepath.Join(demo, "scripts", "ci-check.sh")); err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Errorf("scripts/ci-check.sh not executable: %v", err)
	}
	out, code := testutil.RunEnv(t, demo, env, bin, "check")
	if code != 0 || !strings.Contains(out, "14 passed, 0 failed, 2 skipped") || !strings.Contains(out, "(rebar source repo only)") {
		t.Errorf("rebar check in a new project (exit %d):\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, demo, env, bin, "run"); code != 0 || !strings.Contains(out, "Built into rebar:") || !strings.Contains(out, "  check-todos\n") || strings.Contains(out, "Scripts in") {
		t.Errorf("rebar run (exit %d):\n%s", code, out)
	}

	repo := filepath.Join(work, "existing")
	testutil.WriteFile(t, repo, "README.md", "# existing\n")
	testutil.Git(t, repo, "init", "-q", "-b", "main")
	if out, code := testutil.RunEnv(t, repo, env, bin, "init"); code != 0 || strings.Contains(out, "rebar checkout not found") {
		t.Fatalf("rebar init (exit %d):\n%s", code, out)
	}
	for _, rel := range []string{"scripts/ci-check.sh", ".claude/settings.json", "architecture/CONTRACT-TEMPLATE.md"} {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Errorf("rebar init wrote no %s", rel)
		}
	}

	kit := t.TempDir()
	if out, code := testutil.RunEnv(t, work, env, bin, "install-shims", "--dir", kit); code != 0 {
		t.Fatalf("install-shims (exit %d):\n%s", code, out)
	}
	if out, code := testutil.RunEnv(t, work, env, bin, "install-shims", "--dir", kit); code != 0 || !strings.Contains(out, "up to date") {
		t.Errorf("install-shims again (exit %d):\n%s", code, out)
	}
}
