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

// steward (CONTRACT:S1-STEWARD.2.0; runGolden with the files it
// writes as part of the output), with goldens in testdata/steward
// (regenerate with go test ./cmd -run TestSteward -update-golden).
// Contract references in fixture bodies are spelled @contract:.

// stewardTimestamp is the scan's generated_at, which differs between runs.
var stewardTimestamp = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

var stewardCheck = goldenCheck{
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

var stewardCases = []goldenCase{
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

func TestStewardGolden(t *testing.T) {
	runGolden(t, stewardCheck, stewardCases)
}

// TestStewardBinaryReference: a binary file holding a reference is no
// implementation. (The script diverged by platform: BSD grep printed
// "Binary file … matches" for it, which the script then took for a file
// name; GNU grep 3.5+ printed nothing on stdout, as the native steward.)
func TestStewardBinaryReference(t *testing.T) {
	files := map[string]string{
		".rebarrc":                            "tier = 1\n",
		"architecture/CONTRACT-S1-BIN.1.0.md": specContract("S1-BIN"),
		"data/blob.json":                      "@contract:S1-BIN.1.0\n\x00\n",
		"src/a.go":                            "// @contract:S1-BIN.1.0\n",
	}
	root := writeTree(t, files, nil)
	c, _ := checks.Lookup("steward")
	var out strings.Builder
	if code := c.Run(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, []string{"--summary"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "architecture", ".state", "S1-BIN.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	if state := strings.ReplaceAll(string(data), root, "$ROOT"); !strings.Contains(state, `"$ROOT/src/a.go"`) || strings.Contains(state, "blob.json") {
		t.Errorf("the steward lists the binary file or misses the text one:\n%s", state)
	}
}

// TestStewardCLI runs `rebar run steward` end to end: the same output and
// reports as the steward run in process on the same tree.
func TestStewardCLI(t *testing.T) {
	files := map[string]string{}
	for rel, body := range stewardTree {
		files[rel] = testutil.Tags.Replace(body)
	}
	bin, home := testBinary(t), t.TempDir()
	c, _ := checks.Lookup("steward")
	for _, args := range [][]string{{"run", "steward"}, {"run", "steward", "--summary"}} {
		var outs, reports [2]string
		for i := range 2 {
			root := physical(t, writeTree(t, files, nil))
			var out string
			if i == 0 {
				var code int
				out, code = testutil.RunEnv(t, root, cliEnv(home), bin, args...)
				if code != 0 {
					t.Fatalf("rebar %v: exit %d\n%s", args, code, out)
				}
			} else {
				var b strings.Builder
				c.Run(&checks.Context{Root: root, Stdout: &b, Stderr: &b}, args[2:])
				out = b.String()
			}
			outs[i] = stewardTimestamp.ReplaceAllString(strings.ReplaceAll(out, root, "$ROOT"), "<ts>")
			report, _ := os.ReadFile(filepath.Join(root, "architecture", ".state", "steward-report.json"))
			reports[i] = stewardTimestamp.ReplaceAllString(strings.ReplaceAll(string(report), root, "$ROOT"), "<ts>")
		}
		if outs[0] != outs[1] || reports[0] != reports[1] {
			t.Errorf("rebar %v: CLI:\n%s\n%s\nin process:\n%s\n%s", args, outs[0], reports[0], outs[1], reports[1])
		}
	}
}
