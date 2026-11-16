package cmd

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Parity of compute-registry (runParity), and its golden files: every
// case's expected output is kept in testdata/compute-registry, checked
// against the script by the parity test and against the native check by
// TestComputeRegistryGolden, which needs no bash (it outlives the script).
// Regenerate the goldens from the script with
//
//	go test ./cmd -run 'TestComputeRegistry' -update-golden
//
// Contract references in fixture bodies are spelled @contract:.

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden files from the scripts' output")

// gitTree makes root a git work tree with its files staged, then writes
// the untracked files.
func gitTree(t *testing.T, root string, untracked map[string]string, loc *time.Location) {
	t.Helper()
	testutil.Git(t, root, "init", "-q", "-b", "main")
	testutil.Git(t, root, "add", "-A")
	for rel, body := range untracked {
		testutil.WriteFile(t, root, rel, fixtureText(body, loc))
	}
}

// treeState maps every file below root, outside .git, to its contents.
func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		state[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// takeChanges describes the files a run wrote or removed below root since
// before, and puts root back as it was.
func takeChanges(t *testing.T, root string, before map[string]string) string {
	t.Helper()
	after := treeState(t, root)
	var paths []string
	for p := range after {
		if body, ok := before[p]; !ok || body != after[p] {
			paths = append(paths, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		full := filepath.Join(root, p)
		body, existed := before[p]
		if now, ok := after[p]; ok {
			b.WriteString("--- wrote " + p + " ---\n" + now)
		} else {
			b.WriteString("--- removed " + p + " ---\n")
		}
		if existed {
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Remove(full); err != nil {
			t.Fatal(err)
		}
	}
	return b.String()
}

// goldenName is the golden file of a case.
func goldenName(dir, name string) string {
	return filepath.Join("testdata", dir, strings.Trim(nonWord.ReplaceAllString(strings.ToLower(name), "-"), "-")+".golden")
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// goldenText is output as a golden file keeps it: the tree's root spelled
// $ROOT, the exit code on the last line.
func goldenText(out, root string, code int) string {
	return strings.ReplaceAll(out, root, "$ROOT") + "--- exit " + strconv.Itoa(code) + " ---\n"
}

// checkGolden compares out (exit code) with a case's golden file, or
// rewrites it under -update-golden.
func checkGolden(t *testing.T, dir, name, out, root string, code int) {
	t.Helper()
	path := goldenName(dir, name)
	got := goldenText(out, root, code)
	if *updateGolden {
		testutil.WriteFile(t, ".", path, got)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden file (run with -update-golden): %v", err)
	}
	if got != string(want) {
		t.Errorf("differs from %s:\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// runGolden runs the native check in process on every case's tree and
// compares its output, and the files it wrote, with the golden files.
func runGolden(t *testing.T, check parityCheck, cases []parityCase) {
	t.Helper()
	c, ok := checks.Lookup(check.name)
	if !ok {
		t.Fatalf("%s is not registered", check.name)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if *updateGolden {
				t.Skip("goldens are written by the parity test")
			}
			if _, err := os.Stat(goldenName(check.golden, tc.name)); err != nil && tc.pending != "" {
				t.Skipf("pending %q: no golden file yet", tc.pending)
			}
			files := map[string]string{}
			for rel, body := range tc.files {
				files[rel] = fixtureText(body, time.Local)
			}
			root := writeTree(t, files, tc.links)
			if tc.git {
				gitTree(t, root, tc.untracked, time.Local)
			}
			if tc.setup != nil {
				tc.setup(t, root)
			}
			before := treeState(t, root)
			for _, k := range scriptSettings {
				t.Setenv(k, "")
			}
			for _, kv := range tc.env {
				k, v, _ := strings.Cut(strings.ReplaceAll(kv, "$ROOT", root), "=")
				t.Setenv(k, v)
			}
			var out strings.Builder
			code := c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, rootArgs(tc.args, root))
			got := out.String() + takeChanges(t, root, before)
			if check.normalize != nil {
				got = check.normalize(got)
			}
			checkGolden(t, check.golden, tc.name, got, root, code)
		})
	}
}

// registryContract is a contract body with a status, a purpose and a
// reference to itself (which is not source).
func registryContract(status, purpose string) string {
	return "# Contract\n\n**Status:** " + status + "\n\n## Purpose\n\n" + purpose + "\n\n## Interface\n\nSee @contract:SELF.1.0.\n"
}

var registryCheck = parityCheck{name: "compute-registry", writes: true, golden: "compute-registry"}

var registryParityCases = []parityCase{
	{
		name: "default run writes the registry",
		files: map[string]string{
			"architecture/CONTRACT-S1-API.1.0.md":        registryContract("Active", "Serves the public API."),
			"architecture/CONTRACT-S2-STORE.2.1.md":      registryContract("draft", "Stores things."),
			"architecture/CONTRACT-C1-UI.3.md":           registryContract("Stable", "Renders | pipes | in the purpose."),
			"architecture/CONTRACT-I1-HTTP.1.0.md":       registryContract("Active", "HTTP interface."),
			"architecture/CONTRACT-P1-WIRE.1.0.md":       registryContract("Active", "Wire protocol."),
			"architecture/CONTRACT-X9-ODD.1.0.md":        registryContract("Active", "Other category."),
			"architecture/CONTRACT-s1-lower.1.0.md":      registryContract("Active", "lowercase s is Other."),
			"architecture/CONTRACT-NOVERSION.md":         registryContract("Active", "No version in the name."),
			"architecture/CONTRACT-S1-API.1.0.impl.md":   "# companion\n",
			"architecture/CONTRACT-TEMPLATE.md":          registryContract("template", "Template."),
			"architecture/CONTRACT-SEAM-TEMPLATE.md":     "# seam\n",
			"architecture/CONTRACT-REGISTRY.template.md": "# t\n",
			"architecture/CONTRACT-GAPS.md":              "# gaps\n",
			"architecture/CONTRACT-REGISTRY.md":          "old\n",
			"architecture/README.md":                     "not a contract\n",
			"architecture/CONTRACT-S3-DIR.1.0.md/x":      "a directory, not a contract\n",
		},
	},
	{
		name:  "stdout",
		files: map[string]string{"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Serves.")},
		args:  []string{"--stdout"},
	},
	{
		name:  "an unknown argument generates",
		files: map[string]string{"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Serves.")},
		args:  []string{"--frobnicate"},
	},
	{
		name:  "no contracts directory, stdout",
		files: map[string]string{"README.md": "x\n"},
		args:  []string{"--stdout"},
	},
	{
		name:  "empty contracts directory",
		files: map[string]string{"architecture/README.md": "x\n"},
	},
	{
		name: "check: current",
		files: map[string]string{
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Serves."),
			"architecture/CONTRACT-REGISTRY.md": "# Contract Registry\n\n<!-- AUTO-GENERATED by scripts/compute-registry.sh — do not edit manually -->\n" +
				"<!-- Regenerate: ./scripts/compute-registry.sh -->\n<!-- Generated: 2025-01-01 -->\n\n## Services\n\n| ID | Version | Status | Purpose |\n|----|---------|--------|---------|\n" +
				"| S1-API | 1.0 | Active | Serves. |\n\n## Contract Files\n\n<!-- Plain list of filenames for grep-based tooling -->\n- CONTRACT-S1-API.1.0.md\n",
		},
		args: []string{"--check"},
	},
	{
		name: "check: stale",
		files: map[string]string{
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Serves."),
			"architecture/CONTRACT-S2-NEW.1.0.md": registryContract("Active", "New."),
			"architecture/CONTRACT-REGISTRY.md": "# Contract Registry\n\n<!-- AUTO-GENERATED by scripts/compute-registry.sh — do not edit manually -->\n" +
				"<!-- Regenerate: rebar run compute-registry -->\n\n## Services\n\n| ID | Version | Status | Purpose |\n|----|---------|--------|---------|\n" +
				"| S1-API | 1.0 | Active | Serves. |\n\n## Contract Files\n\n<!-- Plain list of filenames for grep-based tooling -->\n- CONTRACT-S1-API.1.0.md\n\n",
		},
		args: []string{"--check"},
	},
	{
		name:  "check: missing registry",
		files: map[string]string{"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Serves.")},
		args:  []string{"--check"},
	},
	{
		name: "check: an empty registry for no contracts",
		files: map[string]string{
			"architecture/CONTRACT-REGISTRY.md": "",
		},
		args: []string{"--check"},
	},
	{
		name: "statuses",
		files: map[string]string{
			"architecture/CONTRACT-S01-BOLD.1.0.md":     "**Status:** Active\n",
			"architecture/CONTRACT-S02-PLAIN.1.0.md":    "Status: Deprecated\n",
			"architecture/CONTRACT-S03-CASE.1.0.md":     "STATUS: shouting\n**status:** later\n",
			"architecture/CONTRACT-S04-NONE.1.0.md":     "# no status\n",
			"architecture/CONTRACT-S05-COLON.1.0.md":    "Status: Draft: v2 pending\n",
			"architecture/CONTRACT-S06-SPACES.1.0.md":   "Status:   lots   of   space  \t\n",
			"architecture/CONTRACT-S07-QUOTES.1.0.md":   "Status: \"Active\" 'really'\n",
			"architecture/CONTRACT-S08-STARS.1.0.md":    "**Status:** **Bold** value\n",
			"architecture/CONTRACT-S09-EMPTY.1.0.md":    "Status:\n",
			"architecture/CONTRACT-S10-INDENT.1.0.md":   "  Status: indented does not count\n",
			"architecture/CONTRACT-S11-LATER.1.0.md":    "# T\n\nintro\n\nStatus: second paragraph\n",
			"architecture/CONTRACT-S12-CRLF.1.0.md":     "Status: Windows\r\n",
			"architecture/CONTRACT-S13-SLASH.1.0.md":    "Status: a\\b c\\\\d\n",
			"architecture/CONTRACT-S14-UTF8.1.0.md":     "Status: Entwurf – ünïcödé\n",
			"architecture/CONTRACT-S15-NOEOL.1.0.md":    "Status: no newline",
			"architecture/CONTRACT-S16-STATUSES.1.0.md": "Statuses: plural\n",
			"architecture/CONTRACT-S17-ECHO.1.0.md":     "Status: -n\n",
		},
		args: []string{"--stdout"},
	},
	{
		name:       "a status right after the colon",
		files:      map[string]string{"architecture/CONTRACT-S1-A.1.0.md": "Status:stable\n"},
		args:       []string{"--stdout"},
		divergesOn: "bsd", // BSD sed reads \s as s, so s/.*:\s*// also drops the s of stable
		want:       registryWant("| S1-A | 1.0 | stable | — |", "CONTRACT-S1-A.1.0.md"),
	},
	{
		name: "a binary contract",
		files: map[string]string{
			"architecture/CONTRACT-S1-BIN.1.0.md": "Status: Active\n\x00\n## Purpose\n\nBinary.\n",
		},
		args: []string{"--stdout"},
		// BSD grep prints "Binary file … matches", which the script took
		// for the status; GNU grep prints nothing, a draft.
		divergesOn: "bsd",
		want:       registryWant("| S1-BIN | 1.0 | draft | Binary. |", "CONTRACT-S1-BIN.1.0.md"),
	},
	{
		name: "purposes",
		files: map[string]string{
			"architecture/CONTRACT-S01-LONG.1.0.md":      "## Purpose\n" + strings.Repeat("abcdefghij", 9) + "\n",
			"architecture/CONTRACT-S02-EIGHTY.1.0.md":    "## Purpose\n" + strings.Repeat("abcdefghij", 8) + "\n",
			"architecture/CONTRACT-S03-EIGHTYONE.1.0.md": "## Purpose\n" + strings.Repeat("abcdefghij", 8) + "k\n",
			"architecture/CONTRACT-S04-UTF8.1.0.md":      "## Purpose\n" + strings.Repeat("é", 79) + "ü\n",
			"architecture/CONTRACT-S05-UTF8LONG.1.0.md":  "## Purpose\n" + strings.Repeat("ö", 100) + "\n",
			"architecture/CONTRACT-S06-PIPES.1.0.md":     "## Purpose\n" + strings.Repeat("|", 79) + "\n",
			"architecture/CONTRACT-S07-SKIP.1.0.md":      "## Purpose\n\n   \n\t\n### Sub heading\n\nAfter the sub heading.\n",
			"architecture/CONTRACT-S08-NONE.1.0.md":      "# No purpose\n\nText.\n",
			"architecture/CONTRACT-S09-EMPTY.1.0.md":     "## Purpose\n\n## Next\n\n",
			"architecture/CONTRACT-S10-LATER.1.0.md":     "## Purpose\n\n## Other\n\nfrom the next section\n",
			"architecture/CONTRACT-S11-PREFIX.1.0.md":    "## Purposeful\nmatches as a prefix\n",
			"architecture/CONTRACT-S12-INDENT.1.0.md":    "## Purpose\n    indented text\n",
			"architecture/CONTRACT-S13-CRLF.1.0.md":      "## Purpose\r\n\r\nWindows line\r\n",
			"architecture/CONTRACT-S14-NUMBERED.1.0.md":  "## 2. Purpose\n\nnot without the heading setting\n",
			"architecture/CONTRACT-S15-TWICE.1.0.md":     "## Purpose\n## Purpose\nsecond heading is skipped\n",
			"architecture/CONTRACT-S16-INVALID.1.0.md":   "## Purpose\n" + strings.Repeat("ab\xe9", 30) + "\n",
			"architecture/CONTRACT-S17-NOEOL.1.0.md":     "## Purpose\nno newline",
			"architecture/CONTRACT-S18-BACKSLASH.1.0.md": "## Purpose\nback\\slash and \\| escaped\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "purpose style bytes",
		files: map[string]string{
			".rebarrc":                               "registry_purpose_style = bytes\n",
			"architecture/CONTRACT-S01-LONG.1.0.md":  "## Purpose\n" + strings.Repeat("abcdefghij", 9) + "\n",
			"architecture/CONTRACT-S02-UTF8.1.0.md":  "## Purpose\n" + strings.Repeat("é", 50) + "\n",
			"architecture/CONTRACT-S03-PIPES.1.0.md": "## Purpose\na | b | c\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "purpose heading, numbered headings",
		files: map[string]string{
			".rebarrc":                               "registry_purpose_heading = Purpose|Why this exists\n",
			"architecture/CONTRACT-S1-NUM.1.0.md":    "## 2. Purpose\n\nnumbered\n",
			"architecture/CONTRACT-S2-WHY.1.0.md":    "## Why this exists\n\nwhy\n",
			"architecture/CONTRACT-S3-NUMWHY.1.0.md": "## 10.Why this exists\nnumbered without a space\n",
			"architecture/CONTRACT-S4-PLAIN.1.0.md":  "## Purpose\nplain\n",
			"architecture/CONTRACT-S5-OTHER.1.0.md":  "## Scope\nnot a purpose\n",
			"architecture/CONTRACT-S6-LETTER.1.0.md": "## A. Purpose\nletters are not numbers\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "purpose heading with a comment",
		files: map[string]string{
			".rebarrc":                            "registry_purpose_heading = Summary # the heading we use\nregistry_purpose_style = ellipsis\n",
			"architecture/CONTRACT-S1-SUM.1.0.md": "## Summary\nsummary text\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "scope prefixes",
		files: map[string]string{
			".rebarrc": "registry_scope_prefixes = SDK- MCP-\n",
			"architecture/CONTRACT-SDK-S1-FOO.1.0.md": registryContract("Active", "SDK service."),
			"architecture/CONTRACT-MCP-C1-BAR.1.0.md": registryContract("Active", "MCP component."),
			"architecture/CONTRACT-SDK-MCP-I1.1.0.md": registryContract("Active", "Both prefixes, in order."),
			"architecture/CONTRACT-MCP-SDK-P1.1.0.md": registryContract("Active", "Wrong order: SDK- stays, so it files under its S."),
			"architecture/CONTRACT-X1-PLAIN.1.0.md":   registryContract("Active", "No prefix."),
		},
		args: []string{"--stdout"},
	},
	{
		name: "contracts_dir and REBAR_CONTRACTS_DIR",
		files: map[string]string{
			".rebarrc":                          "contracts_dir = ./docs/specs/\n",
			"docs/specs/CONTRACT-S1-A.1.0.md":   registryContract("Active", "In docs/specs."),
			"specs2/CONTRACT-S2-B.1.0.md":       registryContract("Active", "In specs2."),
			"architecture/CONTRACT-S3-C.1.0.md": registryContract("Active", "Not read."),
		},
		env: []string{"REBAR_CONTRACTS_DIR=specs2/"},
	},
	{
		name: "contracts_dir from .rebarrc",
		files: map[string]string{
			".rebarrc":                        "contracts_dir = ./docs/specs/\n",
			"docs/specs/CONTRACT-S1-A.1.0.md": registryContract("Active", "In docs/specs."),
		},
	},
	{
		name: "REBAR_RCFILE",
		files: map[string]string{
			".rebarrc":                          "registry_purpose_style = bytes\n",
			"alt.rc":                            "registry_impl_counts = true\n",
			"architecture/CONTRACT-S1-A.1.0.md": registryContract("Active", strings.Repeat("x", 90)),
			"src/a.go":                          "// @contract:S1-A.1.0\n",
		},
		args: []string{"--stdout"},
		env:  []string{"REBAR_RCFILE=$ROOT/alt.rc"},
	},
	{
		name: "impl counts outside git",
		files: map[string]string{
			".rebarrc":                                "registry_impl_counts = true\n",
			"TODO.md":                                 "- [ ] implement S4-PLANNED\n",
			"architecture/CONTRACT-S1-API.1.0.md":     registryContract("Active", "Counted."),
			"architecture/CONTRACT-S1-API.2.0.md":     registryContract("Active", "Second version, same ID."),
			"architecture/CONTRACT-S2-ORPHAN.1.0.md":  registryContract("Active", "Nothing implements it."),
			"architecture/CONTRACT-S4-PLANNED.1.0.md": registryContract("Active", "Tracked in TODO.md."),
			"architecture/CONTRACT-S5-PRE.1.0.md":     registryContract("Active", "Prefix of S5-PREFIX."),
			"src/a.go":                                "// @contract:S1-API.1.0\n// @contract:S1-API.2.0 twice in one file\n",
			"src/b.ts":                                "// @contract:S1-API.2.0\n",
			"src/c.py":                                "# @contract:S5-PREFIX.1.0 counts for S5-PRE too\n",
			"src/d.txt":                               "@contract:S1-API.1.0 wrong extension\n",
			"src/e.go":                                "// @contract:S9-GHOST.1.0 and @contract:S8-SPOOK.2.3\n",
			"bin/tool":                                "#!/bin/sh\n# @contract:S1-API.1.0 extensionless under bin/\n",
			"bin/sub/deeper":                          "# @contract:S1-API.1.0\n",
			"node_modules/x/f.js":                     "// @contract:S2-ORPHAN.1.0\n",
			"pkg/vendor/g.go":                         "// @contract:S2-ORPHAN.1.0\n",
			"dist/h.js":                               "// @contract:S2-ORPHAN.1.0\n",
			".claude/w/i.go":                          "// @contract:S2-ORPHAN.1.0\n",
			".git-like/j.go":                          "// @contract:S1-API.1.0 hidden dirs are searched\n",
			"distance.go":                             "// @contract:S1-API.1.0 names are not directories\n",
			"src/bin.go":                              "// @contract:S2-ORPHAN.1.0\n\x00\n",
			"architecture/scratch.go":                 "// @contract:S2-ORPHAN.1.0 the contracts dir is not source\n",
			"src/crlf.rs":                             "// @contract:S6-CRLF.1.0\r\n",
			"src/short.go":                            "// @contract:S7-SHORT.1 no minor is not a shadow\n",
			"src/vendor":                              "a file named like an excluded dir\n",
		},
	},
	{
		name: "health outside git",
		files: map[string]string{
			"TODO.md":                                "S2-ORPHAN is planned\n",
			"architecture/CONTRACT-S1-API.1.0.md":    registryContract("Active", "Counted."),
			"architecture/CONTRACT-S1-API.2.0.md":    registryContract("Active", "Second version."),
			"architecture/CONTRACT-S2-ORPHAN.1.0.md": registryContract("Active", "Orphan."),
			"architecture/CONTRACT-S3-LOST.1.0.md":   registryContract("Active", "Untracked zombie."),
			"src/a.go":                               "// @contract:S1-API.1.0 @contract:S9-SHADOW.1.0\n",
			"lib/b.jsx":                              "// @contract:S8-OTHER.4.5\n",
		},
		args: []string{"--health"},
	},
	{
		name: "health with nothing to report",
		files: map[string]string{
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
			"src/a.go":                            "// @contract:S1-API.1.0\n",
		},
		args: []string{"--health"},
	},
	{
		name: "impl counts in git: tracked files only",
		files: map[string]string{
			".gitignore":                             "build/\n",
			"architecture/CONTRACT-S1-API.1.0.md":    registryContract("Active", "Counted."),
			"architecture/CONTRACT-S2-ORPHAN.1.0.md": registryContract("Active", "Orphan."),
			"src/a.go":                               "// @contract:S1-API.1.0\n",
			"bin/tool":                               "# @contract:S1-API.1.0\n",
			"deep/x/y/z.rb":                          "# @contract:S1-API.1.0 any depth\n",
			"src/bin.go":                             "// @contract:S2-ORPHAN.1.0\n\x00\n",
			"vendor/v.go":                            "// @contract:S2-ORPHAN.1.0\n",
			"a/node_modules/n.js":                    "// @contract:S2-ORPHAN.1.0\n",
			"architecture/impl.go":                   "// @contract:S2-ORPHAN.1.0\n",
			"src/ghost.java":                         "// @contract:S9-GHOST.1.0\n",
		},
		untracked: map[string]string{
			"src/scratch.go": "// @contract:S2-ORPHAN.1.0 untracked\n",
			"build/out.go":   "// @contract:S2-ORPHAN.1.0 ignored\n",
		},
		git:  true,
		env:  []string{"REBAR_REGISTRY_IMPL_COUNTS=yes"},
		args: []string{"--stdout"},
	},
	{
		name: "health in git, symlinks are not searched",
		files: map[string]string{
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
			"target.txt":                          "// @contract:S1-API.1.0\n",
		},
		links: map[string]string{"link.go": "target.txt"},
		git:   true,
		args:  []string{"--health"},
	},
	{
		name: "impl counts off: env overrides .rebarrc",
		files: map[string]string{
			".rebarrc":                            "registry_impl_counts = true\n",
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
		},
		env:  []string{"REBAR_REGISTRY_IMPL_COUNTS=0"},
		args: []string{"--stdout"},
	},
	{
		name: "impl globs and exclude dirs",
		files: map[string]string{
			".rebarrc":                            "registry_impl_counts = on\nregistry_impl_globs = *.kt tools/\nregistry_exclude_dirs = generated\n",
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
			"src/a.kt":                            "// @contract:S1-API.1.0\n",
			"src/b.go":                            "// @contract:S1-API.1.0 not a configured glob\n",
			"tools/run":                           "# @contract:S1-API.1.0\n",
			"bin/old":                             "# @contract:S1-API.1.0 bin/ no longer searched\n",
			"src/generated/c.kt":                  "// @contract:S1-API.1.0\n",
			"node_modules/d.kt":                   "// @contract:S1-API.1.0 no longer excluded\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "impl globs and exclude dirs in git",
		files: map[string]string{
			".rebarrc":                            "registry_impl_counts = on\nregistry_impl_globs = *.kt tools/\nregistry_exclude_dirs = generated\n",
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
			"src/a.kt":                            "// @contract:S1-API.1.0\n",
			"src/b.go":                            "// @contract:S1-API.1.0 not a configured glob\n",
			"tools/run":                           "# @contract:S1-API.1.0\n",
			"src/generated/c.kt":                  "// @contract:S1-API.1.0\n",
			"node_modules/d.kt":                   "// @contract:S1-API.1.0 no longer excluded\n",
		},
		git:  true,
		args: []string{"--stdout"},
	},
	{
		name: "contract namespace",
		files: map[string]string{
			".rebarrc":                              "contract_namespace = github.com/acme/app\nregistry_impl_counts = true\n",
			"architecture/CONTRACT-S1-API.1.0.md":   registryContract("Active", "Counted."),
			"architecture/CONTRACT-S2-LOCAL.1.0.md": registryContract("Active", "Qualified with this repo."),
			"src/a.go":                              "// @contract:S1-API.1.0\n",
			"src/b.go":                              "// @contract:github.com/acme/app:S2-LOCAL.1.0\n",
			"src/c.go":                              "// @contract:github.com/other/lib:S3-FOREIGN.1.0 is not a shadow\n",
			"src/d.go":                              "// @contract:github.com/acme/app:S4-MISSING.1.0 is one\n",
			"src/e.go":                              "// @contract:githubXcom/acme/app:S2-LOCAL.1.0 dots are literal\n",
		},
	},
	{
		name: "an invalid contract namespace is ignored, with a warning",
		files: map[string]string{
			".rebarrc":                            "contract_namespace = bad ns!\nregistry_impl_counts = true\n",
			"architecture/CONTRACT-S1-API.1.0.md": registryContract("Active", "Counted."),
			"src/a.go":                            "// @contract:S1-API.1.0\n",
		},
		args: []string{"--stdout"},
	},
	{
		name: "symlinked contract file",
		files: map[string]string{
			"shared/CONTRACT-S1-A.1.0.md": registryContract("Active", "Through a symlink."),
			"architecture/README.md":      "x\n",
		},
		links: map[string]string{"architecture/CONTRACT-S1-A.1.0.md": "../shared/CONTRACT-S1-A.1.0.md", "architecture/CONTRACT-S2-DANGLING.1.0.md": "nowhere.md"},
		args:  []string{"--stdout"},
	},
	{
		name: "default run over a longer registry",
		files: map[string]string{
			"architecture/CONTRACT-S1-A.1.0.md": registryContract("Active", "A."),
			"architecture/CONTRACT-REGISTRY.md": strings.Repeat("a much longer old registry\n", 50),
		},
	},
}

// registryWant is the --stdout registry of one Services row.
func registryWant(row, file string) string {
	return "# Contract Registry\n\n<!-- AUTO-GENERATED by scripts/compute-registry.sh — do not edit manually -->\n" +
		"<!-- Regenerate: rebar run compute-registry -->\n\n## Services\n\n| ID | Version | Status | Purpose |\n|----|---------|--------|---------|\n" +
		row + "\n\n## Contract Files\n\n<!-- Plain list of filenames for grep-based tooling -->\n- " + file + "\n\n"
}

func TestComputeRegistryParity(t *testing.T) {
	runParity(t, registryCheck, registryParityCases)
}

func TestComputeRegistryGolden(t *testing.T) {
	runGolden(t, registryCheck, registryParityCases)
}
