package checks

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// clearSettings unsets the environment the gate reads, for the test.
func clearSettings(t *testing.T) {
	t.Helper()
	for _, k := range []string{"REBAR_TIER", "REBAR_CHECKS_ENABLE", "REBAR_RCFILE", "REBAR_CONTRACT_NAMESPACE", "_REBAR_CHECKS_VALIDATED", "_REBAR_NAMESPACE_VALIDATED"} {
		t.Setenv(k, "")
	}
}

func TestTier(t *testing.T) {
	clearSettings(t)
	for rc, want := range map[string]string{
		"":                                "3",
		"tier = 2\n":                      "2",
		"tier=1":                          "1",
		"tier = 1 # partial\n":            "3", // the comment stays in the value
		"tier_note = x\ntier = 1\n":       "3", // the first tier* line decides
		"tier = 4\n":                      "3",
		"  tier = 1\n":                    "3",
		"tier = 2\r\n":                    "3",
		"tier = 1\t\n":                    "3", // tr drops spaces, not tabs
		"tier = 1 \n":                     "1",
		"# tier = 1\ntier = 2\n":          "2",
		"tier = a = 1\n":                  "1", // after the last =
		"rebar_tier = 1\n":                "3",
		"tier\t=\t2\n":                    "2",
		"tier = 1\ntier = 2\n":            "1",
		"other = 1\ntiered=2\ntier = 1\n": "2",
	} {
		root := t.TempDir()
		if rc != "" {
			testutil.WriteFile(t, root, ".rebarrc", rc)
		}
		if got := Tier(root); got != want {
			t.Errorf(".rebarrc %q: Tier = %q, want %q", rc, got, want)
		}
	}

	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "tier = 1\n")
	alt := testutil.WriteFile(t, root, "alt.rc", "tier = 2\n")
	t.Setenv("REBAR_RCFILE", alt)
	if got := Tier(root); got != "2" {
		t.Errorf("REBAR_RCFILE: Tier = %q", got)
	}
	t.Setenv("REBAR_TIER", "two")
	if got := Tier(root); got != "two" {
		t.Errorf("REBAR_TIER is used as given: Tier = %q", got)
	}
}

func TestCheckTierAndSkip(t *testing.T) {
	clearSettings(t)
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "tier = 1\nchecks_enable = check-freshness.sh bogus\n")
	if got := CheckTier(root, "check-freshness"); got != "3" {
		t.Errorf("CheckTier(enabled) = %q", got)
	}
	if got := CheckTier(root, "check-jtbd-presence.sh"); got != "1" {
		t.Errorf("CheckTier(not enabled) = %q", got)
	}

	skip := func(min int, check string) (bool, string, string) {
		var out, errOut bytes.Buffer
		got := Skip(&Context{Root: root, Stdout: &out, Stderr: &errOut}, min, check)
		return got, out.String(), errOut.String()
	}
	got, out, errOut := skip(2, "check-freshness")
	if got || out != "NOTE: tier 1 < required tier 2; running check-freshness because .rebarrc checks_enable names it\n" {
		t.Errorf("enabled below its tier: %v %q", got, out)
	}
	if errOut != "rebar: WARN: checks_enable names unknown check 'bogus' (ignored; use a check script name without .sh, e.g. check-ground-truth)\n" {
		t.Errorf("unknown name warning: %q", errOut)
	}
	if os.Getenv("_REBAR_CHECKS_VALIDATED") != "1" {
		t.Error("the warning did not mark the run validated")
	}
	got, out, errOut = skip(2, "check-jtbd-presence")
	if !got || out != "SKIP: tier 1 < required tier 2 (set REBAR_TIER or .rebarrc to change, or name it in checks_enable)\n" || errOut != "" {
		t.Errorf("below its tier: %v %q %q (the warning is printed once)", got, out, errOut)
	}
	if got, out, _ = skip(1, "check-jtbd-presence"); got || out != "" {
		t.Errorf("at its tier: %v %q", got, out)
	}
	t.Setenv("REBAR_TIER", "x")
	if got, out, _ = skip(2, "check-jtbd-presence"); got || out != "" {
		t.Errorf("a tier that is not a number runs the check: %v %q", got, out)
	}
	t.Setenv("REBAR_TIER", "0")
	if got, out, _ = skip(2, "check-jtbd-presence"); !got || !strings.HasPrefix(out, "SKIP: tier 0 < required tier 2") {
		t.Errorf("tier 0: %v %q", got, out)
	}
}

func TestTestInt(t *testing.T) {
	for s, want := range map[string]int64{"3": 3, " 2 ": 2, "+1": 1, "-1": -1, "007": 7, "\t14\n": 14} {
		if n, ok := TestInt(s); !ok || n != want {
			t.Errorf("TestInt(%q) = %d, %v", s, n, ok)
		}
	}
	for _, s := range []string{"", "x", "1.5", "1 2", "0x10", "99999999999999999999"} {
		if _, ok := TestInt(s); ok {
			t.Errorf("TestInt(%q) ok", s)
		}
	}
}

func TestContractNamespaceWarnsOnce(t *testing.T) {
	clearSettings(t)
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "contract_namespace = S1-FOO.1.0\n")
	var errOut bytes.Buffer
	ctx := &Context{Root: root, Stdout: &errOut, Stderr: &errOut}
	if ns := ContractNamespace(ctx); ns != "" {
		t.Errorf("invalid namespace = %q", ns)
	}
	if ns := ContractNamespace(ctx); ns != "" || strings.Count(errOut.String(), "ignoring contract_namespace") != 1 {
		t.Errorf("second call: %q, output %q", ns, errOut.String())
	}
	testutil.WriteFile(t, root, ".rebarrc", "contract_namespace =  github.com/a/b/ \n")
	if ns := ContractNamespace(ctx); ns != "github.com/a/b" {
		t.Errorf("namespace = %q", ns)
	}
}

func TestAgeDays(t *testing.T) {
	for _, tc := range []struct {
		zone, doc, today string
		want             int64
	}{
		{"UTC", "2026-01-01", "2026-01-15", 14},
		{"America/New_York", "2026-03-01", "2026-03-15", 14},  // spring forward: a 23-hour day
		{"America/New_York", "2026-10-25", "2026-11-08", 14},  // fall back: a 25-hour day
		{"America/New_York", "2026-11-08", "2026-10-25", -13}, // truncated toward zero, as in the shell
		{"Europe/London", "2025-11-15", "2026-11-15", 365},
		{"Australia/Lord_Howe", "2026-03-30", "2026-04-13", 14}, // a 30-minute DST change
		{"America/Sao_Paulo", "2018-10-28", "2018-11-11", 14},   // across a day without a midnight
		{"America/Sao_Paulo", "2018-11-04", "2018-11-18", 14},   // that day, an hour off midnight
		{"America/Sao_Paulo", "2018-10-21", "2018-11-04", 14},
	} {
		loc, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Skip("no zone data:", err)
		}
		doc, ok := parseDayIn(tc.doc, loc)
		today, ok2 := parseDayIn(tc.today, loc)
		if !ok || !ok2 {
			t.Fatalf("%s: parse %s / %s", tc.zone, tc.doc, tc.today)
		}
		if got := ageDays(today, doc); got != tc.want {
			t.Errorf("%s %s → %s: %d days, want %d", tc.zone, tc.doc, tc.today, got, tc.want)
		}
	}
}

func TestParseDay(t *testing.T) {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		if _, ok := parseDayIn("2018-11-04", loc); !ok {
			t.Error("2018-11-04 had no midnight in Sao Paulo; GNU and BSD date still read it")
		}
	}
	for _, s := range []string{"2026-02-28", "2024-02-29", "1969-12-31", "0001-01-01", "9999-12-31"} {
		if _, ok := parseDayIn(s, time.UTC); !ok {
			t.Errorf("%s rejected", s)
		}
	}
	for _, s := range []string{"2026-02-29", "2025-02-30", "2026-13-01", "2026-00-10", "2026-01-00", "2026-01-32", "0000-00-00"} {
		if _, ok := parseDayIn(s, time.UTC); ok {
			t.Errorf("%s accepted (GNU date rejects it)", s)
		}
	}
}

// TestFreshnessAtAnyTimeOfDay: ages are whole calendar days whatever the
// time of day the check runs at (the BSD date bug, 12043c9, made them a day
// short when a second ticked during the run).
func TestFreshnessAtAnyTimeOfDay(t *testing.T) {
	clearSettings(t)
	root := t.TempDir()
	testutil.WriteFile(t, root, "a.md", "<!-- freshness: 2026-08-07 -->\n")
	testutil.WriteFile(t, root, "b.md", "last-synced: 2026-11-01\n")
	testutil.WriteFile(t, root, "c.md", "freshness: 2026-10-31\n")
	defer func() { now = time.Now }()
	for _, clock := range []string{"00:00:00", "00:00:01", "12:00:00", "23:59:59"} {
		at, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-11-15 "+clock, time.Local)
		now = func() time.Time { return at }
		var out bytes.Buffer
		code := runFreshness(&Context{Root: root, Stdout: &out, Stderr: &out}, nil)
		want := "STALE (100 days): ./a.md — freshness: 2026-08-07\n" +
			"STALE (15 days): ./c.md — freshness: 2026-10-31\n\n" +
			"Checked 3 docs with currency markers (freshness:/last-synced:), 2 stale (>14 days).\n" +
			"0 .md file(s) carry no currency marker at all — unchecked, not verified.\n\n" +
			"Stale docs may contain outdated status claims.\n" +
			"Review and update the freshness date after verifying content.\n"
		if code != 1 || out.String() != want {
			t.Errorf("at %s: exit %d\n%s", clock, code, out.String())
		}
	}
}

func TestSectionPresent(t *testing.T) {
	h := "## Why this exists"
	for body, want := range map[string]bool{
		"":                                 false,
		h + "\n":                           false,
		h + "\ncontent\n":                  true,
		h + "\n\n  \t\n## Next\ncontent\n": false,
		h + "\n### Sub\n":                  true,
		h + "\n# Top\ncontent\n":           false,
		h + " (draft)\nx\n":                true,
		"x\n" + h:                          false,
		h + "\n<!-- a -->\n":               false,
		h + "\n<!-- a\nb\n-->\n":           false,
		h + "\n<!-- a\nb --> c\n":          true,
		h + "\n<!-- a --> b <!-- c -->\n":  false, // awk\'s gsub takes the longest match
		h + "\nx <!-- open\n":              true,
		h + "\n<!-- open\n## Who\nstill -->\nx\n": false,
		h + "\r\n\r\n":           false,
		h + "\n##Who\nx\n":       true, // not a heading without the space
		"## Why this exist\nx\n": false,
		h + "\n" + h + "\nx\n":   false, // a second heading ends the first section
	} {
		if got := sectionPresent([]byte(body), h); got != want {
			t.Errorf("sectionPresent(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestCompareNumeric(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"1", "1", 0}, {"01", "1", 0}, {"10", "9", 1}, {"9", "10", -1}, {"0", "000", 0}, {"123456789012345678901", "2", 1}} {
		got := compareNumeric(tc.a, tc.b)
		if (got > 0) != (tc.want > 0) || (got < 0) != (tc.want < 0) {
			t.Errorf("compareNumeric(%s, %s) = %d, want sign of %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestHeaderHelpers(t *testing.T) {
	for p, want := range map[string]bool{
		"src/a.go": false, "src/a_test.go": true, "src/x.spec.tsx": true, "src/x.pb.go": true,
		"src/vendor/a.go": true, "vendor/a.go": false, "src/my_generated/a.go": true, "src/a.gen.ts": true,
		"src/build/a.js": true, "src/.git/a.go": true, "src/generated/a.go": false,
	} {
		if got := headerExempt(p); got != want {
			t.Errorf("headerExempt(%s) = %v", p, got)
		}
	}
	dir := t.TempDir()
	at := func(line int) string {
		return testutil.WriteFile(t, dir, "f.go", strings.Repeat("x\n", line-1)+"// Architecture: here\n")
	}
	if !hasContractHeader(at(15)) || hasContractHeader(at(16)) {
		t.Error("the header must be within the first 15 lines")
	}
	if hasContractHeader(dir + "/missing.go") {
		t.Error("an unreadable file has no header")
	}
}

func TestGreps(t *testing.T) {
	reg := []byte("- CONTRACT-AxB.1y0.md\n- CONTRACT-S1-FOO.1.0.md\n")
	if !greps(reg, "CONTRACT-A.B.1.0.md") {
		t.Error("dots in the name match any character, as in grep")
	}
	if !greps(reg, "CONTRACT-S1-FOO.1.0.md") || greps(reg, "CONTRACT-S2.1.0.md") {
		t.Error("greps")
	}
}
