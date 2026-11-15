package migrate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Regression tests for the PR #18 review (B = blocker, S = should-fix).

func commitAll(t *testing.T, p string) {
	t.Helper()
	testutil.Git(t, p, "add", "-A")
	testutil.Git(t, p, "commit", "-q", "-m", "update")
}

func installScript(t *testing.T, install, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(install, "scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func render(p *Plan) string {
	var b bytes.Buffer
	Render(&b, p, false)
	return b.String()
}

// B1: scripts/ as a symlink to a directory outside the project (a shared
// scripts dir, or the install's own scripts/) must be refused, and nothing
// there touched.
func TestRefusesSymlinkedScriptsDir(t *testing.T) {
	install := installRoot(t)
	shared := t.TempDir()
	testutil.WriteExecutable(t, shared, "check-todos.sh", installScript(t, install, "check-todos.sh"))
	before := treeHash(t, shared)

	for name, target := range map[string]string{"shared dir": shared, "install scripts": filepath.Join(install, "scripts")} {
		p := testutil.InitRepo(t)
		testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
		if err := os.Symlink(target, filepath.Join(p, "scripts")); err != nil {
			t.Fatal(err)
		}
		commitAll(t, p)
		if _, err := BuildPlan(opts(p, install)); err == nil || !strings.Contains(err.Error(), "scripts/ resolves") {
			t.Fatalf("%s: BuildPlan err = %v, want a refusal", name, err)
		}
	}
	if treeHash(t, shared) != before {
		t.Fatal("the shared scripts dir changed")
	}
}

// B1: a copy swapped for a symlink after planning is not deleted through.
func TestApplyRefusesSymlinkedTarget(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	outside := testutil.WriteExecutable(t, t.TempDir(), "check-ground-truth.sh", strings.Replace(legacyGroundTruth, "EXTRA", ":", 1))
	gt := filepath.Join(p, "scripts", "check-ground-truth.sh")
	_ = os.Remove(gt)
	if err := os.Symlink(outside, gt); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, opts(p, install), ApplyOptions{AllowDirty: true}); err == nil {
		t.Fatal("apply went through a symlinked copy")
	}
	if !isFile(outside) {
		t.Fatal("the symlink target outside the project was deleted")
	}
}

// B2: a rebar source tree other than the install, or any repo sharing the
// install's root commit, is refused.
func TestRefusesRebarCheckouts(t *testing.T) {
	install := installRoot(t)
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, "templates/project-bootstrap/README.md", "x\n")
	testutil.WriteExecutable(t, p, "scripts/steward.sh", "#!/usr/bin/env bash\n")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh", "#!/usr/bin/env bash\necho other\n")
	commitAll(t, p)
	if _, err := BuildPlan(opts(p, install)); err == nil || !strings.Contains(err.Error(), "rebar source tree") {
		t.Fatalf("source tree: err = %v", err)
	}

	// A fake install and a clone of it (no template dir: only the shared
	// root commit gives it away).
	fake := testutil.InitRepo(t)
	testutil.WriteExecutable(t, fake, "scripts/check-todos.sh", "#!/usr/bin/env bash\necho fake\n")
	commitAll(t, fake)
	clone := filepath.Join(t.TempDir(), "clone")
	testutil.Run(t, fake, "git", "clone", "-q", fake, clone)
	if _, err := BuildPlan(opts(clone, fake)); err == nil || !strings.Contains(err.Error(), "shares history") {
		t.Fatalf("clone: err = %v", err)
	}
}

// S1: `ls docs/*.md` equals find on a project without dotfiles; the probe
// tree has them and keeps the copy.
func TestMetricProbeCatchesDotfiles(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, `echo "docs=$(ls "$REPO_ROOT"/docs/*.md | wc -l | tr -d ' ')"`, "go_files = 2\nshell_scripts = 3\ndocs = 1\n")
	testutil.WriteFile(t, p, "docs/a.md", "# a\n")
	commitAll(t, p)
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	gt := scriptPlan(plan, "check-ground-truth.sh")
	if gt.Verdict != Keep || !strings.Contains(strings.Join(gt.Details, "\n"), "kept      docs = 1 — same value on the project, but differs on a probe tree") {
		t.Fatalf("ls glob accepted:\n%s", render(plan))
	}
}

// S2: a customized scanner's extra filter is exercised by bait files; one
// that can't be turned into a probe path keeps the copy.
func TestScannerFiltersAreExercised(t *testing.T) {
	install := installRoot(t)
	orig := installScript(t, install, "check-todos.sh")
	anchor := `  | grep -v "check-todos" \` + "\n"
	if !strings.Contains(orig, anchor) {
		t.Skip("install check-todos.sh changed shape")
	}
	for name, filter := range map[string]string{
		"test files": `  | grep -v "_test\.go:" \` + "\n",
		"regex":      `  | grep -v "[0-9][0-9]*x" \` + "\n",
	} {
		p := testutil.InitRepo(t)
		testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
		testutil.WriteFile(t, p, "src/a.go", "package a\n")
		testutil.WriteExecutable(t, p, "scripts/check-todos.sh", strings.Replace(orig, anchor, anchor+filter, 1))
		commitAll(t, p)
		plan, err := BuildPlan(opts(p, install))
		if err != nil {
			t.Fatal(err)
		}
		if sp := scriptPlan(plan, "check-todos.sh"); sp.Verdict != Keep {
			t.Fatalf("%s: copy with an extra filter not kept:\n%s", name, render(plan))
		}
	}
}

// S3: the dry run never runs anything in the project, and a copy that
// writes files is kept.
func TestCopyWithSideEffectsIsKept(t *testing.T) {
	install := installRoot(t)
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
	testutil.WriteFile(t, p, "src/a.go", "package a\n")
	orig := installScript(t, install, "check-todos.sh")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh",
		strings.Replace(orig, "set -euo pipefail\n", "set -euo pipefail\ndate >> \"$(dirname \"$0\")/../.todo-history.log\"\n", 1))
	commitAll(t, p)
	before := treeHash(t, p)
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if treeHash(t, p) != before {
		t.Fatal("the dry run changed the project")
	}
	sp := scriptPlan(plan, "check-todos.sh")
	if sp.Verdict != Keep || !strings.Contains(sp.Reason, "writes files") || !strings.Contains(sp.Reason, ".todo-history.log") {
		t.Fatalf("writer not kept: %+v", sp)
	}
}

// S4: exported REBAR_* variables don't change the plan. A tier-1 project
// with an ungated check-freshness copy gets checks_enable either way.
func TestEnvironmentDoesNotSteerDecisions(t *testing.T) {
	install := installRoot(t)
	orig := installScript(t, install, "check-freshness.sh")
	gate := ` && _rebar_skip 2 check-freshness && exit 0`
	if !strings.Contains(orig, gate) {
		t.Skip("install check-freshness.sh changed shape")
	}
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 1\n")
	testutil.WriteExecutable(t, p, "scripts/check-freshness.sh", strings.Replace(orig, gate, "", 1))
	commitAll(t, p)

	clean, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("REBAR_TIER", "3")
	t.Setenv("REBAR_CHECKS_ENABLE", "check-freshness")
	t.Setenv("REBAR_CONTRACTS_DIR", "elsewhere")
	dirty, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	for _, pl := range []*Plan{clean, dirty} {
		if v, ok := acceptedValue(pl.Added, "checks_enable"); !ok || v != "check-freshness" {
			t.Fatalf("checks_enable not proposed:\n%s", render(pl))
		}
	}
	if !strings.Contains(render(dirty), "environment: REBAR_CHECKS_ENABLE, REBAR_CONTRACTS_DIR, REBAR_TIER") {
		t.Fatalf("no warning about the environment:\n%s", render(dirty))
	}
}

// S5: a verified copy that other files call becomes a thin wrapper, apply
// refuses a dirty tree, and a regression rolls everything back.
func TestCallersGetAWrapperAndApplyIsSafe(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	testutil.WriteExecutable(t, p, "scripts/ci.sh", "#!/usr/bin/env bash\n\"$(dirname \"$0\")/check-ground-truth.sh\"\n")
	testutil.WriteFile(t, p, ".github/workflows/ci.yml", "jobs:\n  x:\n    steps:\n      - run: scripts/ci.sh\n")
	testutil.WriteFile(t, p, "METRICS", "go_files = 2\nshell_scripts = 4\n")
	commitAll(t, p)

	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	gt := scriptPlan(plan, "check-ground-truth.sh")
	if gt.Verdict != Replace || !gt.Shim || !isThinWrapper(gt.ReplaceContent) ||
		!bytes.Contains(gt.ReplaceContent, []byte("compute_metrics()")) {
		t.Fatalf("called copy not wrapped:\n%s", render(plan))
	}
	if len(plan.MetricsEdits) != 0 {
		t.Fatalf("a wrapper keeps the file; METRICS must not change: %+v", plan.MetricsEdits)
	}
	if !strings.Contains(render(plan), "CI: .github/workflows/ci.yml:4 runs scripts/ci.sh, which calls the wrapper(s) scripts/check-ground-truth.sh") {
		t.Fatalf("no note for CI reaching the wrapper through scripts/ci.sh:\n%s", render(plan))
	}

	// Dirty tree: refused, nothing changed.
	testutil.WriteFile(t, p, "scratch.txt", "wip\n")
	before := treeHash(t, p)
	if _, err := Apply(plan, opts(p, install), ApplyOptions{}); err == nil || !strings.Contains(err.Error(), "uncommitted or untracked") {
		t.Fatalf("dirty tree: err = %v", err)
	}
	if treeHash(t, p) != before {
		t.Fatal("refused apply changed files")
	}
	os.Remove(filepath.Join(p, "scratch.txt"))

	// A regression (forced: METRICS edited wrongly) rolls back.
	before = treeHash(t, p)
	plan.MetricsEdits = []MetricsEdit{{Key: "go_files", Op: "=", From: "2", To: "99"}}
	res, err := Apply(plan, opts(p, install), ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.RolledBack || treeHash(t, p) != before {
		t.Fatalf("regression not rolled back (rolled back: %v)", res.RolledBack)
	}
}

// S6: CI that calls a script which now needs the rebar CLI gets a note.
func TestCINoteForWrappers(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	testutil.WriteFile(t, p, ".github/workflows/ci.yml", "jobs:\n  x:\n    steps:\n      - run: scripts/check-ground-truth.sh\n")
	commitAll(t, p)
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render(plan), "CI: .github/workflows/ci.yml:4 runs scripts/check-ground-truth.sh, which now calls the `rebar` CLI") {
		t.Fatalf("no CI note:\n%s", render(plan))
	}
}

func TestMergeRCClosesAnOpenBlock(t *testing.T) {
	out, _ := MergeRC([]byte("tier = 1\n"+blockStart+"\nsource_dirs = a\n"), []Setting{{Key: "k", Value: "v"}})
	if strings.Count(string(out), blockStart) != 1 || !strings.HasSuffix(string(out), "k = v\n"+blockEnd+"\n") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestFactsKeepNumberOrder(t *testing.T) {
	if ok, _ := compareRuns(runResult{Stdout: "3 errors, 0 warnings"}, runResult{Stdout: "0 errors, 3 warnings"}); ok {
		t.Fatal("swapped counts compared equal")
	}
	if ok, _ := compareRuns(runResult{Stdout: "x", Stderr: "grep: a/b: No such file"}, runResult{Stdout: "x"}); ok {
		t.Fatal("stderr findings ignored")
	}
}

// A: an install without git history (an installer-made release tree)
// still recognizes an unmodified old copy, from the embedded manifest.
func TestReleasedBlobsWithoutGitHistory(t *testing.T) {
	install := installRoot(t)
	cur := installScript(t, install, "inbox-watch.sh")
	var old string
	for h := range manifestBlobs()["inbox-watch.sh"] {
		if h == gitBlobHash([]byte(cur)) {
			continue
		}
		if out, err := testutil.TryGit(install, "cat-file", "blob", h); err == nil {
			old = out + "\n"
			if gitBlobHash([]byte(old)) == h {
				break
			}
			old = ""
		}
	}
	if old == "" {
		t.Skip("no older inbox-watch.sh blob in this checkout's history")
	}
	tree := t.TempDir()
	testutil.Run(t, install, "cp", "-R", filepath.Join(install, "scripts"), filepath.Join(tree, "scripts"))
	testutil.Run(t, install, "cp", "-R", filepath.Join(install, "templates"), filepath.Join(tree, "templates"))

	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
	testutil.WriteExecutable(t, p, "scripts/inbox-watch.sh", old)
	commitAll(t, p)
	plan, err := BuildPlan(opts(p, tree))
	if err != nil {
		t.Fatal(err)
	}
	if sp := scriptPlan(plan, "inbox-watch.sh"); sp.Verdict != Delete {
		t.Fatalf("unmodified old copy not recognized without git history:\n%s", render(plan))
	}
}

// A: every script blob at a local release tag is in the manifest (fails
// when a release is tagged without running scripts/gen-released-blobs.sh).
func TestReleasedManifestCoversLocalTags(t *testing.T) {
	install := installRoot(t)
	tags, err := testutil.TryGit(install, "tag", "--list", "v*")
	if err != nil || strings.TrimSpace(tags) == "" {
		t.Skip("no release tags in this checkout")
	}
	have := manifestBlobs()
	for _, tag := range strings.Fields(tags) {
		out, err := testutil.TryGit(install, "ls-tree", "-r", tag, "--", "scripts", "templates/project-bootstrap/scripts")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			f := strings.Fields(line)
			if len(f) != 4 {
				continue
			}
			path := f[3]
			name := filepath.Base(path)
			dir := filepath.Dir(path)
			if (dir != "scripts" && dir != "templates/project-bootstrap/scripts") || !(strings.HasSuffix(name, ".sh") || name == "README.md") {
				continue
			}
			if !have[name][f[2]] {
				t.Fatalf("%s:%s (%s) missing from released-blobs.txt; run scripts/gen-released-blobs.sh", tag, path, f[2])
			}
		}
	}
}

// B: a deleted copy with a CONTRACT: header changes the registry (the
// header shows up as a shadow contract); apply regenerates it and says
// why instead of reporting a regression.
func TestDeletionThatChangesTheRegistry(t *testing.T) {
	install := installRoot(t)
	steward := installScript(t, install, "steward.sh")
	if !strings.Contains(steward, "CONTRACT"+":S1-STEWARD") {
		t.Skip("install steward.sh has no CONTRACT header")
	}
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
	testutil.WriteFile(t, p, "architecture/CONTRACT-C1-STORE.1.0.md", "# Store\n\n**Status:** active\n\n## Purpose\n\nStores.\n")
	testutil.WriteExecutable(t, p, "scripts/steward.sh", steward)
	testutil.WriteExecutable(t, p, "scripts/compute-registry.sh", installScript(t, install, "compute-registry.sh"))
	commitAll(t, p) // the registry searches tracked files
	testutil.Run(t, p, "bash", "scripts/compute-registry.sh")
	commitAll(t, p)
	reg, _ := os.ReadFile(filepath.Join(p, "architecture", "CONTRACT-REGISTRY.md"))
	if !strings.Contains(string(reg), "S1-STEWARD") {
		t.Skip("registry does not list the steward header in this version")
	}

	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RegenRegistry || !strings.Contains(render(plan), "CONTRACT-REGISTRY.md: would be regenerated because removing the copies changes it") {
		t.Fatalf("registry change not planned:\n%s", render(plan))
	}
	res, err := Apply(plan, opts(p, install), ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RolledBack {
		t.Fatalf("apply reported its own registry change as a regression: %+v", res.Checks)
	}
	reg, _ = os.ReadFile(filepath.Join(p, "architecture", "CONTRACT-REGISTRY.md"))
	if strings.Contains(string(reg), "S1-STEWARD") {
		t.Fatal("registry not regenerated after the deletion")
	}
}

// C: a kept copy keeps another only when it runs it, not when a comment or
// a message names it.
func TestDependenciesIgnoreComments(t *testing.T) {
	install := installRoot(t)
	for _, tc := range []struct {
		body     string
		wantKeep bool
	}{
		{"#!/usr/bin/env bash\n# see check-registry.sh for the registry\necho \"run scripts/check-registry.sh\"\necho custom\n", false},
		{"#!/usr/bin/env bash\n\"$(dirname \"$0\")/check-registry.sh\" # registry first\necho custom\n", true},
	} {
		p := testutil.InitRepo(t)
		testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
		testutil.WriteExecutable(t, p, "scripts/scan-consumers.sh", tc.body)
		testutil.WriteExecutable(t, p, "scripts/check-registry.sh", installScript(t, install, "check-registry.sh"))
		commitAll(t, p)
		plan, err := BuildPlan(opts(p, install))
		if err != nil {
			t.Fatal(err)
		}
		if got := scriptPlan(plan, "check-registry.sh").Verdict == Keep; got != tc.wantKeep {
			t.Fatalf("kept=%v, want %v for:\n%s\n%s", got, tc.wantKeep, tc.body, render(plan))
		}
	}
}

// D: the integrity note says what `rebar commit` does with deleted files.
func TestIntegrityNote(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	testutil.WriteFile(t, p, ".rebar/integrity.json", `{"checksums":{"scripts":{"scripts/check-ground-truth.sh":"x"}}}`)
	commitAll(t, p)
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render(plan), "stage the changes (`git add -A`) and run `rebar commit`, which rehashes the changed files and prunes the entries of deleted ones") {
		t.Fatalf("integrity note:\n%s", render(plan))
	}
}

// v2CheckTodos is a released check-todos.sh that hard-codes its
// directories and --include list (the v2 template), from the embedded
// released sources.
func v2CheckTodos(t *testing.T) string {
	t.Helper()
	for _, h := range sortedKeys(manifestBlobs()["check-todos.sh"]) {
		src := string(releasedSource("", h))
		if strings.Contains(src, "for d in src internal cmd client packages lib app; do") && !strings.Contains(src, "_rebar-config") {
			return src
		}
	}
	t.Skip("no v2 check-todos.sh among the released sources")
	return ""
}

func TestApplySettings(t *testing.T) {
	src := v2CheckTodos(t)
	got := applySettings(src, map[string]string{"source_dirs": "lib tools", "source_extensions": "go sh"})
	if !strings.Contains(got, "for d in lib tools; do") {
		t.Fatalf("directories not applied:\n%s", got)
	}
	if strings.Contains(got, `--include="*.ts"`) || strings.Count(got, `--include="*.go" --include="*.sh"`) != 2 {
		t.Fatalf("extensions not applied:\n%s", got)
	}
	if out, err := exec.Command("bash", "-n", testutil.WriteExecutable(t, t.TempDir(), "x.sh", got)).CombinedOutput(); err != nil {
		t.Fatalf("not valid bash: %v\n%s", err, out)
	}
}

// A copy that differs from a released version only in its settings (and
// wording) is that version plus settings: it goes, with the behaviour
// change on this project reported (#19 excludes directories by name, so a
// TODO in lib/distance.go, dropped by the copy's `grep -v dist`, now counts).
func TestCustomizedOnlyInSettings(t *testing.T) {
	install := installRoot(t)
	src := v2CheckTodos(t)
	copySrc := strings.ReplaceAll(src, "for d in src internal cmd client packages lib app; do", "for directory in lib tools; do")
	copySrc = strings.ReplaceAll(copySrc, `[ -d "$d" ] && DIRS+=("$d")`, `[ -d "$directory" ] && DIRS+=("$directory")`)
	copySrc = strings.ReplaceAll(copySrc, `--include="*.jsx" \`, `--include="*.jsx" --include="*.sh" \`)
	copySrc = strings.Replace(copySrc, "# check-todos.sh", "# check-todos.sh — adapted for this repo;", 1)
	if copySrc == src || !strings.Contains(copySrc, `--include="*.sh"`) {
		t.Skip("released v2 check-todos.sh changed shape")
	}
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
	testutil.WriteFile(t, p, "lib/a.go", "package lib\n")
	testutil.WriteFile(t, p, "tools/x.sh", "#!/bin/sh\n")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh", copySrc)
	commitAll(t, p)

	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	sp := scriptPlan(plan, "check-todos.sh")
	if sp.Verdict != Delete || !strings.Contains(sp.Reason, "customized only in settings") {
		t.Fatalf("settings-only copy not recognized:\n%s", render(plan))
	}
	for _, want := range []string{"+ source_dirs       = lib tools", "+ source_extensions = go ts tsx js py rs jsx sh", "impact on this project: none"} {
		if !strings.Contains(render(plan), want) {
			t.Fatalf("plan lacks %q:\n%s", want, render(plan))
		}
	}
	if _, err := Apply(plan, opts(p, install), ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	if isFile(filepath.Join(p, "scripts", "check-todos.sh")) {
		t.Fatal("settings-only copy not deleted")
	}

	// With a TODO the copy's substring filter hid, the plan lists it, and
	// apply rolls back because the check would start failing.
	p2 := testutil.InitRepo(t)
	testutil.WriteFile(t, p2, ".rebarrc", "tier = 3\n")
	testutil.WriteFile(t, p2, "lib/distance.go", "package lib // TODO"+": measure\n")
	testutil.WriteExecutable(t, p2, "scripts/check-todos.sh", copySrc)
	commitAll(t, p2)
	plan, err = BuildPlan(opts(p2, install))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render(plan), "impact on this project: the install reports 1 finding(s) the copy did not (upstream change (#19)") ||
		!strings.Contains(render(plan), "lib/distance.go:1") {
		t.Fatalf("impact not reported:\n%s", render(plan))
	}
	res, err := Apply(plan, opts(p2, install), ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.RolledBack || !isFile(filepath.Join(p2, "scripts", "check-todos.sh")) {
		t.Fatalf("passing→failing change not rolled back: %+v", res.Checks)
	}
}

// A real customization stays, and when the only difference on the probe
// tree is what its substring filter drops, the reason says so.
func TestSubstringFilterHint(t *testing.T) {
	install := installRoot(t)
	src := v2CheckTodos(t)
	// A real customization: an extra counter line in the output.
	copySrc := strings.Replace(src, `echo "OK: 0 untracked TODOs.`, `echo "scanned: ${#DIRS[@]} dirs"; echo "OK: 0 untracked TODOs.`, 1)
	if copySrc == src {
		t.Skip("released v2 check-todos.sh changed shape")
	}
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 3\n")
	testutil.WriteFile(t, p, "src/a.go", "package a\n")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh", copySrc)
	commitAll(t, p)
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	sp := scriptPlan(plan, "check-todos.sh")
	if sp.Verdict != Keep {
		t.Fatalf("real customization deleted:\n%s", render(plan))
	}
	if !strings.Contains(strings.Join(sp.Details, "\n"), "the copy's substring filters") {
		t.Fatalf("no substring hint:\n%s", render(plan))
	}
}

// Old substring filters map to source_exclude_dirs only for words that
// name directories; the install default needs no setting.
func TestExcludedDirNames(t *testing.T) {
	src := "grep -v \"TRACKED-TASK:\" | grep -v \"TODO\\.md\" | grep -v \"check-todos\" | grep -v \"node_modules\\|vendor\\|dist\\|build\\|\\.git\"\n" +
		"grep -rn x --exclude-dir=\"third_party\" .\n"
	if got := strings.Join(excludedDirNames("check-todos.sh", src), " "); got != "third_party node_modules vendor dist build" {
		t.Fatalf("excludedDirNames = %q", got)
	}
	if !sameSetting("source_exclude_dirs", "dist vendor node_modules", installDefaults["source_exclude_dirs"]) {
		t.Fatal("the default set in another order must need no setting")
	}
}
