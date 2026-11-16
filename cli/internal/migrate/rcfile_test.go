package migrate

import (
	"strings"
	"testing"
)

func TestMergeRCAppendsBlockAndKeepsExisting(t *testing.T) {
	rc := []byte("# REBAR Configuration\ntier = 1   # partial\nregistry_impl_counts = false\n")
	out, added := MergeRC(rc, []Setting{
		{Key: "registry_impl_counts", Value: "true", Source: "scripts/compute-registry.sh"}, // already set: skipped
		{Key: "registry_scope_prefixes", Value: "SDK- MCP-", Source: "scripts/compute-registry.sh"},
		{Key: "metric.a.paths", Value: "src", Source: "scripts/check-ground-truth.sh"},
		{Key: "metric.a.names", Value: "*.go", Source: "scripts/check-ground-truth.sh"},
		{Key: "metric.bb.contains", Value: "x#y", Source: "scripts/check-ground-truth.sh"}, // unwritable
	})
	want := "# REBAR Configuration\ntier = 1   # partial\nregistry_impl_counts = false\n\n" +
		blockStart + "\n" +
		"# from scripts/compute-registry.sh\n" +
		"registry_scope_prefixes = SDK- MCP-\n" +
		"# from scripts/check-ground-truth.sh\n" +
		"metric.a.paths = src\n" +
		"metric.a.names = *.go\n" +
		blockEnd + "\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	if len(added) != 3 {
		t.Fatalf("added %d settings, want 3: %+v", len(added), added)
	}

	// A second merge of the same settings changes nothing (idempotent).
	again, added2 := MergeRC(out, added)
	if string(again) != string(out) || len(added2) != 0 {
		t.Fatalf("second merge changed the file:\n%s", again)
	}

	// New settings later go inside the existing block, not a second one.
	more, _ := MergeRC(out, []Setting{{Key: "source_dirs", Value: "sdk", Source: "scripts/check-todos.sh"}})
	if strings.Count(string(more), blockStart) != 1 {
		t.Fatalf("second block written:\n%s", more)
	}
	if !strings.Contains(string(more), "# from scripts/check-todos.sh\nsource_dirs = sdk\n"+blockEnd) {
		t.Fatalf("setting not appended to the block:\n%s", more)
	}
}

func TestMergeRCAlignsMetricGroups(t *testing.T) {
	out, _ := MergeRC(nil, []Setting{
		{Key: "metric.go_modules.names", Value: "go.mod"},
		{Key: "metric.go_modules.exclude_dirs", Value: "vendor"},
		{Key: "metric.x.paths", Value: "a"},
	})
	want := blockStart + "\nmetric.go_modules.names        = go.mod\nmetric.go_modules.exclude_dirs = vendor\n\nmetric.x.paths = a\n" + blockEnd + "\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	// No trailing newline in the original: one is added before the block.
	out, _ = MergeRC([]byte("tier = 2"), []Setting{{Key: "k", Value: "v"}})
	if !strings.HasPrefix(string(out), "tier = 2\n\n"+blockStart) {
		t.Fatalf("got:\n%s", out)
	}
}

// RCValue and HasMetric read .rebarrc the way _rebar_config does: first
// line wins, `#` starts a comment, keys start at column 0.
func TestRCValue(t *testing.T) {
	rc := []byte("# metric.c.paths = commented\n  indented = no\nk = one # note\nk = two\nmetric.a.names = *.go\n")
	if v, ok := RCValue(rc, "k"); !ok || v != "one" {
		t.Fatalf("k = %q, %v", v, ok)
	}
	if _, ok := RCValue(rc, "indented"); ok {
		t.Fatal("an indented line is not a key")
	}
	if !HasMetric(rc, "a") || HasMetric(rc, "c") {
		t.Fatal("HasMetric")
	}
	if got := strings.Join(RCKeys(rc), ","); got != "k,metric.a.names" {
		t.Fatalf("RCKeys = %s", got)
	}
}

func TestApplyMetricsEdits(t *testing.T) {
	in := "# counts\nshell_scripts = 16\ntest_files >= 353   # floor\nother = 3\n"
	got := applyMetricsEdits(in, []MetricsEdit{{Key: "shell_scripts", Op: "=", From: "16", To: "12"}, {Key: "test_files", Op: ">=", From: "353", To: "350"}})
	want := "# counts\nshell_scripts = 12\ntest_files >= 350   # floor\nother = 3\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompareRuns(t *testing.T) {
	a := runResult{Stdout: "OK: 0 untracked TODOs. 9 tracked task(s).\n"}
	b := runResult{Stdout: "OK: 0 untracked TODOs. 9 tracked tasks.\n"}
	if ok, how := compareRuns(a, b); !ok || !strings.Contains(how, "wording") {
		t.Fatalf("wording-only difference: %v %s", ok, how)
	}
	// List numbering in instructions is wording, not a finding.
	a = runResult{Code: 1, Stdout: "FAIL: 1 found:\nsdk/a.go:1:// x\nFix it.\n"}
	b = runResult{Code: 1, Stdout: "FAIL: 1 found:\nsdk/a.go:1:// x\nEither:\n  1. fix it\n  2. track it\n"}
	if ok, _ := compareRuns(a, b); !ok {
		t.Fatal("numbered instructions made outputs differ")
	}
	if ok, _ := compareRuns(runResult{Stdout: "OK: 9 tracked"}, runResult{Stdout: "OK: 8 tracked"}); ok {
		t.Fatal("different counts compared equal")
	}
	if ok, _ := compareRuns(runResult{Stdout: "BROKEN: a/x.go:1"}, runResult{Stdout: "BROKEN: a/y.go:1"}); ok {
		t.Fatal("different findings compared equal")
	}
	if ok, _ := compareRuns(runResult{Code: 0, Stdout: "x"}, runResult{Code: 1, Stdout: "x"}); ok {
		t.Fatal("different exit status compared equal")
	}
}

func TestParseOutputs(t *testing.T) {
	mv := parseCheckOutput("\x1b[0;32mOK\x1b[0m:    a = 3\nOK: b = 4 (floor 2)\nDRIFT: c — METRICS says 1, code says 7\nDRIFT: d — METRICS floor is 9, repo has 5\nMISSING: e is not documented\n")
	got := []string{}
	for _, k := range mv.Keys {
		got = append(got, k+"="+mv.Val[k])
	}
	if strings.Join(got, " ") != "a=3 b=4 c=7 d=5" {
		t.Fatalf("parseCheckOutput = %v", got)
	}
	if h := gitBlobHash([]byte("hello\n")); h != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatalf("gitBlobHash = %s", h)
	}
}
