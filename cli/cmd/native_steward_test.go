package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Parity of steward (CONTRACT:S1-STEWARD.2.0; runParity with the files it
// writes as part of the output), with goldens in testdata/steward
// (regenerate with go test ./cmd -run TestSteward -update-golden).
// Contract references in fixture bodies are spelled @contract:.

// stewardTimestamp is the scan's generated_at, which differs between runs.
var stewardTimestamp = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

var stewardCheck = parityCheck{
	name:      "steward",
	writes:    true,
	golden:    "steward",
	normalize: func(out string) string { return stewardTimestamp.ReplaceAllString(out, "<generated-at>") },
}

// specContract has every section the spec gate asks for.
func specContract(id string) string {
	return "# " + id + "\n\n## Interfaces\n\nx\n\n## Behavioral Contracts\n\nx\n\n## Error Contracts\n\nx\n\n## Test Requirements\n\nx\n\n## Implementing Files\n\nx\n"
}

// stewardTree has a contract in each lifecycle state, references in and
// out of scope, and TODO.md discoveries.
var stewardTree = map[string]string{
	".rebarrc":                               "tier = 1\n",
	"architecture/CONTRACT-S1-DRAFT.1.0.md":  "# S1-DRAFT\n\n## interfaces\n\n## ERRORS\n",
	"architecture/CONTRACT-S2-ACTIVE.2.1.md": specContract("S2-ACTIVE"),
	"architecture/CONTRACT-S3-TEST.1.0.md":   specContract("S3-TEST"),
	"architecture/CONTRACT-S4-DONE.1.0.md":   specContract("S4-DONE"),
	"architecture/CONTRACT-TEMPLATE.md":      specContract("TEMPLATE"),
	"architecture/CONTRACT-REGISTRY.md":      "# registry @contract:S4-DONE.1.0\n",
	"architecture/notes.go":                  "// @contract:S2-ACTIVE.2.1 under the contracts dir: not an implementation\n",
	"architecture-old/keep.go":               "// @contract:S2-ACTIVE.2.1 the dir is a prefix of the line: dropped too\n",
	"src/test.go":                            "package src\n// @contract:S3-TEST.1.0\n// @contract:S3-TEST.1.0 twice in one file\n",
	"src/z.go":                               "// @contract:S4-DONE.1.0\n",
	"src/a/done.go":                          "// @contract:S4-DONE.1.0\n",
	"src/done_test.go":                       "// @contract:S4-DONE.1.0\n",
	"web/done.spec.ts":                       "// @contract:S4-DONE.1.0\n",
	"web/done.test.js":                       "// @contract:S4-DONE.1.0\n",
	"web/done.tsx":                           "// @contract:S4-DONE.1.0\n",
	"conf/done.yaml":                         "ref: @contract:S4-DONE.1.0\n",
	"docs/done.md":                           "@contract:S4-DONE.1.0 markdown is not an implementation\n",
	"node_modules/x/done.js":                 "// @contract:S4-DONE.1.0\n",
	"src/vendor/node_modules/y.go":           "// @contract:S4-DONE.1.0\n",
	"src/mention.go":                         "// @contract:S4-DONE.1.0 see /node_modules/ (the line is dropped)\n",
	"src/prefix.go":                          "// @contract:S4-DONEX.1.0 an ID the pattern prefixes\n",
	"TODO.md": "# TODO\n\n" +
		"- **BUG** S4-DONE crashes\n" +
		"  - **discovery** S4-DONE: a \"quoted\" \\path\twith a tab\n" +
		"- **Drift** and **DISPUTE** S3-TEST: the first tag counts\n" +
		"- **DISPUTE** S3-TEST again\n" +
		"- **DISCOVERY** S2-ACTIVE caf\xe9 in Latin-1, ünïcode, del\x7f\n" +
		"- S2-ACTIVE without a tag\n" +
		"- **BUG** S1-DRAFT\r\n" +
		"- **BUGS** S1-DRAFT is not a tag\n",
}

// fileWithColon is a file whose name holds a colon, which the script's
// `cut -d: -f1` truncates.
func fileWithColon(t *testing.T, root string) {
	if err := os.WriteFile(filepath.Join(root, "src", "odd:name.go"), []byte(testutil.Tags.Replace("// @contract:S2-ACTIVE.2.1\n")), 0o644); err != nil {
		t.Skip("file names with a colon unsupported:", err)
	}
}

var stewardParityCases = []parityCase{
	{name: "empty project", files: map[string]string{"README.md": "# x\n"}},
	{name: "no contracts", files: map[string]string{".rebarrc": "tier = 1\n", "architecture/README.md": "# Architecture\n"}},
	{name: "every lifecycle state", files: stewardTree},
	{name: "--json", files: stewardTree, args: []string{"--json"}},
	{name: "--summary", files: stewardTree, args: []string{"--summary"}},
	{name: "an unknown mode is a full scan", files: stewardTree, args: []string{"--verbose"}},
	{name: "an empty mode is a full scan", files: stewardTree, args: []string{""}},
	{name: "--check an ID", files: stewardTree, args: []string{"--check", "S4-DONE"}},
	{name: "--check an ID prefix: the state glob matches nothing", files: stewardTree, args: []string{"--check", "S4"}},
	{name: "--check an unknown ID lists the valid ones", files: withFiles(stewardTree, map[string]string{
		"architecture/CONTRACT-GAPS.md":             "x\n",
		"architecture/CONTRACT-S4-DONE.1.0.impl.md": "x\n",
		"architecture/CONTRACT-PLAIN.md":            "x\n",
		"architecture/CONTRACT-S4-DONE.2.0.md":      "x\n",
	}), args: []string{"--check", "S9-NONE"}},
	{name: "--check without an ID", files: stewardTree, args: []string{"--check"}},
	{
		name: "--check prints every state file of the ID",
		files: withFiles(stewardTree, map[string]string{
			"architecture/.state/S4-DONE.0.9.json": `{"contract_id":"S4-DONE","version":"0.9","n":[1,2.5,{}],"s":"a\u00e9\u2028/"}`,
		}),
		args: []string{"--check", "S4-DONE"},
	},
	{
		name: "unversioned, major-only and companion contract files",
		files: map[string]string{
			".rebarrc":                                "tier = 1\n",
			"architecture/CONTRACT-PLAIN.md":          specContract("PLAIN"),
			"architecture/CONTRACT-MAJOR.3.md":        specContract("MAJOR"),
			"architecture/CONTRACT-A.B.1.2.3.md":      specContract("A.B"),
			"architecture/CONTRACT-MAJOR.3.0.impl.md": "# companion\n",
			"architecture/CONTRACT-GAPS.md":           "# gaps\n",
			"src/plain.go":                            "// @contract:PLAIN\n// @contract:MAJOR.3\n// @contract:A.B.1.2\n",
		},
	},
	{
		name: "contracts_dir and contract_namespace",
		files: map[string]string{
			".rebarrc":                            "tier = 1\ncontracts_dir = ./docs/specs/\ncontract_namespace = github.com/org/repo\n",
			"docs/specs/CONTRACT-C1-API.1.0.md":   specContract("C1-API"),
			"architecture/CONTRACT-C2-OLD.1.0.md": specContract("C2-OLD"),
			"src/own.go":                          "// @contract:github.com/org/repo:C1-API.1.0\n",
			"src/plain.go":                        "// @contract:C1-API.1.0\n",
			"src/foreign.go":                      "// @contract:github.com/other/repo:C1-API.1.0\n",
			"src/own_test.go":                     "// @contract:github.com/org/repo:C1-API.1.0\n",
		},
	},
	{
		name: "an invalid contract_namespace is warned about once",
		files: map[string]string{
			".rebarrc":                            "tier = 1\ncontract_namespace = S1-BAD.1.0\n",
			"architecture/CONTRACT-C1-API.1.0.md": specContract("C1-API"),
		},
	},
	{
		name: "failing and customized enforcement checks",
		files: withFiles(stewardTree, map[string]string{
			"src/b.go":                   "// @todo untracked\n",
			"scripts/check-freshness.sh": "#!/usr/bin/env bash\nexit 0\n",
		}),
		setup: func(t *testing.T, root string) {
			testutil.WriteExecutable(t, root, "scripts/check-contract-headers.sh", "#!/usr/bin/env bash\necho custom\nexit 3\n")
			testutil.WriteExecutable(t, root, "scripts/compute-registry.sh", "#!/usr/bin/env bash\n# rebar-thin-wrapper\nexit 1\n")
		},
	},
	{name: "a file name with a colon", files: stewardTree, setup: fileWithColon},
	{
		name: "a binary contract file still passes the spec gate",
		files: map[string]string{
			".rebarrc":                            "tier = 1\n",
			"architecture/CONTRACT-S1-BIN.1.0.md": specContract("S1-BIN") + "\x00\n",
		},
	},
}

// stewardLocale runs the scripts in the C locale: the native steward sorts
// in byte order, and GNU grep under a UTF-8 locale takes a line that is not
// valid UTF-8 for binary data (neither is emulated). LC_ALL also beats a
// runner's own LC_ALL, which LC_COLLATE alone would not.
func stewardLocale(t *testing.T) { t.Setenv("LC_ALL", "C") }

func TestStewardParity(t *testing.T) {
	t.Setenv("REBAR_TEST_RUN_CLI", "1")
	stewardLocale(t)
	runParity(t, stewardCheck, stewardParityCases)
}

func TestStewardGolden(t *testing.T) {
	runGolden(t, stewardCheck, stewardParityCases)
}

// TestStewardBinaryReference: a binary file holding a reference is no
// implementation. BSD grep prints "Binary file … matches" for it, which the
// script then takes for a file name; GNU grep 3.5+ prints nothing on
// stdout, as the native port.
func TestStewardBinaryReference(t *testing.T) {
	requireBash(t)
	stewardLocale(t)
	files := map[string]string{
		".rebarrc":                            "tier = 1\n",
		"architecture/CONTRACT-S1-BIN.1.0.md": specContract("S1-BIN"),
		"data/blob.json":                      "@contract:S1-BIN.1.0\n\x00\n",
		"src/a.go":                            "// @contract:S1-BIN.1.0\n",
	}
	state := func(root string) string {
		data, err := os.ReadFile(filepath.Join(root, "architecture", ".state", "S1-BIN.1.0.json"))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(data), root, "$ROOT")
	}
	root := writeTree(t, files, nil)
	script := filepath.Join(testutil.RebarSource(t), "scripts", "steward.sh")
	if out, code := testutil.RunEnv(t, root, testutil.IsolatedEnv("REBAR_PROJECT_ROOT="+root), "bash", script, "--summary"); code != 0 {
		t.Fatalf("script: exit %d\n%s", code, out)
	}
	shell := state(root)
	c, _ := checks.Lookup("steward")
	var out strings.Builder
	if code := c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, []string{"--summary"}); code != 0 {
		t.Fatalf("native: exit %d\n%s", code, out.String())
	}
	native := state(root)
	if !strings.Contains(native, `"$ROOT/src/a.go"`) || strings.Contains(native, "blob.json") {
		t.Errorf("native lists the binary file or misses the text one:\n%s", native)
	}
	switch grepFlavour(t) {
	case "bsd":
		if !strings.Contains(shell, `"Binary file $ROOT/data/blob.json matches"`) {
			t.Errorf("BSD grep: the script was expected to list the binary-file message:\n%s", shell)
		}
	case "gnu":
		if shell != native {
			t.Errorf("GNU grep: script and native differ:\n%s\n%s", shell, native)
		}
	}
}

// TestStewardCLI runs `rebar run steward` and `rebar check` end to end:
// the native steward under the default implementation, the script under
// REBAR_CHECK_IMPL=shell, with the same output and reports.
func TestStewardCLI(t *testing.T) {
	requireBash(t)
	stewardLocale(t)
	files := map[string]string{}
	for rel, body := range stewardTree {
		files[rel] = testutil.Tags.Replace(body)
	}
	bin, home, src := testBinary(t), t.TempDir(), testutil.RebarSource(t)
	for _, args := range [][]string{{"run", "steward"}, {"run", "steward", "--summary"}} {
		var outs [2]string
		var reports [2]string
		for i, impl := range []string{"shell", "go"} {
			root := writeTree(t, files, nil)
			out, code := testutil.RunEnv(t, root, cliEnv(home, impl, src), bin, args...)
			if code != 0 {
				t.Fatalf("%s rebar %v: exit %d\n%s", impl, args, code, out)
			}
			outs[i] = stewardTimestamp.ReplaceAllString(strings.ReplaceAll(out, root, "$ROOT"), "<ts>")
			report, _ := os.ReadFile(filepath.Join(root, "architecture", ".state", "steward-report.json"))
			reports[i] = stewardTimestamp.ReplaceAllString(strings.ReplaceAll(string(report), root, "$ROOT"), "<ts>")
		}
		if outs[0] != outs[1] || reports[0] != reports[1] {
			t.Errorf("rebar %v: shell:\n%s\n%s\ngo:\n%s\n%s", args, outs[0], reports[0], outs[1], reports[1])
		}
	}
}
