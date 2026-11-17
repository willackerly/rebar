package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Parity of the git gates (runParity), check-fix-commit, check-bypass-flags
// and check-version-bump, with their goldens in testdata/<check>
// (regenerate with go test ./cmd -run 'Gate|VersionBump' -update-golden).
// Commits are made at a fixed date with a fixed identity under a
// throwaway git config, so their SHAs, which the output names, are the
// same everywhere.

// fixedGit isolates git from the developer's configuration and pins the
// commit dates, for the setup and for every run of the check. Once a test
// has called it, calls from its (parallel) subtests change nothing:
// runParity calls it before the cases when any has a setup.
func fixedGit(t *testing.T) {
	t.Helper()
	if os.Getenv("GIT_COMMITTER_DATE") == fixedDate && os.Getenv("GIT_CONFIG_NOSYSTEM") == "1" {
		return
	}
	testutil.IsolateGitConfig(t)
	t.Setenv("GIT_AUTHOR_DATE", fixedDate)
	t.Setenv("GIT_COMMITTER_DATE", fixedDate)
}

const fixedDate = "2026-01-02T03:04:05Z"

// commitMsg commits everything in root (allowing an empty commit) with
// message msg exactly as given.
func commitMsg(t *testing.T, root, msg string) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(f, []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "--allow-empty", "--allow-empty-message", "--cleanup=verbatim", "-F", f)
}

// commits makes root a repository (when it is not one) holding one commit
// per message, in order.
func commits(msgs ...string) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		fixedGit(t)
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			testutil.Git(t, root, "init", "-q", "-b", "main")
		}
		for _, m := range msgs {
			commitMsg(t, root, m)
		}
	}
}

// mergeHistory is main with one commit, a side branch with side, main
// with another, then a --no-ff merge of side whose message is merge.
func mergeHistory(side, merge string) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		commits("feat: base")(t, root)
		testutil.Git(t, root, "checkout", "-q", "-b", "side")
		testutil.WriteFile(t, root, "side.txt", "side\n")
		commitMsg(t, root, side)
		testutil.Git(t, root, "checkout", "-q", "main")
		testutil.WriteFile(t, root, "main.txt", "main\n")
		commitMsg(t, root, "feat: main moves on")
		f := filepath.Join(t.TempDir(), "msg")
		testutil.WriteFile(t, filepath.Dir(f), "msg", merge)
		testutil.Git(t, root, "merge", "-q", "--no-ff", "--cleanup=verbatim", "-F", f, "side")
	}
}

// shallowClone makes root a --depth 2 clone of a repository holding one
// commit per message.
func shallowClone(msgs ...string) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		src := t.TempDir()
		commits(msgs...)(t, src)
		testutil.Git(t, filepath.Dir(root), "clone", "-q", "--depth", "2", "file://"+src, root)
	}
}

var fixCommitCheck = parityCheck{name: "check-fix-commit", golden: "check-fix-commit"}

var fixCommitParityCases = []parityCase{
	{name: "head: not a fix", setup: commits("feat: add the thing\n\nbody\n")},
	{name: "head: fix without reproduced on", setup: commits("fix: crash on start\n\nThe app crashed.\n")},
	{name: "head: fix(scope) with reproduced on", setup: commits("fix(cli): crash on start\n\nReproduced on: abc1234 (local)\n")},
	{name: "head: regression without evidence", setup: commits("regression(api): slow\n\nReproduced on:   \n")},
	{name: "head: regression with evidence", setup: commits("regression: slow\n\nReproduced on: https://example.com/log/1\n")},
	{name: "head: reproduced on indented", setup: commits("fix: x\n\n  Reproduced on: y\n")},
	{name: "head: reproduced on followed by a vertical tab", setup: commits("fix: x\n\nReproduced on: \v\n")},
	{name: "head: reproduced on with a tab", setup: commits("fix: x\n\nReproduced\ton:\tlogs/run.txt\n")},
	{name: "head: no space after the colon", setup: commits("fix:x\n")},
	{name: "head: capitalised Fix", setup: commits("Fix: x\n")},
	{name: "head: fixup is not fix", setup: commits("fixup: x\n")},
	{name: "head: scope with a parenthesis", setup: commits("fix(a(b)): x\n")},
	{name: "head: multi-line subject", setup: commits("fix: first\nsecond line\n\nbody\n")},
	{name: "head: non-ascii", setup: commits("fix: café ✓ ünïcode\n\nReproduced on: журнал ✓\n")},
	{name: "head: non-ascii violation", setup: commits("fix(ü): naïve ✓\n\nкороче\n")},
	{name: "head: escapes are literal", setup: commits("fix: 100% \\n \\t -e $HOME `x` 'q' \"d\"\n\n\\nReproduced on: x\n")},
	{name: "head: empty message", setup: commits("")},
	{name: "head: crlf message", setup: commits("fix: crlf\r\n\r\nReproduced on: x\r\n")},
	{name: "head: crlf empty evidence", setup: commits("fix: crlf\r\n\r\nReproduced on:\r\n")},
	{name: "head: empty repository", git: true},
	{name: "head: not a repository"},
	{name: "warn: violation does not fail", args: []string{"--warn"}, setup: commits("fix: x\n")},
	{
		name:  "file: commit-msg with comments",
		files: map[string]string{"msg.txt": "# Please enter the commit message\n\nfix: from the hook\n# Reproduced on: commented out\n"},
		args:  []string{"msg.txt"},
	},
	{
		name:  "file: satisfied",
		files: map[string]string{"msg.txt": "\n\nregression(x): y\n\nReproduced on: deploy 42\n"},
		args:  []string{"msg.txt"},
	},
	{
		name:  "file: crlf",
		files: map[string]string{"msg.txt": "fix: crlf\r\n\r\nbody\r\n"},
		args:  []string{"msg.txt", "--warn"},
	},
	{
		name:  "file: blank line of spaces is a subject",
		files: map[string]string{"msg.txt": "   \nfix: second\n"},
		args:  []string{"msg.txt"},
	},
	{
		name:  "file: no trailing newline",
		files: map[string]string{"msg.txt": "fix: x\n\nReproduced on: y"},
		args:  []string{"msg.txt"},
	},
	{name: "file: empty file", files: map[string]string{"msg.txt": ""}, args: []string{"msg.txt"}},
	{name: "file: missing", args: []string{"nope.txt"}},
	{name: "file: empty argument", args: []string{""}},
	{name: "file: a directory", files: map[string]string{"d/x": ""}, args: []string{"d"}},
	{
		name:  "file: the last operand wins",
		files: map[string]string{"a.txt": "fix: a\n", "b.txt": "feat: b\n"},
		args:  []string{"a.txt", "b.txt"},
	},
	{
		name:  "file: an operand after --range wins",
		files: map[string]string{"a.txt": "fix: a\n"},
		args:  []string{"--range", "HEAD", "a.txt"},
		setup: commits("feat: x\n"),
	},
	{name: "flag: unknown", args: []string{"-x"}},
	{name: "flag: lone dash", args: []string{"-"}},
	{name: "flag: help", args: []string{"--help"}},
	{name: "flag: help after an operand", args: []string{"msg.txt", "-h"}},
	{
		name:  "range: violations in a range",
		args:  []string{"--range", "HEAD~3..HEAD"},
		setup: commits("feat: one\n", "fix: two\n", "fix(x): three\n\nReproduced on: y\n", "regression: four\n"),
	},
	{
		name:  "range: warn",
		args:  []string{"--warn", "--range", "main"},
		setup: commits("fix: one\n", "fix: two\n"),
	},
	{name: "range: merge commits", args: []string{"--range", "main"}, setup: mergeHistory("fix: on the side\n", "fix: merge side\n")},
	{name: "range: bad revision", args: []string{"--range", "nope..HEAD"}, setup: commits("fix: x\n")},
	{name: "range: empty repository", args: []string{"--range", "HEAD"}, git: true},
	{name: "range: other rev-list arguments", args: []string{"--range", "--all"}, setup: commits("fix: x\n")},
	{name: "range: shallow clone, beyond the boundary", args: []string{"--range", "HEAD~3..HEAD"}, setup: shallowClone("fix: 1\n", "fix: 2\n", "fix: 3\n", "fix: 4\n")},
	{name: "range: shallow clone, the fetched commits", args: []string{"--range", "HEAD"}, setup: shallowClone("fix: 1\n", "fix: 2\n", "fix: 3\n", "fix: 4\n")},
	{
		name: "range: without a range",
		args: []string{"--range"},
		// bash dies on `set -u` with "<script>: line N: $2: unbound variable".
		divergesOn: "all",
		want:       "check-fix-commit: --range needs a revision range (<sha>..<sha>)\n",
		wantExit:   1,
	},
}

func TestFixCommitParity(t *testing.T) { runParity(t, fixCommitCheck, fixCommitParityCases) }
func TestFixCommitGolden(t *testing.T) { runGolden(t, fixCommitCheck, fixCommitParityCases) }

var bypassFlagsCheck = parityCheck{name: "check-bypass-flags", golden: "check-bypass-flags"}

var bypassFlagsParityCases = []parityCase{
	{name: "head: no bypass", setup: commits("feat: x\n\nbody\n")},
	{name: "head: no-verify without tickets", setup: commits("chore: ship\n\nPushed with git push --no-verify because CI was down.\n")},
	{name: "head: tickets", setup: commits("chore: ship\n\nUsed --skip-tests.\nBypass tickets: T-1, T-2\n")},
	{name: "head: empty tickets", setup: commits("chore: ship\n\nUsed --skip-e2e.\nBypass tickets:\n")},
	{name: "head: meta opt-out", setup: commits("docs: bypass flags\n\nThe gate catches --skip-stress / --no-verify.\nBypass-flags-meta: documentation\n")},
	{name: "head: empty meta opt-out", setup: commits("docs: x\n\n--no-verify\nBypass-flags-meta: \n")},
	{name: "head: in the subject", setup: commits("chore: deploy --force\n")},
	{name: "head: force at the end of a line", setup: commits("chore: x\n\nran deploy --force\n")},
	{name: "head: force with a tab", setup: commits("chore: x\n\nran deploy --force\tnow\n")},
	{name: "head: forced and force-with-lease are not force", setup: commits("chore: x\n\n--forced --force-with-lease\n")},
	{name: "head: more than three matches", setup: commits("chore: x\n\n--skip-ci --skip-build --no-gpg-sign SKIP_TESTS=1\nSKIP_E2E=1 SKIP_STRESS=1 BYPASS_=x\n")},
	{name: "head: every skip flag", setup: commits("chore: x\n\n--skip-stress --skip-tests --skip-all --skip-e2e --skip-smoke --skip-ci --skip-build --skip-fix --skip-preflight\n")},
	{name: "head: case matters", setup: commits("chore: x\n\n--No-Verify skip_tests=1 --SKIP-CI\n")},
	{name: "head: indented tickets do not count", setup: commits("chore: x\n\n--no-verify\n  Bypass tickets: T-1\n")},
	{name: "head: non-ascii", setup: commits("chore: ünïcode ✓\n\n— --skip-smoke — café\n")},
	{name: "head: crlf", setup: commits("chore: crlf\r\n\r\nran --force\r\nBypass tickets:\r\n")},
	{name: "head: empty repository", git: true},
	{name: "warn", args: []string{"--warn"}, setup: commits("chore: x\n\n--no-verify\n")},
	{
		name:  "file: commit-msg",
		files: map[string]string{"COMMIT_EDITMSG": "# comment\nchore: from the hook\n\n--skip-tests\n"},
		args:  []string{"COMMIT_EDITMSG"},
	},
	{
		name:  "file: commented tickets do not count",
		files: map[string]string{"m": "chore: x\n\n--no-verify\n# Bypass tickets: T-1\n"},
		args:  []string{"m"},
	},
	{name: "file: missing", args: []string{"nope"}},
	{name: "flag: unknown", args: []string{"--nope"}},
	{name: "flag: help", args: []string{"-h"}},
	{name: "range: merge commits", args: []string{"--range", "main"}, setup: mergeHistory("chore: side --no-verify\n", "Merge side with --force \n")},
	{name: "range: some satisfied", args: []string{"--range", "HEAD~2..HEAD"}, setup: commits("feat: a\n", "chore: b --skip-ci\n", "chore: c --skip-ci\n\nBypass tickets: X-1\n")},
	{name: "range: shallow clone", args: []string{"--range", "HEAD~3..HEAD"}, setup: shallowClone("chore: --force \n", "chore: 2\n", "chore: --no-verify\n")},
	{
		name:       "range: without a range",
		args:       []string{"--warn", "--range"},
		divergesOn: "all",
		want:       "check-bypass-flags: --range needs a revision range (<sha>..<sha>)\n",
		wantExit:   1,
	},
}

func TestBypassFlagsParity(t *testing.T) { runParity(t, bypassFlagsCheck, bypassFlagsParityCases) }
func TestBypassFlagsGolden(t *testing.T) { runGolden(t, bypassFlagsCheck, bypassFlagsParityCases) }

// detectedAt is the timestamp check-version-bump writes.
var detectedAt = regexp.MustCompile(`(- \*\*detected:\*\* )[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

var versionBumpCheck = parityCheck{
	name:      "check-version-bump",
	golden:    "check-version-bump",
	writes:    true,
	normalize: func(out string) string { return detectedAt.ReplaceAllString(out, "${1}<now>") },
}

// renameCommit commits the files of the tree, renames each from → to with
// git mv, and commits again.
func renameCommit(renames ...string) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		commits("contracts\n")(t, root)
		for i := 0; i+1 < len(renames); i += 2 {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, renames[i+1])), 0o755); err != nil {
				t.Fatal(err)
			}
			testutil.Git(t, root, "mv", renames[i], renames[i+1])
		}
		commitMsg(t, root, "bump\n")
	}
}

const bumpContract = "# Contract\n\n**Status:** active\n\n## Purpose\n\nA contract body long enough for git to see a rename.\nline\nline\nline\n"

var versionBumpParityCases = []parityCase{
	{name: "not a repository"},
	{name: "empty repository", git: true},
	{name: "root commit", files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract}, setup: commits("init\n")},
	{
		name:  "major bump",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1-FOO.1.0.md", "architecture/CONTRACT-S1-FOO.2.0.md"),
	},
	{
		name:  "three-part versions",
		files: map[string]string{"architecture/CONTRACT-C1-AGENTS.1.2.3.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-C1-AGENTS.1.2.3.md", "architecture/CONTRACT-C1-AGENTS.1.3.0.md"),
	},
	{
		name:  "dotted id",
		files: map[string]string{"architecture/CONTRACT-S1.FOO.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1.FOO.1.0.md", "architecture/CONTRACT-S1.FOO.1.1.md"),
	},
	{
		name:  "renamed to another contract",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1-FOO.1.0.md", "architecture/CONTRACT-S1-BAR.1.0.md"),
	},
	{
		name:  "unversioned names",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1-FOO.md", "architecture/CONTRACT-S1-FOO.v2.md"),
	},
	{
		name: "two bumps and an existing outbox",
		files: map[string]string{
			"architecture/CONTRACT-S1-FOO.1.0.md":             bumpContract,
			"architecture/CONTRACT-S2-BAR.2.0.md":             bumpContract + "bar\n",
			"architecture/.state/pending-notifications.md":    "# Pending\n\n## OLD: 1.0 → 2.0\n- **status:** pending\n\n## GONE: 1.0 → 1.1\n- **status:** sent\n",
			"architecture/CONTRACT-S3-BAZ.1.0.md":             bumpContract + "baz\n",
			"architecture/CONTRACT-S3-BAZ.1.0.impl.md":        "companion\n",
			"docs/CONTRACT-S9-ELSEWHERE.1.0.md":               bumpContract + "elsewhere\n",
			"architecture/sub/CONTRACT-S8-NESTED.1.0.md.orig": bumpContract + "nested\n",
		},
		setup: renameCommit(
			"architecture/CONTRACT-S1-FOO.1.0.md", "architecture/CONTRACT-S1-FOO.2.0.md",
			"architecture/CONTRACT-S2-BAR.2.0.md", "architecture/CONTRACT-S2-BAR.2.1.md",
			"docs/CONTRACT-S9-ELSEWHERE.1.0.md", "docs/CONTRACT-S9-ELSEWHERE.2.0.md",
		),
	},
	{
		name: "contracts_dir and an absolute outbox",
		files: map[string]string{
			".rebarrc":                            "contracts_dir = ./docs/specs/\n",
			"docs/specs/CONTRACT-S1-FOO.1.0.md":   bumpContract,
			"architecture/CONTRACT-S2-BAR.1.0.md": bumpContract + "bar\n",
		},
		env: []string{"REBAR_OUTBOX=$ROOT/out/notes.md"},
		setup: renameCommit(
			"docs/specs/CONTRACT-S1-FOO.1.0.md", "docs/specs/CONTRACT-S1-FOO.2.0.md",
			"architecture/CONTRACT-S2-BAR.1.0.md", "architecture/CONTRACT-S2-BAR.2.0.md",
		),
	},
	{
		name:  "relative outbox",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract},
		env:   []string{"REBAR_OUTBOX=notes/outbox.md"},
		setup: renameCommit("architecture/CONTRACT-S1-FOO.1.0.md", "architecture/CONTRACT-S1-FOO.1.1.md"),
	},
	{
		name:  "a space in the name",
		files: map[string]string{"architecture/CONTRACT-S1 FOO.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1 FOO.1.0.md", "architecture/CONTRACT-S1 FOO.2.0.md"),
	},
	{
		name:  "a non-ascii name",
		files: map[string]string{"architecture/CONTRACT-S1-CAFÉ.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1-CAFÉ.1.0.md", "architecture/CONTRACT-S1-CAFÉ.2.0.md"),
	},
	{
		name:  "moved out of the contracts directory",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract},
		setup: renameCommit("architecture/CONTRACT-S1-FOO.1.0.md", "archive/CONTRACT-S1-FOO.2.0.md"),
	},
	{
		name:  "merge commit at HEAD",
		files: map[string]string{"architecture/CONTRACT-S1-FOO.1.0.md": bumpContract},
		setup: func(t *testing.T, root string) {
			mergeHistory("feat: side\n", "Merge side\n")(t, root)
		},
	},
}

func TestVersionBumpParity(t *testing.T) { runParity(t, versionBumpCheck, versionBumpParityCases) }
func TestVersionBumpGolden(t *testing.T) { runGolden(t, versionBumpCheck, versionBumpParityCases) }
