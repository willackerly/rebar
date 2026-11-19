package checks

import "testing"

func TestContractPathVersion(t *testing.T) {
	for _, tc := range []struct{ path, id, version string }{
		{"architecture/CONTRACT-S1-FOO.1.0.md", "S1-FOO", "1.0"},
		{"architecture/CONTRACT-C1-AGENTS.2.10.3.md", "C1-AGENTS", "2.10.3"},
		{"a/CONTRACT-S1.FOO.1.0.md", "S1.FOO", "1.0"},
		{"a/CONTRACT-A..1.0.md", "A.", "1.0"},
		{"a/CONTRACT-.x.1.0.md", "x", "1.0"}, // awk joins no dot after an empty first field
		{"a/CONTRACT-1.0.md", "", "1.0"},
		{"a/CONTRACT-1.2.3.md", "", "1.2.3"},
		{"a/CONTRACT-S1-FOO.md", "", ""},
		{"a/CONTRACT-S1-FOO.v2.md", "", ""},
		{"a/CONTRACT-.md", "", ""},
	} {
		id, v := contractPathVersion(tc.path)
		if id != tc.id || v != tc.version {
			t.Errorf("contractPathVersion(%q) = %q, %q; want %q, %q", tc.path, id, v, tc.id, tc.version)
		}
	}
}

func TestEchoLinesAndGrepQ(t *testing.T) {
	if !grepQ(fixSatisfied, "fix: x\n\nReproduced on: y") || grepQ(fixSatisfied, "fix: x\n\nReproduced on: \v") {
		t.Error("Reproduced on: needs a character that is not [[:space:]] (\\v included)")
	}
	if !grepQ(bypassMention, "a\nran --force") || grepQ(bypassMention, "--force-with-lease") {
		t.Error("--force must end the line or precede a space")
	}
}
