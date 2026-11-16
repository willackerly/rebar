package checks

import (
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestEchoEscapes(t *testing.T) {
	cases := map[string]string{
		`plain`:            "plain\n",
		`\033[0;31mred`:    "\x1b[0;31mred\n",
		`a\tb\\c`:          "a\tb\\c\n",
		`\x41\x4a2`:        "AJ2\n",
		`\x`:               "\\x\n",
		`stop\chere`:       "stop",
		`\q unknown`:       "\\q unknown\n",
		`\0101 octal`:      "A octal\n",
		`trailing\`:        "trailing\\\n",
		`\u00e9 bash 4.2+`: "\\u00e9 bash 4.2+\n",
	}
	for in, want := range cases {
		if got := echoEscapes(in); got != want {
			t.Errorf("echoEscapes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRCField(t *testing.T) {
	rc := []byte("metric.a.paths = x y # comment\nmetric.a.paths = second\nmetricXb.names=*.go\nmetric.c.names =\nmetric.c.names = later\n")
	for _, tc := range []struct{ name, field, def, want string }{
		{"a", "paths", ".", "x y"},
		{"b", "names", "", "*.go"}, // grep reads the key's dots as any character
		{"c", "names", "dflt", "dflt"},
		{"d", "paths", ".", "."},
	} {
		if got := rcField(rc, tc.name, tc.field, tc.def); got != tc.want {
			t.Errorf("rcField(%s.%s) = %q, want %q", tc.name, tc.field, got, tc.want)
		}
	}
}

func TestAtLeast(t *testing.T) {
	for _, tc := range []struct {
		v, f string
		want bool
	}{
		{"3", "2", true}, {"2", "2", true}, {"1", "2", false}, {"007", "7", true},
		{"3", "two", false}, {"x", "1", false}, {"1", "99999999999999999999", false}, {"", "0", false},
	} {
		if got := atLeast(tc.v, tc.f); got != tc.want {
			t.Errorf("atLeast(%q, %q) = %v", tc.v, tc.f, got)
		}
	}
}

// TestCountMetricSedErrors: a distinct pattern sed cannot use (no group
// for \1, or not a regular expression) makes the script's pipeline print
// nothing, so the metric is 0.
func TestCountMetricSedErrors(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "a/CONTRACT-X.1.0.md", "")
	for _, distinct := range []string{"CONTRACT", "(", `^CONTRACT-(.*)\.md$`} {
		rc := []byte("metric.m.paths = a\nmetric.m.distinct = " + distinct + "\n")
		want := 0
		if strings.Contains(distinct, "(.*)") {
			want = 1
		}
		if got := countMetric(root, rc, "m"); got != want {
			t.Errorf("distinct %q: %d, want %d", distinct, got, want)
		}
	}
}

// TestGroundTruthUnterminatedQuote: xargs fails on a METRICS value with an
// unterminated quote, and under set -e the script stops there, exit 1.
func TestGroundTruthUnterminatedQuote(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "metric.m.names = METRICS\nmetric.n.names = METRICS\n")
	testutil.WriteFile(t, root, "METRICS", "m = it's\nn = 1\n")
	for _, k := range []string{"REBAR_TIER", "REBAR_RCFILE", "REBAR_CHECKS_ENABLE", "REBAR_METRICS_STRICT"} {
		t.Setenv(k, "")
	}
	var out, errOut strings.Builder
	code := runGroundTruth(&Context{Root: root, Stdout: &out, Stderr: &errOut}, nil)
	if code != 1 || strings.Contains(out.String(), "n = 1") || strings.Contains(out.String(), "drift") || !strings.Contains(errOut.String(), "unterminated quote") {
		t.Errorf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
}
