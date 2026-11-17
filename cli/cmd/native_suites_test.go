package cmd

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The suites: ci-check, pre-commit and cold-start-checks, with goldens in
// testdata/<suite> (captured from the suite scripts; regenerate with
// go test ./cmd -run 'CICheck|PreCommit|ColdStart' -update-golden).

// suiteProject is a small adopter project at tier 1 whose checks pass.
var suiteProject = map[string]string{
	".rebarrc":               "tier = 1\n",
	".rebar-version":         "v3.1.0\n",
	"README.md":              "# Proj\n\n> **rebar v3.1.0** | **Tier 1: PARTIAL**\n\nSee [the guide](docs/guide.md).\n",
	"AGENTS.md":              "# Agents\n",
	"docs/guide.md":          "<!-- freshness: {{ago:3}} -->\n# Guide\n",
	"src/a.go":               "package a\n// @tracked later\n",
	"architecture/README.md": "# Architecture\n",
}

// suiteBroken trips the fast checks: an untracked TODO, a reference to a
// contract that does not exist, a stale doc.
var suiteBroken = withFiles(suiteProject, map[string]string{
	"src/b.go":      "package b\n// @todo untracked\n// @contract:S9-GONE.1.0\n",
	"docs/old.md":   "<!-- freshness: {{ago:40}} -->\n# Old\n",
	".rebarrc":      "tier = 2\n",
	"README.md":     "# Proj\n\n> **rebar v3.1.0** | **Tier 2: ADOPTED**\n\nSee [the guide](docs/guide.md) and [gone](docs/gone.md).\n",
	"docs/guide.md": "<!-- freshness: {{ago:3}} -->\n# Guide\n",
})

// suiteContracts is a tier 3 project with contracts, a registry and a
// metric that drifted.
var suiteContracts = withFiles(suiteProject, map[string]string{
	".rebarrc":                            "tier = 3\nsource_dirs = src\n",
	"README.md":                           "# Proj\n\n> **rebar v3.1.0** | **Tier 3: ENFORCED**\n",
	"architecture/CONTRACT-S1-FOO.1.0.md": "# S1-FOO\n\n**Status:** active\n\n## Why this exists\n\nx\n\n## Who needs this\n\nUs.\n\n## Scenarios\n\nOne.\n",
	"architecture/CONTRACT-S2-BAR.1.0.md": "# S2-BAR\n\n**Status:** draft\n\n## Purpose\n\ny\n",
	"architecture/CONTRACT-REGISTRY.md":   "stale\n",
	"METRICS":                             "go_files = 7\n",
	"METRICS.md":                          "| go_files | `find src -name '*.go' \\| wc -l` |\n",
	"src/a.go":                            "// @contract:S1-FOO.1.0\npackage a\n",
})

// committed makes the tree's first commit (stable SHAs: fixedGit).
var committed = commits("feat: initial commit")

// withRegistry generates the contract registry, then commits.
func withRegistry(t *testing.T, root string) {
	t.Helper()
	c, _ := checks.Lookup("compute-registry")
	var out strings.Builder
	if code := c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, nil); code != 0 {
		t.Fatalf("compute-registry: exit %d\n%s", code, out.String())
	}
	committed(t, root)
}

// suiteSkipAll skips every check of the full suite.
var suiteSkipAll = []string{
	"SKIP_CONTRACT_HEADERS=1", "SKIP_CONTRACT_REFS=1", "SKIP_DOC_REFS=1", "SKIP_TODOS=1", "SKIP_FRESHNESS=1",
	"SKIP_REGISTRY=1", "SKIP_JTBD=1", "SKIP_PREFIX_UNIQUENESS=1", "SKIP_GROUND_TRUTH=1", "SKIP_COMPLIANCE=1",
	"SKIP_DECAY_PATTERNS=1", "SKIP_BOOTSTRAP_SYNC=1", "SKIP_SCRIPT_TESTS=1", "SKIP_FIX_COMMIT=1",
	"SKIP_BYPASS_FLAGS=1", "SKIP_STEWARD=1",
}

// noSteward skips the steward scan, which writes reports.
var noSteward = []string{"SKIP_STEWARD=1"}

// projectCopies adds the project's own scripts: a customized executable
// check-todos.sh, a thin-wrapper check-freshness.sh (never picked) and a
// copy of check-doc-refs.sh that is not executable.
func projectCopies(t *testing.T, root string) {
	testutil.WriteExecutable(t, root, "scripts/check-todos.sh", "#!/usr/bin/env bash\necho 'custom todos'\necho 'OK: custom'\n")
	testutil.WriteExecutable(t, root, "scripts/check-freshness.sh", "#!/usr/bin/env bash\n# rebar-thin-wrapper\necho 'thin wrapper ran'; exit 1\n")
	testutil.WriteFile(t, root, "scripts/check-doc-refs.sh", "#!/usr/bin/env bash\necho 'not executable'\n")
	committed(t, root)
}

var ciCheckCases = []goldenCase{
	{name: "clean tier 1 project", files: suiteProject, git: true, setup: withRegistry, env: noSteward},
	{name: "failures fail by default", files: suiteBroken, git: true, setup: committed, env: noSteward},
	{name: "failures with --no-strict pass", files: suiteBroken, git: true, setup: committed, env: noSteward, args: []string{"--no-strict"}},
	{name: "an empty first argument is strict", files: suiteBroken, git: true, setup: committed, env: noSteward, args: []string{""}},
	{name: "explicit --strict, later arguments ignored", files: suiteBroken, git: true, setup: committed, env: noSteward, args: []string{"--strict", "--no-strict"}},
	{name: "any other first argument is not strict", files: suiteBroken, git: true, setup: committed, env: noSteward, args: []string{"strict"}},
	{name: "every check skipped", files: suiteBroken, git: true, setup: committed, env: suiteSkipAll},
	{
		name:  "skip values other than 1 do not skip",
		files: suiteBroken, git: true, setup: committed,
		env: []string{"SKIP_STEWARD=1", "SKIP_TODOS=true", "SKIP_FRESHNESS=0", "SKIP_CONTRACT_REFS=1", "SKIP_DOC_REFS= 1", "SKIP_BOOTSTRAP_SYNC=1", "SKIP_SCRIPT_TESTS=1"},
	},
	{name: "tier 3 contracts with drift", files: suiteContracts, git: true, setup: committed, env: noSteward},
	{name: "tier 3 contracts, ground truth skipped", files: suiteContracts, git: true, setup: committed, env: []string{"SKIP_STEWARD=1", "SKIP_GROUND_TRUTH=1", "SKIP_REGISTRY=1"}},
	{name: "fix commit without a reproduction", files: suiteProject, git: true, setup: commits("feat: init", "fix: the widget"), env: noSteward},
	{name: "project copies of checks", files: suiteProject, git: true, setup: projectCopies, env: noSteward},
	{
		name:  "checks_enable and contract_namespace warn once",
		files: withFiles(suiteProject, map[string]string{".rebarrc": "tier = 1\nchecks_enable = check-nope check-ground-truth.sh\ncontract_namespace = S1-BAD.1.0\n"}),
		git:   true, setup: committed, env: noSteward,
	},
	// The first run writes the steward's reports, which the checks of the
	// next run would see: the tree ignores them (as the contract has it) and
	// has a STEWARD_REPORT.md from the start, which check-freshness counts.
	{
		name: "steward runs last",
		files: withFiles(suiteContracts, map[string]string{
			"STEWARD_REPORT.md": "<!-- freshness: {{ago:0}} -->\n",
			".gitignore":        "STEWARD_REPORT.md\narchitecture/.state/\n",
		}),
		git: true, setup: committed,
	},
	{name: "REBAR_TIER overrides .rebarrc", files: suiteBroken, git: true, setup: committed, env: []string{"SKIP_STEWARD=1", "REBAR_TIER=1"}},
}

func TestCICheckGolden(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1") // scripts the native suite starts may hand off to this binary
	runGolden(t, goldenCheck{name: "ci-check", golden: "ci-check"}, ciCheckCases)
}

var preCommitCases = []goldenCase{
	{name: "clean", files: suiteProject, git: true, setup: withRegistry},
	{name: "failures", files: suiteBroken, git: true, setup: committed},
	{name: "ground truth drift at tier 3", files: suiteContracts, git: true, setup: committed},
	{name: "ground truth enabled below its tier", files: withFiles(suiteContracts, map[string]string{".rebarrc": "tier = 1\nchecks_enable = check-ground-truth\n"}), git: true, setup: committed},
	{name: "project copies", files: suiteProject, git: true, setup: projectCopies},
	{
		name:  "a project copy of ground truth that is not executable",
		files: withFiles(suiteProject, map[string]string{"scripts/check-ground-truth.sh": "#!/usr/bin/env bash\necho never\n"}),
		git:   true, setup: committed,
	},
	{name: "unknown checks_enable name", files: withFiles(suiteProject, map[string]string{".rebarrc": "tier = 1\nchecks_enable = nope\n"}), git: true, setup: committed},
	{name: "arguments are ignored", files: suiteBroken, git: true, setup: committed, args: []string{"--no-strict"}},
}

func TestPreCommitGolden(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1")
	runGolden(t, goldenCheck{name: "pre-commit", golden: "pre-commit"}, preCommitCases)
}

// maturityContracts declare every kind of Status: line.
var maturityContracts = withFiles(suiteProject, map[string]string{
	"architecture/CONTRACT-A1-STUB.1.0.md":     "**Status:** stub\n",
	"architecture/CONTRACT-A2-DRAFT.1.0.md":    "Status: Draft\n",
	"architecture/CONTRACT-A3-PROG.1.0.md":     "**Status:**\tIN-PROGRESS (since May)\n",
	"architecture/CONTRACT-A4-ACT.1.0.md":      "# A4\n\nintro\n**Status:** active\n**Status:** stub\n",
	"architecture/CONTRACT-A5-ACT.1.0.md":      "**Status:** Active\r\n",
	"architecture/CONTRACT-A6-VER.1.0.md":      "**Status:** verified\n",
	"architecture/CONTRACT-A7-SUP.1.0.md":      "**Status:** superseded by A8\n",
	"architecture/CONTRACT-A8-ODD.1.0.md":      "**Status:** frozen\n",
	"architecture/CONTRACT-A9-STAR.1.0.md":     "*Status:* active\n",
	"architecture/CONTRACT-B1-NONE.1.0.md":     "# no status\n",
	"architecture/CONTRACT-B2-EMPTY.1.0.md":    "**Status:**   \n",
	"architecture/CONTRACT-B3-INDENT.1.0.md":   "  **Status:** active\n",
	"architecture/CONTRACT-TEMPLATE.md":        "**Status:** stub\n",
	"architecture/CONTRACT-SEAM-TEMPLATE.md":   "**Status:** stub\n",
	"architecture/CONTRACT-REGISTRY.md":        "**Status:** active\n",
	"architecture/CONTRACT-GAPS.md":            "**Status:** active\n",
	"architecture/CONTRACT-A4-ACT.1.0.impl.md": "**Status:** active\n",
	"architecture/CONTRACT-C1-DIR.1.0.md/x":    "a directory\n",
	"architecture/sub/CONTRACT-D1-DEEP.1.0.md": "**Status:** active\n",
	"architecture/contract-lower.md":           "**Status:** active\n",
})

// upstream gives the tree an origin it is n commits ahead of.
func upstream(n int) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		committed(t, root)
		remote := filepath.Join(t.TempDir(), "origin.git")
		testutil.Git(t, root, "clone", "-q", "--bare", root, remote)
		testutil.Git(t, root, "remote", "add", "origin", remote)
		testutil.Git(t, root, "fetch", "-q", "origin")
		testutil.Git(t, root, "branch", "-q", "--set-upstream-to", "origin/main")
		for i := 0; i < n; i++ {
			commitMsg(t, root, "chore: ahead")
		}
	}
}

var coldStartCases = []goldenCase{
	{name: "clean, no contracts", files: suiteProject, git: true, setup: withRegistry},
	{name: "failures", files: suiteBroken, git: true, setup: committed},
	{name: "tier 3 contracts with drift", files: suiteContracts, git: true, setup: committed},
	{name: "maturity of every kind of Status line", files: maturityContracts, git: true, setup: committed},
	{
		name: "pre-v3 contracts",
		files: withFiles(suiteProject, map[string]string{
			"architecture/CONTRACT-S1-OLD.1.0.md": "# old\n",
			"architecture/CONTRACT-TEMPLATE.md":   "**Status:** stub\n",
		}),
		git: true, setup: committed,
	},
	{
		name: "contracts_dir",
		files: withFiles(suiteProject, map[string]string{
			".rebarrc":                          "tier = 1\ncontracts_dir = ./docs/specs/\n",
			"docs/specs/CONTRACT-S1-X.1.0.md":   "**Status:** verified\n",
			"architecture/CONTRACT-S2-Y.1.0.md": "**Status:** draft\n",
		}),
		git: true, setup: committed,
	},
	{name: "unpushed commits", files: suiteProject, git: true, setup: upstream(2)},
	{name: "many unpushed commits", files: suiteProject, git: true, setup: upstream(7)},
	{name: "up to date with upstream", files: suiteProject, git: true, setup: upstream(0)},
	{name: "not a git repository", files: suiteProject},
	{name: "project copies", files: suiteProject, git: true, setup: projectCopies},
	{name: "warnings go to stdout", files: withFiles(suiteProject, map[string]string{".rebarrc": "tier = 1\nchecks_enable = nope\ncontract_namespace = bad ns\n"}), git: true, setup: committed},
}

// elapsed is the cold-start hook's run time, which differs between runs.
var elapsed = regexp.MustCompile(`(passing|failing) \([0-9]+s\)`)

func TestColdStartGolden(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1")
	runGolden(t, goldenCheck{
		name:      "cold-start-checks",
		golden:    "cold-start-checks",
		normalize: func(out string) string { return elapsed.ReplaceAllString(out, "$1 (Ns)") },
	}, coldStartCases)
}

// TestSuitesWithoutInstall runs the CLI (this binary) with no rebar tree to
// find: the suites run every check and skip the rebar-source-only ones as
// such.
func TestSuitesWithoutInstall(t *testing.T) {
	root := writeTree(t, suiteProject, nil)
	testutil.Git(t, root, "init", "-q", "-b", "main")
	withRegistry(t, root)
	bin, home := testBinary(t), t.TempDir()

	out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "check")
	for _, want := range []string{
		"SKIP  Bootstrap Sync (rebar source repo only)",
		"SKIP  Script Tests (rebar source repo only)",
		"PASS  Steward",
		"PASS  TODO Tracking", "14 passed, 0 failed, 2 skipped",
	} {
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("rebar check (exit %d) lacks %q:\n%s", code, want, out)
		}
	}
	if out, code := testutil.RunEnv(t, root, cliEnv(home), bin, "check", "--cold-start"); code != 0 || !strings.Contains(out, "all 4 checks passing") {
		t.Errorf("rebar check --cold-start (exit %d):\n%s", code, out)
	}
}
